package proxy

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// Proxy представляет данные прокси.
type Proxy struct {
	IP       string
	Port     string
	Login    string
	Password string
}

func checkProxy(ctx context.Context, p Proxy, testURL string) (*url.URL, error) {
	// Формируем URL прокси.
	proxyURLStr := fmt.Sprintf("socks5://%s:%s@%s:%s", p.Login, p.Password, p.IP, p.Port)
	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL format: %v", err)
	}

	// Создаем контекст с таймаутом.
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Создаем SOCKS5 диалер.
	dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create dialer: %v", err)
	}

	// Используем DialContext для учета таймаута.
	conn, err := dialer.(proxy.ContextDialer).DialContext(dialCtx, "tcp", testURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect via proxy: %v", err)
	}
	conn.Close()

	return proxyURL, nil
}

// readProxies читает прокси из файла.
func readProxies(filePath string) ([]Proxy, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var proxies []Proxy
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 4 {
			fmt.Printf("Skipping invalid line: %s\n", line)
			continue
		}

		proxies = append(proxies, Proxy{
			IP:       parts[0],
			Port:     parts[1],
			Login:    parts[2],
			Password: parts[3],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return proxies, nil
}

// checkProxies проверяет список прокси в многопоточном режиме.
func checkProxies(proxies []Proxy, testURL string, workers int) ([]*url.URL, error) {
	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		workingURLs []*url.URL
	)

	// Канал для передачи задач.
	proxyChan := make(chan Proxy, len(proxies))
	for _, p := range proxies {
		proxyChan <- p
	}
	close(proxyChan)

	// Запускаем воркеры.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range proxyChan {
				proxyURL, err := checkProxy(context.Background(), p, testURL)
				if err != nil {
					fmt.Printf("🚫 Proxy %s:%s: %v\n", p.IP, p.Port, err)
					continue
				}
				fmt.Printf("✅ Proxy %s:%s\n", p.IP, p.Port)
				mu.Lock()
				workingURLs = append(workingURLs, proxyURL)
				mu.Unlock()
			}
		}()
	}

	// Ожидаем завершения всех воркеров.
	wg.Wait()
	return workingURLs, nil
}

func Get(filePath string) ([]*url.URL, error) {

	proxies, err := readProxies(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading proxies: %v", err)
	}

	// Проверяем прокси.
	workingProxies, err := checkProxies(proxies, "api.telegram.org:443", 10)
	if err != nil {
		return nil, fmt.Errorf("error checking proxies: %v", err)
	}

	return workingProxies, nil

}
