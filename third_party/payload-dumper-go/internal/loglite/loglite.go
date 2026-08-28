// Package loglite provides the subset of log/slog used by the payload core.
// It keeps the core buildable with Go 1.20 for Windows 7 compatibility.
package loglite

import (
	"io"
	"time"
)

type Attr struct{}
type Handler struct{}
type HandlerOptions struct{ Level int }
type Logger struct{}

const LevelWarn = 4

func String(string, string) Attr          { return Attr{} }
func Int(string, int) Attr                { return Attr{} }
func Int64(string, int64) Attr            { return Attr{} }
func Uint64(string, uint64) Attr          { return Attr{} }
func Bool(string, bool) Attr              { return Attr{} }
func Duration(string, time.Duration) Attr { return Attr{} }

func NewTextHandler(io.Writer, *HandlerOptions) *Handler { return &Handler{} }
func New(*Handler) *Logger                               { return &Logger{} }
func NewDiscard() *Logger                                { return &Logger{} }

func (*Logger) Info(string, ...Attr)  {}
func (*Logger) Debug(string, ...Attr) {}
func (*Logger) Warn(string, ...Attr)  {}
