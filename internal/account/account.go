package account

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"net/url"
	"path/filepath"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram/dcs"
	_ "modernc.org/sqlite"
)

// Account представляет Telegram аккаунт с привязанной сессией и прокси.
type Account struct {
	ID          string
	SessionPath string
	StatePath   string
	Proxy       *url.URL
	Storage     *session.StorageMemory
	Resolver    dcs.Resolver
	UsedToday   int
	IsBanned    bool
	LastUsed    int64
	lock        sync.Mutex
	AppID       int
	AppHash     string
	FloodWait   int64
	InUse       bool
}

type accountState struct {
	ID        string `json:"id"`
	AppID     int    `json:"app_id"`
	AppHash   string `json:"app_hash"`
	IsBanned  bool   `json:"is_banned"`
	UsedToday int    `json:"used_today"`
	LastUsed  int64  `json:"last_used"`
	FloodWait int64  `json:"flood_wait"`
}

func getID(path string) string {
	path = filepath.Base(path)
	return strings.TrimSuffix(path, filepath.Ext(path))
}

func NewAccount(path string, proxy *url.URL) *Account {
	a := &Account{
		SessionPath: path,
		Proxy:       proxy,
		UsedToday:   0,
		LastUsed:    time.Now().Unix(),
	}

	a.ID = getID(a.SessionPath)
	a.StatePath = strings.TrimSuffix(a.SessionPath, filepath.Ext(a.SessionPath)) + ".state"
	a.LoadState()
	if a.AppHash == "" || a.AppID == 0 {
		a.TryLoadAppCredsFromJson()
	}
	return a

}

func (a *Account) TryLoadAppCredsFromJson() {
	apiID, apiHash, err := getAppCredentials(a.SessionPath)
	if err == nil {
		a.lock.Lock()
		defer a.lock.Unlock()
		a.AppHash = apiHash
		a.AppID = apiID
	}
}

func (a *Account) IsValid() bool {
	return !a.IsBanned && a.UsedToday < 200 && a.FloodWait == 0
}

func (a *Account) MarkBanned() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.IsBanned = true
}

func (a *Account) SetFloodWait(seconds int) {
	a.lock.Lock()
	defer a.lock.Unlock()
	now := time.Now().Unix()
	a.FloodWait = now + int64(seconds)
}

func (a *Account) IncrementUsage() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.UsedToday++
	a.LastUsed = time.Now().Unix()
}

func (a *Account) Release() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.InUse = false
}

func (a *Account) LoadState() error {

	data, err := os.ReadFile(a.StatePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("failed to read state file for %s: %v", a.ID, err)
		}
		return err
	}

	var state accountState
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("failed to unmarshal state for %s: %v", a.ID, err)
		return err
	}

	a.lock.Lock()
	a.AppHash = state.AppHash
	a.AppID = state.AppID
	a.IsBanned = state.IsBanned

	a.UsedToday = state.UsedToday
	a.LastUsed = state.LastUsed
	a.FloodWait = state.FloodWait
	a.lock.Unlock()
	return nil
}

func (a *Account) SaveState() error {
	a.lock.Lock()
	state := accountState{
		ID:       a.ID,
		AppID:    a.AppID,
		AppHash:  a.AppHash,
		IsBanned: a.IsBanned,

		UsedToday: a.UsedToday,
		LastUsed:  a.LastUsed,
		FloodWait: a.FloodWait,
	}
	a.lock.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("failed to marshal state for %s: %v", a.ID, err)
		return err

	}
	if err := os.WriteFile(a.StatePath, data, 0644); err != nil {
		log.Printf("failed to write state for %s: %v", a.ID, err)
		return err
	}
	return nil
}
