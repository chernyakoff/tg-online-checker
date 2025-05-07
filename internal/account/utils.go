package account

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram/dcs"
	"golang.org/x/net/proxy"
)

const (
	defaultAppID   = 2040
	defaultAppHash = "b18441a1ff607e10a989891a5462e627"
)

func normalizeKey(key string) string {
	key = strings.ToLower(key)
	key = strings.ReplaceAll(key, "_", "")
	return key
}

type jsonData struct {
	Version int
	Data    session.Data
}

func getSessionFiles(sessionDir string) ([]string, error) {
	sessionFiles := []string{}
	err := filepath.Walk(sessionDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(path) == ".session" {
			sessionFiles = append(sessionFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	//fmt.Printf("Found %d session files\n", len(sessionFiles))
	return sessionFiles, nil
}

func getStorage(a *Account) (*session.StorageMemory, error) {
	data, err := SqiteSession(a.SessionPath)
	if err != nil {
		return nil, err
	}
	v := jsonData{
		Version: 1,
		Data:    *data,
	}

	buf, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	storage := &session.StorageMemory{}
	storage.StoreSession(context.Background(), buf)
	return storage, nil
}

func getResolver(a *Account) (dcs.Resolver, error) {

	auth := &proxy.Auth{
		User:     a.Proxy.User.Username(),
		Password: "",
	}
	if pwd, ok := a.Proxy.User.Password(); ok {
		auth.Password = pwd
	}

	dialer, err := proxy.SOCKS5("tcp", a.Proxy.Host, auth, proxy.Direct)
	if err != nil {
		return nil, err
	}

	dialFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.Dial(network, addr)
	}

	return dcs.Plain(dcs.PlainOptions{Dial: dialFunc}), nil

}

// loadAppCredentials загружает app_id и app_hash из JSON файла
func getAppCredentials(sessionPath string) (int, string, error) {

	jsonPath := strings.TrimSuffix(sessionPath, filepath.Ext(sessionPath)) + ".json"
	// Открываем файл
	file, err := os.Open(jsonPath)
	if err != nil {
		// Возвращаем значения по умолчанию, если файл не найден
		return defaultAppID, defaultAppHash, nil
	}
	defer file.Close()

	// Декодируем JSON в map для обработки произвольных ключей
	var raw map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&raw); err != nil {
		return 0, "", fmt.Errorf("failed to decode JSON: %w", err)
	}

	// Ищем app_id и app_hash, игнорируя регистр и подчеркивания
	var appID int
	var appHash string
	foundAppID, foundAppHash := false, false

	for key, value := range raw {
		normalizedKey := normalizeKey(key)
		switch normalizedKey {
		case "appid", "apiid":
			switch v := value.(type) {
			case float64:
				appID = int(v)
				foundAppID = true
			case string:
				var err error
				if appID, err = strconv.Atoi(v); err != nil {
					return 0, "", fmt.Errorf("invalid app_id format for key %s: %w", key, err)
				}
				foundAppID = true
			default:
				return 0, "", fmt.Errorf("invalid app_id type for key %s: %T", key, v)
			}
		case "apphash", "apihash":
			if hash, ok := value.(string); ok {
				appHash = hash
				foundAppHash = true
			} else {
				return 0, "", fmt.Errorf("invalid app_hash type for key %s: %T", key, value)
			}
		}
	}

	// Если оба поля найдены, возвращаем их
	if foundAppID && foundAppHash {
		return appID, appHash, nil
	}

	// Если хотя бы одно поле не найдено, возвращаем значения по умолчанию
	return defaultAppID, defaultAppHash, nil
}
