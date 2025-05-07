package main

import (
	"context"
	"log"
	"sync"
	"tg-online-checker/internal/account"
	"tg-online-checker/internal/model"
	"tg-online-checker/internal/proxy"
	"tg-online-checker/internal/sink"
)

func main() {
	cfg := MustLoadConfig()

	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxies, err := proxy.Get(cfg.File.Proxy)
	if err != nil {
		log.Fatalf("cant get proxies: %v", err)
	}

	// WaitGroup для ожидания завершения всех MonitorWorker
	var wg sync.WaitGroup

	handler, err := sink.NewCSVHandler(cfg.File.Result) // или CSV/Console
	if err != nil {
		log.Fatalf("Ошибка создания csv обработчика: %v", err)
	}
	resultSink := sink.NewResultSink(handler)
	defer resultSink.Close()

	// Читаем пользователей
	users, err := GetUsers(cfg.File.Users)
	if err != nil {
		log.Fatalf("cant get users: %v", err)
	}

	taskChan := make(chan model.Command, len(users))
	manager, err := account.NewManager(cfg.Dir.Sessions, proxies)
	if err != nil {
		log.Fatalf("cant create account manager: %v", err)
	}
	doneProducing := make(chan struct{})

	worker := Worker{
		ctx:           ctx,
		wg:            &wg,
		manager:       manager,
		taskChan:      taskChan,
		sink:          resultSink,
		doneProducing: doneProducing,
	}

	// Запускаем 5 параллельных MonitorWorker
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go worker.Monitor()
	}

	// Запускаем генератор задач
	go func() {

		TaskProducer(ctx, users, taskChan, doneProducing)
		// После отправки всех задач отменяем контекст
		log.Println("[main] all tasks produced, canceling context")

	}()

	// Ожидаем завершения всех воркеров
	wg.Wait()
	log.Println("[main] all workers completed, shutting down")
}
