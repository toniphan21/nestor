package tui

type basePage struct {
	busy    bool
	focused bool
	gen     int
	width   int
	height  int
}

func (p *basePage) Focus() {
	p.focused = true
	p.gen++
}

func (p *basePage) Blur() {
	p.focused = false
	p.gen++
}

func (p *basePage) Busy() bool {
	return p.busy
}

func (p *basePage) SetSize(w int, h int) {
	p.width = w
	p.height = h
}
