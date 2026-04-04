// Package logger provides structured logging for the Vitis server.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

var global *slog.Logger

func init() {
	global = slog.New(newConsoleHandler(os.Stdout, slog.LevelInfo))
}

// Init configures the global logger with the given level and format.
// Supported levels: "debug", "info", "warn", "error".
// Supported formats: "console" (default), "json".
func Init(level string, format string, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}

	lvl := parseLevel(level)

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})
	default:
		handler = newConsoleHandler(w, lvl)
	}

	global = slog.New(handler)
	slog.SetDefault(global)
}

// L returns the global logger instance.
func L() *slog.Logger {
	return global
}

// Info logs at info level.
func Info(msg string, args ...any) {
	global.Info(msg, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	global.Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	global.Error(msg, args...)
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	global.Debug(msg, args...)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

const (
	colorReset  = "\033[0m"
	colorGray   = "\033[90m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
)

type consoleHandler struct {
	w     io.Writer
	level slog.Level
	mu    *sync.Mutex
	attrs []slog.Attr
	group string
}

func newConsoleHandler(w io.Writer, level slog.Level) *consoleHandler {
	return &consoleHandler{w: w, level: level, mu: &sync.Mutex{}}
}

func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	var buf strings.Builder

	ts := r.Time.Format(time.TimeOnly)
	buf.WriteString(colorGray)
	buf.WriteString(ts)
	buf.WriteString(colorReset)
	buf.WriteByte(' ')

	lvl, color := formatLevel(r.Level)
	buf.WriteString(color)
	buf.WriteString(colorBold)
	buf.WriteString(lvl)
	buf.WriteString(colorReset)
	buf.WriteByte(' ')

	buf.WriteString(r.Message)

	prefix := h.group
	for _, a := range h.attrs {
		writeAttr(&buf, prefix, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&buf, prefix, a)
		return true
	})

	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, buf.String())
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &consoleHandler{
		w:     h.w,
		level: h.level,
		mu:    h.mu,
		attrs: append(append([]slog.Attr{}, h.attrs...), attrs...),
		group: h.group,
	}
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	g := name
	if h.group != "" {
		g = h.group + "." + name
	}
	return &consoleHandler{
		w:     h.w,
		level: h.level,
		mu:    h.mu,
		attrs: append([]slog.Attr{}, h.attrs...),
		group: g,
	}
}

func formatLevel(l slog.Level) (string, string) {
	switch {
	case l >= slog.LevelError:
		return "ERROR", colorRed
	case l >= slog.LevelWarn:
		return "WARN ", colorYellow
	case l >= slog.LevelInfo:
		return "INFO ", colorGreen
	default:
		return "DEBUG", colorCyan
	}
}

func writeAttr(buf *strings.Builder, prefix string, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}
	buf.WriteByte(' ')
	buf.WriteString(colorGray)
	if prefix != "" {
		buf.WriteString(prefix)
		buf.WriteByte('.')
	}
	buf.WriteString(a.Key)
	buf.WriteByte('=')
	buf.WriteString(colorReset)
	buf.WriteString(fmt.Sprintf("%v", a.Value.Any()))
}
