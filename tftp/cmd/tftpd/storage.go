package main

import "sync"

// Storage Interface for storage
type Storage interface {
	Get(filename string) ([]byte, bool)
	Store(filename string, data []byte)
}

type inMemoryStore struct {
	files map[string][]byte
	lock  sync.RWMutex
}

// NewInMemoryStore constructs an in memory file store
func NewInMemoryStore() *inMemoryStore {
	files := make(map[string][]byte)
	return &inMemoryStore{files: files}
}

func (t *inMemoryStore) Get(filename string) ([]byte, bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	f, o := t.files[filename]
	return f, o
}

func (t *inMemoryStore) Store(filename string, data []byte) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.files[filename] = data
}
