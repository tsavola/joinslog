// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package joinslog

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

// mockFailingHandler is a handler that always returns an error
// from its Handle method.
type mockFailingHandler struct {
	slog.Handler
	err error
}

func (h *mockFailingHandler) Handle(ctx context.Context, r slog.Record) error {
	_ = h.Handler.Handle(ctx, r)
	return h.err
}

func TestHandlers(t *testing.T) {
	t.Run("Handle sends log to all handlers", func(t *testing.T) {
		var buf1, buf2 bytes.Buffer
		h1 := slog.NewTextHandler(&buf1, nil)
		h2 := slog.NewJSONHandler(&buf2, nil)

		multi := Handlers(h1, h2)
		logger := slog.New(multi)

		logger.Info("hello world", "user", "test")

		checkLogOutput(t, buf1.String(), "time="+textTimeRE+` level=INFO msg="hello world" user=test`)
		checkLogOutput(t, buf2.String(), `{"time":"`+jsonTimeRE+`","level":"INFO","msg":"hello world","user":"test"}`)
	})

	t.Run("Enabled returns true if any handler is enabled", func(t *testing.T) {
		h1 := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError})
		h2 := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelInfo})

		multi := Handlers(h1, h2)

		if !multi.Enabled(context.Background(), slog.LevelInfo) {
			t.Error("Enabled should be true for INFO level, but got false")
		}
		if !multi.Enabled(context.Background(), slog.LevelError) {
			t.Error("Enabled should be true for ERROR level, but got false")
		}
	})

	t.Run("Enabled returns false if no handlers are enabled", func(t *testing.T) {
		h1 := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError})
		h2 := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelInfo})

		multi := Handlers(h1, h2)

		if multi.Enabled(context.Background(), slog.LevelDebug) {
			t.Error("Enabled should be false for DEBUG level, but got true")
		}
	})

	t.Run("WithAttrs propagates attributes to all handlers", func(t *testing.T) {
		var buf1, buf2 bytes.Buffer
		h1 := slog.NewTextHandler(&buf1, nil)
		h2 := slog.NewJSONHandler(&buf2, nil)

		multi := Handlers(h1, h2).WithAttrs([]slog.Attr{slog.String("request_id", "123")})
		logger := slog.New(multi)

		logger.Info("request processed")

		checkLogOutput(t, buf1.String(), "time="+textTimeRE+` level=INFO msg="request processed" request_id=123`)
		checkLogOutput(t, buf2.String(), `{"time":"`+jsonTimeRE+`","level":"INFO","msg":"request processed","request_id":"123"}`)
	})

	t.Run("WithGroup propagates group to all handlers", func(t *testing.T) {
		var buf1, buf2 bytes.Buffer
		h1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{AddSource: false})
		h2 := slog.NewJSONHandler(&buf2, &slog.HandlerOptions{AddSource: false})

		multi := Handlers(h1, h2).WithGroup("req")
		logger := slog.New(multi)

		logger.Info("user login", "user_id", 42)

		checkLogOutput(t, buf1.String(), "time="+textTimeRE+` level=INFO msg="user login" req.user_id=42`)
		checkLogOutput(t, buf2.String(), `{"time":"`+jsonTimeRE+`","level":"INFO","msg":"user login","req":{"user_id":42}}`)
	})

	t.Run("Handle propagates errors from handlers", func(t *testing.T) {
		errFail := errors.New("mock failing")

		var buf1, buf2 bytes.Buffer
		h1 := slog.NewTextHandler(&buf1, nil)
		h2 := &mockFailingHandler{Handler: slog.NewJSONHandler(&buf2, nil), err: errFail}

		multi := Handlers(h2, h1)

		err := multi.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0))
		if !errors.Is(err, errFail) {
			t.Errorf("Expected error: %v, but got: %v", errFail, err)
		}

		checkLogOutput(t, buf1.String(), "time="+textTimeRE+` level=INFO msg="test message"`)
		checkLogOutput(t, buf2.String(), `{"time":"`+jsonTimeRE+`","level":"INFO","msg":"test message"}`)
	})

	t.Run("Handle with no handlers", func(t *testing.T) {
		multi := Handlers()
		logger := slog.New(multi)

		logger.Info("nothing")

		err := multi.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0))
		if err != nil {
			t.Errorf("Handle with no sub-handlers should return nil, but got: %v", err)
		}
	})
}

// Test that Handlers() copies the input slice and is insulated from future modification.
func TestHandlersCopy(t *testing.T) {
	var buf1 bytes.Buffer
	h1 := slog.NewTextHandler(&buf1, nil)
	slice := []slog.Handler{h1}
	multi := Handlers(slice...)
	slice[0] = nil

	err := multi.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0))
	if err != nil {
		t.Errorf("Expected nil error, but got: %v", err)
	}
	checkLogOutput(t, buf1.String(), "time="+textTimeRE+` level=INFO msg="test message"`)
}
