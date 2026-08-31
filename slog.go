package nestor

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"sync"
)

// ---

type cliHandler struct {
	mu     sync.Mutex
	writer io.Writer
	level  slog.Level
}

func (h *cliHandler) SetLevel(level string) {
	l := slog.LevelDebug
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "info":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	}
	h.level = l
}

func (h *cliHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *cliHandler) Handle(_ context.Context, r slog.Record) error {
	msg := r.Message

	attrs := ""
	r.Attrs(func(a slog.Attr) bool {
		if r.Level == slog.LevelError && a.Key == "error" {
			if err, ok := a.Value.Any().(error); ok {
				if err.Error() == cleanAnsiColor(msg) {
					return true
				}
			}
		}

		attrs += fmt.Sprintf(" %s=%v", a.Key, a.Value)
		return true
	})

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := fmt.Fprintf(h.writer, "%s%s\n", msg, attrs)
	return err
}

func (h *cliHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *cliHandler) WithGroup(name string) slog.Handler {
	return h
}

// ---

type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}

		if err := h.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: hs}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: hs}
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func cleanAnsiColor(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

type cleanHandler struct {
	inner slog.Handler
}

func (h *cleanHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.inner.Enabled(context.Background(), level)
}

func (h *cleanHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Message = cleanAnsiColor(r.Message)
	return h.inner.Handle(ctx, r)
}

func (h *cleanHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &cleanHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *cleanHandler) WithGroup(name string) slog.Handler {
	return &cleanHandler{inner: h.inner.WithGroup(name)}
}

func NewLogger(filePath string, writer io.Writer, level slog.Level) (*slog.Logger, io.Closer, error) {
	return makeLogger(
		slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level}),
		filePath, level,
		nil,
	)
}

func NewCLILogger(filePath string, writer io.Writer, level slog.Level) (*slog.Logger, io.Closer, error) {
	return makeLogger(
		&cliHandler{writer: writer, level: level},
		filePath, level,
		func(f *os.File) slog.Handler {
			return &cleanHandler{inner: slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})}
		},
	)
}

func makeLogger(base slog.Handler, filePath string, level slog.Level, fn func(*os.File) slog.Handler) (*slog.Logger, io.Closer, error) {
	handlers := []slog.Handler{base}

	var f *os.File
	var err error
	if filePath != "" {
		f, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, nil, err
		}

		if fn != nil {
			handlers = append(handlers, fn(f))
		} else {
			handlers = append(handlers, slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level}))
		}
	}

	if f == nil {
		return slog.New(NewMultiHandler(handlers...)), nil, err
	}
	return slog.New(NewMultiHandler(handlers...)), f, err
}

func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	return &multiHandler{handlers: handlers}
}
