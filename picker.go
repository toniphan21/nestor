package nestor

import (
	"bufio"
	"fmt"
	"io"
	randv2 "math/rand/v2"
	"strings"
	"sync"
)

type Picker interface {
	Pick() string
}

func NewPickerWithList(list []string) (Picker, error) {
	if len(list) == 0 {
		return nil, fmt.Errorf("%w: empty list", ErrInvalid)
	}

	p := &picker{list: list}
	p.shuffle()
	return p, nil
}

func NewPickerWithReader(reader io.Reader) (Picker, error) {
	var lines []string
	sc := bufio.NewScanner(reader)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			lines = append(lines, l)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return NewPickerWithList(lines)
}

type picker struct {
	mu   sync.Mutex
	list []string
	idx  int
}

func (p *picker) Pick() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := p.list[p.idx]
	p.idx++
	if p.idx == len(p.list) {
		p.idx = 0
		p.shuffle()
	}
	return n
}

func (p *picker) shuffle() {
	randv2.Shuffle(len(p.list), func(i, j int) {
		p.list[i], p.list[j] = p.list[j], p.list[i]
	})
}
