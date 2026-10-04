// Package logging writes diagnostics for the current application run.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	mu   sync.Mutex
	file *os.File
	path string
}

// New removes logs from earlier runs and starts a fresh diagnostic file.
func New(dataDir string) (*Logger, error) {
	dir := filepath.Join(dataDir, "logs")
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "de_loader.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &Logger{file: f, path: path}, nil
}
func (l *Logger) Event(format string, args ...any) { l.write("INFO", format, args...) }
func (l *Logger) Error(format string, args ...any) { l.write("ERROR", format, args...) }
func (l *Logger) write(level, format string, args ...any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.file, "%s [%s] %s\n", time.Now().Format(time.RFC3339Nano), level, fmt.Sprintf(format, args...))
	_ = l.file.Sync()
}
func (l *Logger) Path() string { return l.path }
func (l *Logger) Read() ([]byte, error) {
	if l == nil {
		return nil, os.ErrNotExist
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.file.Sync()
	return os.ReadFile(l.path)
}
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}
