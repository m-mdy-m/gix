package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Level int

const (
	LevelSilent Level = iota

	LevelVerbose

	LevelDebug
)

type Logger struct {
	mu     sync.Mutex
	level  Level
	out    io.Writer
	prefix string
}

var std = New(os.Stderr)

func New(out io.Writer) *Logger {
	return &Logger{out: out, level: LevelSilent}
}

func SetLevel(l Level) { std.SetLevel(l) }

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) Level() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

func (l *Logger) WithPrefix(prefix string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	return &Logger{out: l.out, level: l.level, prefix: prefix}
}

func (l *Logger) logf(min Level, format string, a ...interface{}) {
	l.mu.Lock()
	level, out, prefix := l.level, l.out, l.prefix
	l.mu.Unlock()

	if level < min {
		return
	}
	msg := fmt.Sprintf(format, a...)
	if prefix != "" {
		fmt.Fprintf(out, "[%s] %s\n", prefix, msg)
		return
	}
	fmt.Fprintln(out, msg)
}

func (l *Logger) Verbosef(format string, a ...interface{}) { l.logf(LevelVerbose, format, a...) }

func (l *Logger) Debugf(format string, a ...interface{}) { l.logf(LevelDebug, format, a...) }

func (l *Logger) Timed(label string, fn func() error) error {
	start := time.Now()
	err := fn()
	l.Debugf("%s took %s", label, time.Since(start).Round(time.Millisecond))
	return err
}

func Verbosef(format string, a ...interface{}) { std.Verbosef(format, a...) }
func Debugf(format string, a ...interface{})   { std.Debugf(format, a...) }
func WithPrefix(prefix string) *Logger         { return std.WithPrefix(prefix) }
func CurrentLevel() Level                      { return std.Level() }
