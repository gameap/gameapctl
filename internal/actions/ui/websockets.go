// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ui

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	contextInternal "github.com/gameap/gameapctl/internal/context"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

type message struct {
	Topic string `json:"topic"`
	Code  string `json:"code"`
	Value string `json:"value,omitempty"`
}

const (
	messageCodePayload = "payload"
	messageCodeError   = "error"
	messageCodeEnd     = "end"
)

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)

		return
	}
	defer func() {
		err := ws.Close()
		if err != nil {
			log.Println("close err:", err)
		}
	}()

	ctx, err := contextInternal.SetOSContext(r.Context())
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to set OS context"))

		return
	}

	for {
		mt, msg, err := ws.ReadMessage()
		if err != nil {
			log.Println(errors.WithMessage(err, "failed to read message"))

			break
		}
		err = wsRequest(ctx, ws, mt, msg)
		if err != nil {
			if errors.Is(err, errClosed) {
				break
			}

			log.Println(errors.WithMessage(err, "failed to handle request"))

			break
		}
	}
}

var errClosed = errors.New("closed")

func wsRequest(ctx context.Context, ws *websocket.Conn, mt int, msg []byte) error {
	var m message

	err := json.Unmarshal(msg, &m)
	if err != nil {
		return errors.WithMessage(err, "failed to unmarshal message")
	}

	log.Printf("recv: %s", msg)

	rw := newResponseWriter(ws, m.Topic)

	if m.Topic == "exit" {
		err = ws.Close()
		if err != nil {
			return errors.WithMessage(err, "failed to close connection")
		}

		done <- struct{}{}

		return errClosed
	}

	err = cmdHandle(ctx, rw, m)
	if err != nil {
		log.Println(errors.WithMessage(err, "failed to handle command"))

		b, err := json.Marshal(message{
			Topic: m.Topic,
			Code:  messageCodeError,
			Value: err.Error(),
		})
		if err != nil {
			return errors.WithMessage(err, "failed to handle command and marshal message")
		}

		err = rw.WriteMessage(mt, b)
		if err != nil {
			return errors.WithMessage(err, "failed to handle command and write error message")
		}
	}

	b, err := json.Marshal(message{
		Topic: m.Topic,
		Code:  messageCodeEnd,
		Value: "",
	})
	if err != nil {
		return errors.WithMessage(err, "failed to marshal message")
	}
	err = rw.WriteMessage(mt, b)
	if err != nil {
		return errors.WithMessage(err, "failed to write message")
	}

	return nil
}

type responseWriter struct {
	mu    sync.Mutex
	conn  *websocket.Conn
	topic string
}

func newResponseWriter(conn *websocket.Conn, topic string) *responseWriter {
	return &responseWriter{topic: topic, conn: conn}
}

func (rw *responseWriter) Write(p []byte) (n int, err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	b, err := json.Marshal(message{
		Topic: rw.topic,
		Code:  messageCodePayload,
		Value: string(p),
	})
	if err != nil {
		return 0, errors.WithMessage(err, "failed to marshal message")
	}
	err = rw.conn.WriteMessage(websocket.TextMessage, b)
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

func (rw *responseWriter) WriteMessage(messageType int, data []byte) error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	return rw.conn.WriteMessage(messageType, data)
}
