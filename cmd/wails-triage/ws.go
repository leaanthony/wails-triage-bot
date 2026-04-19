package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"nhooyr.io/websocket"

	"github.com/leaanthony/wails-triage-bot/internal/agent"
	"github.com/leaanthony/wails-triage-bot/internal/logbus"
	"github.com/leaanthony/wails-triage-bot/internal/wsproto"
)

func wsHandler(ag *agent.Agent, logs *logbus.Bus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true, // local dev; tighten for prod.
		})
		if err != nil {
			log.Printf("ws accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "bye")

		ctx := r.Context()
		history := []openai.ChatCompletionMessage{}

		// Channel-based emitter so the log pump and the agent share the writer.
		frameCh := make(chan wsproto.Frame, 64)
		emit := agent.EmitterFunc(func(f wsproto.Frame) {
			select {
			case frameCh <- f:
			case <-ctx.Done():
			}
		})

		// Writer goroutine: single owner of the WS write side.
		writerDone := make(chan struct{})
		go func() {
			defer close(writerDone)
			for f := range frameCh {
				writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				if err := writeJSON(writeCtx, conn, f); err != nil {
					cancel()
					log.Printf("ws write: %v", err)
					return
				}
				cancel()
			}
		}()

		// Log pump: subscribe to the log bus, forward every line.
		logCh, unsub := logs.Subscribe(128)
		go func() {
			for line := range logCh {
				select {
				case frameCh <- wsproto.Frame{Type: wsproto.FrameLog, Data: line}:
				case <-ctx.Done():
					return
				}
			}
		}()

		defer func() {
			unsub()
			close(frameCh)
			<-writerDone
		}()

		for {
			var inbound wsproto.Frame
			if err := readJSON(ctx, conn, &inbound); err != nil {
				if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
					websocket.CloseStatus(err) == websocket.StatusGoingAway {
					return
				}
				log.Printf("ws read: %v", err)
				return
			}
			if inbound.Type != wsproto.FrameUser {
				continue
			}
			updated, err := ag.Run(ctx, history, inbound.Data, emit)
			if err != nil {
				continue
			}
			history = updated
		}
	}
}

func writeJSON(ctx context.Context, c *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, data)
}

func readJSON(ctx context.Context, c *websocket.Conn, v any) error {
	_, data, err := c.Read(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
