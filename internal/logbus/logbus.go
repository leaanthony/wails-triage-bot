// Package logbus fans out log lines to every subscriber. It implements
// io.Writer so `log.SetOutput` can push lines through it. Writes are
// non-blocking: slow subscribers drop messages.
package logbus

import (
	"bytes"
	"io"
	"sync"
)

type Bus struct {
	mu   sync.RWMutex
	subs map[chan string]struct{}
}

func New() *Bus { return &Bus{subs: map[chan string]struct{}{}} }

// Subscribe returns a receive-only channel of log lines plus an unsubscribe fn.
func (b *Bus) Subscribe(buffer int) (<-chan string, func()) {
	ch := make(chan string, buffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
		close(ch)
	}
}

// Write implements io.Writer. Splits input on newlines so subscribers get one
// logical line per send.
func (b *Bus) Write(p []byte) (int, error) {
	n := len(p)
	for {
		idx := bytes.IndexByte(p, '\n')
		var line string
		if idx < 0 {
			line = string(p)
			p = nil
		} else {
			line = string(p[:idx])
			p = p[idx+1:]
		}
		if line != "" {
			b.broadcast(line)
		}
		if p == nil || len(p) == 0 {
			break
		}
	}
	return n, nil
}

func (b *Bus) broadcast(line string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- line:
		default:
			// Subscriber slow; drop.
		}
	}
}

// Tee returns a writer that sends to both dst and the bus.
func (b *Bus) Tee(dst io.Writer) io.Writer {
	return io.MultiWriter(dst, b)
}
