package blob

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// Memory is an in-memory domain.BlobStore for tests. It stores raw bytes
// verbatim (no image normalization), keyed by the same relative paths as FS.
type Memory struct {
	mu   sync.Mutex
	data map[string][]byte
}

func NewMemory() *Memory { return &Memory{data: map[string][]byte{}} }

var _ domain.BlobStore = (*Memory)(nil)

func memRel(userID, docID string, page int, suffix string) string {
	return filepath.ToSlash(filepath.Join(userID, docID, fmt.Sprintf("%d%s.jpg", page, suffix)))
}

func (m *Memory) Put(userID, docID string, page int, data []byte) (string, error) {
	rel := memRel(userID, docID, page, "")
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.data[rel] = cp
	return rel, nil
}

func (m *Memory) Thumbnail(userID, docID string, page int, data []byte) (string, error) {
	rel := memRel(userID, docID, page, "_thumb")
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.data[rel] = cp
	return rel, nil
}

func (m *Memory) Get(rel string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[rel]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return v, nil
}

func (m *Memory) Delete(rel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, rel)
	return nil
}
