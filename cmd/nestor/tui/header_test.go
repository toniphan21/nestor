package tui

import "testing"

func Test_actionBar_View(t *testing.T) {
	const sep = " | "

	tests := []struct {
		name      string
		actions   []action
		separator string
		width     string // length of this string is the bar width
		want      string
	}{
		{
			name:      "no actions",
			actions:   nil,
			separator: sep,
			width:     "----------",
			want:      "",
		},
		{
			name:      "zero width",
			actions:   []action{{keys: []string{"d"}, desc: "down", priority: 1, order: 0}},
			separator: sep,
			width:     "",
			want:      "",
		},
		{
			name: "everything fits, laid out by order",
			actions: []action{
				{keys: []string{"d"}, desc: "down", priority: 1, order: 0},
				{keys: []string{"v"}, desc: "view", priority: 2, order: 1},
			},
			separator: sep,
			width:     "----------------", // 16: "d down" + " | " + "v view"
			want:      "d down | v view",
		},
		{
			name: "renders multiple keys joined with /",
			actions: []action{
				{keys: []string{"j", "k"}, desc: "move", priority: 1, order: 0},
			},
			separator: sep,
			width:     "--------", // 8: "j/k move"
			want:      "j/k move",
		},
		{
			name: "not enough space hides lowest priority first",
			actions: []action{
				{keys: []string{"d"}, desc: "down", priority: 1, order: 0}, // lower priority, hidden
				{keys: []string{"q"}, desc: "quit", priority: 2, order: 1},
			},
			separator: sep,
			width:     "--------", // 8: fits only one of "d down"/"q quit" (6) + sep (3) + other (6) = 15
			want:      "q quit",
		},
		{
			name: "same priority hides lowest order first (bar is right-aligned)",
			actions: []action{
				{keys: []string{"d"}, desc: "down", priority: 1, order: 0}, // lowest order, hidden
				{keys: []string{"v"}, desc: "view", priority: 1, order: 1},
			},
			separator: sep,
			width:     "--------", // 8: fits only one of "d down"/"v view" (6) + sep (3) + other (6) = 15
			want:      "v view",
		},
		{
			name: "display order follows the order field, not input or priority",
			actions: []action{
				{keys: []string{"x"}, desc: "delete", priority: 3, order: 2},
				{keys: []string{"d"}, desc: "down", priority: 1, order: 0},
				{keys: []string{"v"}, desc: "view", priority: 2, order: 1},
			},
			separator: sep,
			width:     "---------------------------", // 27: "d down" + " | " + "v view" + " | " + "x delete"
			want:      "d down | v view | x delete",
		},
		{
			name:      "custom separator",
			separator: "  ",
			actions: []action{
				{keys: []string{"d"}, desc: "down", priority: 1, order: 0},
				{keys: []string{"v"}, desc: "view", priority: 2, order: 1},
			},
			width: "---------------", // 15: "d down" + "  " + "v view"
			want:  "d down  v view",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := actionBar{
				separator: tt.separator,
				width:     len(tt.width),
				actions:   tt.actions,
			}

			if got := b.View(); got != tt.want {
				t.Errorf("View() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_actionBar_RenderAction(t *testing.T) {
	tests := []struct {
		name string
		in   action
		want string
	}{
		{
			name: "single key and desc",
			in:   action{keys: []string{"d"}, desc: "down"},
			want: "d down",
		},
		{
			name: "multiple keys joined with /",
			in:   action{keys: []string{"j", "k"}, desc: "move"},
			want: "j/k move",
		},
		{
			name: "keys are passed through verbatim, client owns the coloring",
			in:   action{keys: []string{"\x1b[34md\x1b[0m"}, desc: "down"},
			want: "\x1b[34md\x1b[0m down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := actionBar{}
			if got := b.RenderAction(tt.in); got != tt.want {
				t.Errorf("renderAction() = %q, want %q", got, tt.want)
			}
		})
	}
}
