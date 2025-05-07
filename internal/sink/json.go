package sink

import (
	"encoding/json"
	"os"
	"sync"
)

type JSONHandler struct {
	mu      sync.Mutex
	file    *os.File
	results []any
}

func NewJSONHandler(filename string) (*JSONHandler, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return &JSONHandler{file: f}, nil
}

func (h *JSONHandler) Handle(result any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.results = append(h.results, result)
	return nil
}

func (h *JSONHandler) Flush() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	enc := json.NewEncoder(h.file)
	enc.SetIndent("", "  ") // красиво форматируем
	if err := enc.Encode(h.results); err != nil {
		return err
	}
	return h.file.Close()
}
