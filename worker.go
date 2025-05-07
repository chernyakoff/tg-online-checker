package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"tg-online-checker/internal/account"
	"tg-online-checker/internal/model"
	"tg-online-checker/internal/sink"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

type Worker struct {
	taskChan      <-chan model.Command
	ctx           context.Context
	manager       *account.AccountManager
	wg            *sync.WaitGroup
	sink          *sink.ResultSink
	doneProducing <-chan struct{}
}

type Result struct {
	Username string
}

func (w *Worker) handleTask(api *tg.Client, task model.Command) error {

	peer, err := api.ContactsResolveUsername(w.ctx, &tg.ContactsResolveUsernameRequest{Username: task.Username})
	if err != nil {
		return err
	}
	if len(peer.Users) > 0 {
		tgUser, ok := peer.Users[0].(*tg.User)
		if !ok {
			return fmt.Errorf("unexpected type in peer.Users[0]")
		}

		user := model.NewUser(tgUser)
		w.sink.Submit(user)
	}

	log.Printf("Successfully resolved @%s", task.Username)
	return nil
}

func (w *Worker) start(acc *account.Account) {

	defer acc.Release()

	client := telegram.NewClient(acc.AppID, acc.AppHash, telegram.Options{
		SessionStorage: acc.Storage,
		Resolver:       acc.Resolver,
	})

	err := client.Run(w.ctx, func(ctx context.Context) error {

		log.Printf("[%s] Starting worker", acc.ID)

		api := client.API()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case task, ok := <-w.taskChan:

				if !ok {
					log.Printf("[%s] task channel closed, worker exiting", acc.ID)
					return nil // Завершаем работу воркера
				}
				if task.Username == "" {
					continue // защита от мусора, если вдруг попадет
				}

				if !acc.IsValid() {

					return nil
				}

				if err := w.handleTask(api, task); err != nil {

					floodWait := isFloodWait(err)
					fmt.Println("FW ", floodWait)
					if floodWait != 0 {
						acc.SetFloodWait(floodWait)
					}
					if isBanned(err) {
						acc.MarkBanned()
					}
					log.Printf("[%s] error handling task: %v", acc.ID, err)
				}
			}
		}
	})

	if err != nil {
		log.Printf("[%s] client exited: %v", acc.ID, err)
	}
}

// Запускает воркеров и следит за их статусом
func (w *Worker) Monitor() {
	defer w.wg.Done()
	for {
		select {
		case <-w.ctx.Done():
			log.Println("[monitor] context canceled, shutting down")
			return
		default:
			acc := w.manager.GetAvailable()
			if acc == nil {
				log.Println("[monitor] no available accounts, retrying in 5s...")
				select {
				case <-w.ctx.Done():
					log.Println("[monitor] context canceled while waiting for accounts")
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}
			workerExited := make(chan struct{})
			go func() {
				w.start(acc)
				workerExited <- struct{}{}
			}()

			select {
			case <-w.ctx.Done():
				log.Println("[monitor] context canceled, stopping worker")
				return
			case <-workerExited:
				log.Printf("[monitor] worker %s exited, trying next account", acc.ID)

				// Проверяем, завершился ли producer и задачи тоже закончились
				select {
				case <-w.doneProducing:
					if isChannelClosedAndEmpty(w.taskChan) {
						log.Println("[monitor] all tasks processed, exiting monitor")
						return
					}
				default:
					// producer еще не завершил работу — продолжаем
				}
			}
		}
	}
}

func isChannelClosedAndEmpty(ch <-chan model.Command) bool {
	select {
	case _, ok := <-ch:
		return !ok
	default:
		return false
	}
}

func isBanned(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "PHONE_NUMBER_BANNED") ||
		strings.Contains(msg, "USER_DEACTIVATED") ||
		strings.Contains(msg, "AUTH_KEY_UNREGISTERED")
}

var floodWaitRegex = regexp.MustCompile(`FLOOD_WAIT \((\d+)\)`)

func parseFloodWaitSeconds(err error) int {
	matches := floodWaitRegex.FindStringSubmatch(err.Error())
	if len(matches) == 2 {
		seconds, _ := strconv.Atoi(matches[1])
		return seconds
	}
	return 0
}

func isFloodWait(err error) int {
	if err == nil {
		return 0
	}
	// Telegram ошибки часто приходят как строки, содержащие FLOOD_WAIT_X
	if strings.Contains(err.Error(), "FLOOD_WAIT") {
		return parseFloodWaitSeconds(err)
	}
	return 0
}
