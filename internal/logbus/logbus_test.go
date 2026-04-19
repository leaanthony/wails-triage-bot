package logbus

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func recv(t *testing.T, ch <-chan string, timeout time.Duration) (string, bool) {
	t.Helper()
	select {
	case s, ok := <-ch:
		return s, ok
	case <-time.After(timeout):
		return "", false
	}
}

func TestSubscribeAndBroadcast(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe(8)
	defer cancel()

	n, err := b.Write([]byte("hello\nworld\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n != len("hello\nworld\n") {
		t.Errorf("n=%d want %d", n, len("hello\nworld\n"))
	}

	if got, ok := recv(t, ch, time.Second); !ok || got != "hello" {
		t.Errorf("got %q ok=%v", got, ok)
	}
	if got, ok := recv(t, ch, time.Second); !ok || got != "world" {
		t.Errorf("got %q ok=%v", got, ok)
	}
}

func TestWriteNoTrailingNewline(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe(4)
	defer cancel()

	_, _ = b.Write([]byte("partial"))
	if got, ok := recv(t, ch, time.Second); !ok || got != "partial" {
		t.Errorf("got %q ok=%v", got, ok)
	}
}

func TestWriteEmptyLinesIgnored(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe(4)
	defer cancel()

	_, _ = b.Write([]byte("\n\nkeep\n\n"))
	got, ok := recv(t, ch, time.Second)
	if !ok || got != "keep" {
		t.Errorf("got %q ok=%v", got, ok)
	}
	if g, ok := recv(t, ch, 50*time.Millisecond); ok {
		t.Errorf("expected no more, got %q", g)
	}
}

func TestSlowSubscriberDropsNoBlock(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe(1) // tiny buffer
	defer cancel()

	// Fill buffer then keep writing; must not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = b.Write([]byte("x\n"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Write blocked on slow subscriber")
	}
	// Drain whatever landed; must be at least one.
	if _, ok := recv(t, ch, time.Second); !ok {
		t.Error("expected at least one delivered line")
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe(2)
	cancel()
	if _, ok := <-ch; ok {
		t.Error("channel should be closed after cancel")
	}
	// Subsequent writes must not panic on the closed sub.
	_, _ = b.Write([]byte("after\n"))
}

func TestMultipleSubscribers(t *testing.T) {
	b := New()
	c1, cancel1 := b.Subscribe(4)
	c2, cancel2 := b.Subscribe(4)
	defer cancel1()
	defer cancel2()

	var wg sync.WaitGroup
	wg.Add(2)
	var got1, got2 string
	go func() { defer wg.Done(); got1, _ = recv(t, c1, time.Second) }()
	go func() { defer wg.Done(); got2, _ = recv(t, c2, time.Second) }()

	_, _ = b.Write([]byte("ping\n"))
	wg.Wait()
	if got1 != "ping" || got2 != "ping" {
		t.Errorf("got1=%q got2=%q", got1, got2)
	}
}

func TestTee(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe(4)
	defer cancel()

	var buf bytes.Buffer
	w := b.Tee(&buf)
	if _, err := w.Write([]byte("route\n")); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "route\n" {
		t.Errorf("dst got %q", buf.String())
	}
	if got, ok := recv(t, ch, time.Second); !ok || got != "route" {
		t.Errorf("bus got %q ok=%v", got, ok)
	}
}
