// Copyright (c) 2023 Timo Savola. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package joinslog

import (
	"context"
	"errors"
	"log/slog"
)

func Handlers(hs ...slog.Handler) slog.Handler {
	// Flatten nested handler hierarchy.  Recursion is not necessary as all
	// internal handers have been created using this function.
	var flat []slog.Handler
	for _, h := range hs {
		if h == nil {
			continue
		}
		if in, ok := h.(internal); ok {
			flat = append(flat, in.handlers()...)
		} else {
			flat = append(flat, h)
		}
	}
	switch len(flat) {
	case 0:
		return handler0{}
	case 1:
		return flat[0]
	case 2:
		return handler2{flat[0], flat[1]}
	default:
		if cap(flat) == len(flat) {
			return handlers(flat)
		}
		trim := make(handlers, len(flat))
		copy(trim, flat)
		return trim
	}
}

type internal interface {
	handlers() []slog.Handler
}

type handler0 struct{}

func (h handler0) Enabled(context.Context, slog.Level) bool  { return false }
func (h handler0) Handle(context.Context, slog.Record) error { return nil }
func (h handler0) WithAttrs([]slog.Attr) slog.Handler        { return h }
func (h handler0) WithGroup(string) slog.Handler             { return h }
func (h handler0) handlers() []slog.Handler                  { return nil }

type handler2 [2]slog.Handler

func (hs handler2) Enabled(ctx context.Context, l slog.Level) bool {
	return hs[0].Enabled(ctx, l) || hs[1].Enabled(ctx, l)
}

func (hs handler2) Handle(ctx context.Context, r slog.Record) error {
	var errs [2]error
	if hs[0].Enabled(ctx, r.Level) {
		errs[0] = hs[0].Handle(ctx, r.Clone())
	}
	if hs[1].Enabled(ctx, r.Level) {
		errs[1] = hs[1].Handle(ctx, r)
	}
	if errs[0] == nil {
		return errs[1]
	}
	if errs[1] == nil {
		return errs[0]
	}
	return errors.Join(errs[:]...)
}

func (hs handler2) WithAttrs(attrs []slog.Attr) slog.Handler {
	var a []slog.Attr
	// Slice off excess capacity at the back, if there's enough.
	if tail := cap(attrs) - len(attrs); tail >= len(attrs) {
		a = attrs[tail:cap(attrs)]
		attrs = attrs[:len(attrs):tail]
	} else {
		a = make([]slog.Attr, len(attrs))
	}
	copy(a, attrs)
	return handler2{
		hs[0].WithAttrs(a),
		hs[1].WithAttrs(attrs),
	}
}

func (hs handler2) WithGroup(name string) slog.Handler {
	return handler2{
		hs[0].WithGroup(name),
		hs[1].WithGroup(name),
	}
}

func (hs handler2) handlers() []slog.Handler {
	return hs[:]
}

type handlers []slog.Handler

func (hs handlers) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range hs {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (hs handlers) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range hs[:len(hs)-1] {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if h := hs[len(hs)-1]; h.Enabled(ctx, r.Level) {
		if err := h.Handle(ctx, r); err != nil {
			errs = append(errs, err)
		}
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return errors.Join(errs...)
	}
}

func (hs handlers) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs2 := make(handlers, len(hs))
	for i, h := range hs[:len(hs)-1] {
		var a []slog.Attr
		// Slice off excess capacity at the back, if there's enough.
		if tail := cap(attrs) - len(attrs); tail >= len(attrs) {
			a = attrs[tail:cap(attrs)]
			attrs = attrs[:len(attrs):tail]
		} else {
			a = make([]slog.Attr, len(attrs))
		}
		copy(a, attrs)
		hs2[i] = h.WithAttrs(a)
	}
	hs2[len(hs)-1] = hs[len(hs)-1].WithAttrs(attrs)
	return hs2
}

func (hs handlers) WithGroup(name string) slog.Handler {
	hs2 := make(handlers, len(hs))
	for i, h := range hs {
		hs2[i] = h.WithGroup(name)
	}
	return hs2
}

func (hs handlers) handlers() []slog.Handler {
	return hs
}
