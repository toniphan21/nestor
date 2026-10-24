package nestor

type Theme interface {
	Info(s string) string
	Recommended(s string) string
	Warn(s string) string
	Aware(s string) string
	Important(s string) string
}

type Plain struct{}

func (Plain) Info(s string) string        { return s }
func (Plain) Recommended(s string) string { return s }
func (Plain) Warn(s string) string        { return s }
func (Plain) Aware(s string) string       { return s }
func (Plain) Important(s string) string   { return s }

var theme Theme = Plain{}

func SetTheme(t Theme) {
	if t == nil {
		t = Plain{}
	}
	theme = t
}
