package account

import (
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type managerTotals struct {
	mu          sync.Mutex
	sessionDir  string
	total       int
	validCount  int
	bannedCount int
	floodCount  int
}

func (s *managerTotals) update(acc *Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if acc.IsBanned {
		s.bannedCount++
	}
	if acc.FloodWait > 0 {
		s.floodCount++
	}
}

func (s *managerTotals) print() {
	fmt.Printf("📊 Аккаунтов: %d\n", s.total)
	fmt.Printf("✅ Рабочих: %d\n", s.validCount)
	fmt.Printf("🚫 В бане: %d\n", s.bannedCount)
	fmt.Printf("🌊 С таймаутом: %d\n", s.floodCount)

}

// AccountManager управляет пулом аккаунтов.
type AccountManager struct {
	accounts []*Account
	mu       sync.Mutex
	totals   *managerTotals
}

// NewManager создаёт менеджер аккаунтов на основе сессий и списка прокси.
func NewManager(sessionDir string, proxies []*url.URL) (*AccountManager, error) {
	sessionPaths, err := getSessionFiles(sessionDir)
	if err != nil {
		return nil, err
	}
	if len(sessionPaths) == 0 || len(proxies) == 0 {
		return nil, errors.New("empty session list or proxy list")
	}

	totals := &managerTotals{sessionDir: sessionDir, total: len(sessionPaths)}

	accs := make([]*Account, 0, len(sessionPaths))
	for i, path := range sessionPaths {

		acc := NewAccount(path, proxies[i%len(proxies)])
		totals.update(acc)

		if !acc.IsValid() {
			continue
		}

		storage, err := getStorage(acc)
		if err != nil {
			log.Printf("⚠️ Cant create storage for [%s]: %v", acc.ID, err)
			continue
		}
		acc.Storage = storage

		resolver, err := getResolver(acc)
		if err != nil {
			log.Printf("⚠️ Cant create resolver for [%s]: %v", acc.ID, err)
			continue
		}
		acc.Resolver = resolver

		totals.validCount++

		accs = append(accs, acc)
	}

	return &AccountManager{accounts: accs, totals: totals}, nil
}

func (am *AccountManager) PrintTotals() {
	am.totals.print()
}

func (am *AccountManager) RefreshTotals() {
	am.mu.Lock()
	defer am.mu.Unlock()
	for _, acc := range am.accounts {
		acc.LoadState()
		am.totals.update(acc)
	}
}

func (am *AccountManager) Shutdown() error {
	am.mu.Lock()
	defer am.mu.Unlock()
	var errs []error
	for _, acc := range am.accounts {
		if err := acc.SaveState(); err != nil {
			errs = append(errs, fmt.Errorf("account %s: %w", acc.ID, err))
		}

	}
	if len(errs) > 0 {
		return fmt.Errorf("ошибки при сохранении states: %v", errs)
	}
	return nil
}

func (am *AccountManager) GetAccounts() []*Account {
	return am.accounts
}

func (am *AccountManager) GetAvailable() *Account {
	am.mu.Lock()
	defer am.mu.Unlock()
	for _, acc := range am.accounts {
		acc.lock.Lock()
		if acc.IsValid() && !acc.InUse {
			acc.UsedToday++
			acc.LastUsed = time.Now().Unix()
			acc.InUse = true // 👈 помечаем как занятый
			acc.lock.Unlock()
			return acc
		}
		acc.lock.Unlock()
	}
	return nil
}
