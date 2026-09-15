package nestor

import (
	"bufio"
	"bytes"
	"io"
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

func multiWriter(w ...io.Writer) io.Writer {
	var args []io.Writer
	for _, v := range w {
		if v != nil {
			args = append(args, v)
		}
	}
	return io.MultiWriter(args...)
}

func dedup[T comparable](list []T) []T {
	seen := make(map[T]bool)
	var out []T
	for _, v := range list {
		if _, have := seen[v]; have {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// scanLines calls fn for each line. Lines grow unbounded; no size limit.
func scanLines(r io.Reader, fn func([]byte)) error {
	br := bufio.NewReader(r)

	for {
		line, err := br.ReadBytes('\n')

		if line = bytes.TrimRight(line, "\r\n"); len(line) > 0 {
			fn(line)
		}

		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
