package sink

import (
	"encoding/json"
	"fmt"
	"sync"
)

type MemoryHandler struct {
	mu      sync.Mutex
	results []any
}

func NewMemoryHandler() *MemoryHandler {
	return &MemoryHandler{}
}

func (h *MemoryHandler) Handle(result any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.results = append(h.results, result)
	return nil
}

func (h *MemoryHandler) Flush() error {
	return nil
}

// Results returns a copy of the stored results
func (h *MemoryHandler) Results() []any {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]any{}, h.results...)
}

// PrettyPrint prints all results in a human-readable JSON format
func (h *MemoryHandler) PrettyPrint() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, r := range h.results {
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			fmt.Printf("[%d] Error: %v\n", i, err)
			continue
		}
		fmt.Printf("[%d]\n%s\n", i, data)
	}
}
