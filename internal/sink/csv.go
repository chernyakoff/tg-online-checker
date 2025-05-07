package sink

import (
	"encoding/csv"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
)

type CSVHandler struct {
	mu       sync.Mutex
	writer   *csv.Writer
	file     *os.File
	headers  []string
	initOnce sync.Once
}

func NewCSVHandler(filename string) (*CSVHandler, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return &CSVHandler{
		writer: csv.NewWriter(f),
		file:   f,
	}, nil
}

func (h *CSVHandler) Handle(result any) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	val := reflect.ValueOf(result)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	h.initOnce.Do(func() {
		var headers []string
		for i := 0; i < val.NumField(); i++ {
			field := typ.Field(i)
			jsonTag := field.Tag.Get("json")
			if jsonTag != "" {
				headers = append(headers, strings.Split(jsonTag, ",")[0])
			} else {
				headers = append(headers, field.Name)
			}
		}
		h.headers = headers
		_ = h.writer.Write(headers)
	})

	var row []string
	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		row = append(row, fmt.Sprintf("%v", f.Interface()))
	}
	return h.writer.Write(row)
}

func (h *CSVHandler) Flush() error {
	h.writer.Flush()
	return h.file.Close()
}

//--------------------------------
