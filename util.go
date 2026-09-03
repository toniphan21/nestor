package nestor

import (
	"bytes"
	"sync"

	"nhatp.com/go/nestor/infra/fs"
)

// bufferedFile accumulates writes in memory and persists them on Save.
// If nothing was written, Save is a no-op and no file is created.
type bufferedFile struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *bufferedFile) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *bufferedFile) Close() error { return nil }

// Save writes the buffer to path, creating parent dirs.
// Returns false if there was nothing to write.
func (b *bufferedFile) Save(path string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.buf.Len() == 0 {
		return false, nil
	}
	if err := fs.WriteFile(path, b.buf.Bytes()); err != nil {
		return false, err
	}
	return true, nil
}
