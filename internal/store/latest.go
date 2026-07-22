package store

import (
	"sync"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

type Latest struct {
	mu       sync.RWMutex
	snapshot models.Snapshot
}

func NewLatest() *Latest {
	return &Latest{}
}

func (l *Latest) Set(snapshot models.Snapshot) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.snapshot = snapshot
}

func (l *Latest) Get() models.Snapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.snapshot
}

func (l *Latest) Age() time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.snapshot.Timestamp.IsZero() {
		return 0
	}
	return time.Since(l.snapshot.Timestamp)
}
