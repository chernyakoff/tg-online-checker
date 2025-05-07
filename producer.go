package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"tg-online-checker/internal/model"
	"time"
)

func GetUsers(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл: %w", err)
	}
	defer file.Close()

	var items []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			items = append(items, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	return items, nil
}

// Мок-генератор задач (вместо RabbitMQ)
func TaskProducer(ctx context.Context, usernames []string, taskChan chan<- model.Command) {
	defer close(taskChan)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for _, username := range usernames {
		select {
		case <-ctx.Done():
			log.Println("[producer] context canceled, stopping task production")
			return
		case taskChan <- model.Command{Username: username}:
			<-ticker.C
		}
	}
	log.Println("[producer] all tasks sent, closing task channel")
}
