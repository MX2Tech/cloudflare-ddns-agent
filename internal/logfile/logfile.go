// Package logfile is a size-capped append-only log file, for platforms
// where the agent has no journald to write to.
package logfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// File appends to path and, once a write would push it past maxBytes,
// moves it to path+".1" (replacing the previous one) and starts over.
// Disk use is therefore bounded by about 2*maxBytes.
type File struct {
	mu   sync.Mutex
	path string
	max  int64
	f    *os.File
	size int64
}

func Open(path string, maxBytes int64) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}
	l := &File{path: path, max: maxBytes}
	if err := l.open(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *File) open() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("opening log file %q: %w", l.path, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("stat log file %q: %w", l.path, err)
	}
	l.f, l.size = f, info.Size()
	return nil
}

func (l *File) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.size > 0 && l.size+int64(len(p)) > l.max {
		if err := l.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := l.f.Write(p)
	l.size += int64(n)
	return n, err
}

// rotate closes the file before renaming it: Windows cannot rename a file
// that is still open.
func (l *File) rotate() error {
	if err := l.f.Close(); err != nil {
		return fmt.Errorf("closing log file: %w", err)
	}
	old := l.path + ".1"
	if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %q: %w", old, err)
	}
	if err := os.Rename(l.path, old); err != nil {
		return fmt.Errorf("rotating log file: %w", err)
	}
	return l.open()
}

func (l *File) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}
