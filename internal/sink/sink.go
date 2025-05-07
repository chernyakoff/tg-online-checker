package sink

import (
	"sync"
)

type ResultHandler interface {
	Handle(result any) error
	Flush() error // если нужно финализировать запись (например, закрыть файл)
}

type ResultSink struct {
	ch      chan any
	handler ResultHandler
	wg      *sync.WaitGroup
}

func NewResultSink(handler ResultHandler) *ResultSink {
	var wg sync.WaitGroup
	sink := &ResultSink{
		ch:      make(chan any),
		handler: handler,
		wg:      &wg,
	}
	sink.run()
	return sink
}

func (rs *ResultSink) run() {
	rs.wg.Add(1)
	go func() {
		defer rs.wg.Done()
		for r := range rs.ch {
			_ = rs.handler.Handle(r) // можно логировать ошибки
		}
	}()
}

func (rs *ResultSink) Submit(result any) {
	rs.ch <- result
}

func (rs *ResultSink) Close() {
	close(rs.ch)
	rs.wg.Wait()
	_ = rs.handler.Flush()
}
