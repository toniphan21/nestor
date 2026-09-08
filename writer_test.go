package nestor

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func collectLineSplitterHandler() (func([]byte), *[][]byte) {
	var got [][]byte
	return func(line []byte) {
		got = append(got, line)
	}, &got
}

func TestLineSplitter(t *testing.T) {
	tests := []struct {
		name   string
		max    int
		writes []string
		want   []string
	}{
		{
			name:   "single line",
			writes: []string{"hello\n"},
			want:   []string{"hello"},
		},
		{
			name:   "several lines in one write",
			writes: []string{"a\nb\nc\n"},
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "line split across writes",
			writes: []string{"hel", "lo\n"},
			want:   []string{"hello"},
		},
		{
			name:   "trailing partial line not emitted",
			writes: []string{"a\nbc"},
			want:   []string{"a"},
		},
		{
			name:   "empty line",
			writes: []string{"\na\n"},
			want:   []string{"", "a"},
		},
		{
			name:   "carriage return retained",
			writes: []string{"a\r\n"},
			want:   []string{"a\r"},
		},
		{
			name:   "empty write",
			writes: []string{""},
			want:   nil,
		},
		{
			name:   "buffer at max is kept",
			max:    4,
			writes: []string{"abcd", "\n"},
			want:   []string{"abcd"},
		},
		{
			name:   "buffer over max is dropped",
			max:    4,
			writes: []string{"abcdefghij", "xyz\n"},
			want:   []string{"xyz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, got := collectLineSplitterHandler()
			w := NewLineSplitter(tt.max, h)
			for _, s := range tt.writes {
				n, err := w.Write([]byte(s))
				if err != nil {
					t.Fatalf("Write(%q) error = %v", s, err)
				}
				if n != len(s) {
					t.Fatalf("Write(%q) = %d, want %d", s, n, len(s))
				}
			}
			if len(*got) != len(tt.want) {
				t.Fatalf("got %q, want %q", *got, tt.want)
			}
			for i, line := range *got {
				if string(line) != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, line, tt.want[i])
				}
			}
		})
	}
}

func TestLineSplitterByteAtATime(t *testing.T) {
	h, got := collectLineSplitterHandler()
	w := NewLineSplitter(0, h)
	for _, b := range []byte("ab\ncd\n") {
		if _, err := w.Write([]byte{b}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*got) != 2 || string((*got)[0]) != "ab" || string((*got)[1]) != "cd" {
		t.Errorf("got %q, want [ab cd]", *got)
	}
}

func TestLineSplitterHandlersInOrder(t *testing.T) {
	var order []string
	w := NewLineSplitter(0,
		func([]byte) { order = append(order, "first") },
		func([]byte) { order = append(order, "second") },
	)
	w.Write([]byte("x\n"))
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("got %v, want [first second]", order)
	}
}

func TestLineSplitterNoHandlers(t *testing.T) {
	w := NewLineSplitter(0)
	if _, err := w.Write([]byte("a\nb\n")); err != nil {
		t.Fatal(err)
	}
}

func TestLineSplitterDefaultMax(t *testing.T) {
	h, got := collectLineSplitterHandler()
	w := NewLineSplitter(-1, h)
	long := strings.Repeat("x", DefaultLineBufferSize/2)
	w.Write([]byte(long + "\n"))
	if len(*got) != 1 || len((*got)[0]) != len(long) {
		t.Errorf("long line under default max was not emitted intact")
	}
}

func TestLineSplitterLinesAreIndependent(t *testing.T) {
	var got [][]byte
	w := NewLineSplitter(0, func(line []byte) { got = append(got, line) })

	// enough traffic to force the buffer to reuse or grow its backing array
	for i := range 100 {
		fmt.Fprintf(w, "line-%02d\n", i)
	}

	for i, line := range got {
		want := fmt.Sprintf("line-%02d", i)
		if string(line) != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}
}

func TestSessionIDCapture(t *testing.T) {
	// captures on a line equal to "id:<value>"
	capturer := func(line []byte) (string, bool) {
		v, ok := bytes.CutPrefix(line, []byte("id:"))
		return string(v), ok
	}

	tests := []struct {
		name   string
		writes []string
		want   string
	}{
		{
			name:   "single write single line",
			writes: []string{"id:abc\n"},
			want:   "abc",
		},
		{
			name:   "skips non-matching lines",
			writes: []string{"noise\nmore noise\nid:abc\n"},
			want:   "abc",
		},
		{
			name:   "line split across writes",
			writes: []string{"id:a", "bc\n"},
			want:   "abc",
		},
		{
			name:   "several lines in one write",
			writes: []string{"x\nid:abc\ny\n"},
			want:   "abc",
		},
		{
			name:   "first match wins",
			writes: []string{"id:abc\nid:xyz\n"},
			want:   "abc",
		},
		{
			name:   "ignores writes after capture",
			writes: []string{"id:abc\n", "id:xyz\n"},
			want:   "abc",
		},
		{
			name:   "no trailing newline is not a line",
			writes: []string{"id:abc"},
			want:   "",
		},
		{
			name:   "no match",
			writes: []string{"noise\n"},
			want:   "",
		},
		{
			name:   "empty write",
			writes: []string{""},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newSessionIDCapture(capturer)
			for _, w := range tt.writes {
				n, err := c.Write([]byte(w))
				if err != nil {
					t.Fatalf("Write(%q) error = %v", w, err)
				}
				if n != len(w) {
					t.Fatalf("Write(%q) = %d, want %d", w, n, len(w))
				}
			}
			if got := c.SessionID(); got != tt.want {
				t.Errorf("SessionID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSessionIDCaptureNilCapturer(t *testing.T) {
	if c := newSessionIDCapture(nil); c != nil {
		t.Errorf("newSessionIDCapture(nil) = %v, want nil", c)
	}
}

func TestSessionIDCaptureByteAtATime(t *testing.T) {
	c := newSessionIDCapture(func(line []byte) (string, bool) {
		v, ok := bytes.CutPrefix(line, []byte("id:"))
		return string(v), ok
	})
	for _, b := range []byte("noise\nid:abc\n") {
		if _, err := c.Write([]byte{b}); err != nil {
			t.Fatal(err)
		}
	}
	if got := c.SessionID(); got != "abc" {
		t.Errorf("SessionID() = %q, want \"abc\"", got)
	}
}

func TestSessionIDCaptureCapturerNotCalledAfterMatch(t *testing.T) {
	calls := 0
	c := newSessionIDCapture(func(line []byte) (string, bool) {
		calls++
		return "abc", true
	})
	c.Write([]byte("first\n"))
	c.Write([]byte("second\nthird\n"))
	if calls != 1 {
		t.Errorf("capturer called %d times, want 1", calls)
	}
}
