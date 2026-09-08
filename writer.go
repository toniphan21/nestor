package nestor

import (
	"bytes"
	"io"
)

const DefaultLineBufferSize = 4 * 1024 * 1024

func NewLineSplitter(max int, handlers ...func(line []byte)) io.Writer {
	if max <= 0 {
		max = DefaultLineBufferSize
	}
	return &lineSplitter{max: max, handlers: handlers}
}

type lineSplitter struct {
	buf      bytes.Buffer
	max      int
	handlers []func([]byte)
}

func (w *lineSplitter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		b := w.buf.Bytes()
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			if w.buf.Len() > w.max {
				w.buf.Reset()
			}
			return len(p), nil
		}
		line := bytes.Clone(w.buf.Next(i + 1)[:i])

		for _, h := range w.handlers {
			h(line)
		}
	}
}

// ---

func newSessionIDCapture(capturer func(line []byte) (string, bool)) *sessionIDCapture {
	if capturer == nil {
		return nil
	}
	return &sessionIDCapture{capturer: capturer}
}

type sessionIDCapture struct {
	buf      []byte
	id       string
	capturer func(line []byte) (string, bool)
}

func (c *sessionIDCapture) Write(p []byte) (int, error) {
	if c.id != "" {
		return len(p), nil
	}
	c.buf = append(c.buf, p...)
	for {
		i := bytes.IndexByte(c.buf, '\n')
		if i < 0 {
			break
		}

		line := bytes.Clone(c.buf[:i])
		c.buf = c.buf[i+1:]

		v, ok := c.capturer(line)
		if ok {
			c.id = v
			c.buf = nil
			return len(p), nil
		}
	}
	return len(p), nil
}

func (c *sessionIDCapture) SessionID() string { return c.id }
