package opencode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

func NewWriter() io.Writer {
	return &jsonlWriter{
		max: 4 * 1024 * 1024,
	}
}

type jsonlWriter struct {
	buf bytes.Buffer
	max int // guard against a runaway line

	lastSessionID string
}

func (w *jsonlWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		b := w.buf.Bytes()
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			if w.buf.Len() > w.max {
				w.buf.Reset() // drop the oversized line, keep the stream alive
			}
			return len(p), nil
		}
		line := w.buf.Next(i + 1)[:i]
		w.emit(line)
	}
}

func (w *jsonlWriter) emit(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	var ev event
	if err := json.Unmarshal(line, &ev); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}

	h, ok := handlers[ev.Type]
	if ok {
		h(w, ev, line)
	} else {
		if ev.Type != "step_finish" {
			fmt.Fprintln(os.Stderr, pterm.Red(ev.Type))
		}
	}
}

// event captures the fields we care about across stream-json event types.
type event struct {
	Type      string    `json:"type"`
	Timestamp int64     `json:"timestamp"`
	SessionID string    `json:"sessionID"`
	Part      partBlock `json:"part"`
}

type partBlock struct {
	Type  string      `json:"type"`
	Tool  string      `json:"tool"`
	Text  string      `json:"text"`
	State *stateBlock `json:"state"`
}

type stateBlock struct {
	Input map[string]any `json:"input"`
}

type handler func(w *jsonlWriter, e event, line []byte)

var handlers = map[string]handler{
	"step_start": handleStepStart,
	"text":       handleText,
	"tool_use":   handleToolUse,
}

func handleStepStart(w *jsonlWriter, e event, line []byte) {
	if w.lastSessionID != e.SessionID {
		w.lastSessionID = e.SessionID
		fmt.Fprint(os.Stderr, pterm.Gray(fmt.Sprintf("system: init - session ID %s\n", w.lastSessionID)))
	}
}

func handleText(w *jsonlWriter, e event, line []byte) {
	text := strings.TrimSpace(e.Part.Text)
	if text == "" {
		return
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Println(text)
	fmt.Fprintln(os.Stderr, "")
}

func handleToolUse(w *jsonlWriter, e event, line []byte) {
	switch e.Part.Type {
	case "tool":
		switch e.Part.Tool {
		case "bash":
			command := e.Part.State.Input["command"]
			fmt.Fprint(os.Stderr, pterm.Gray(fmt.Sprintf("  tool: %s %s\n", e.Part.Tool, command)))

		case "grep":
			pattern := e.Part.State.Input["pattern"]
			fmt.Fprint(os.Stderr, pterm.Gray(fmt.Sprintf("  tool: %s %s\n", e.Part.Tool, pattern)))

		case "read":
			filePath := e.Part.State.Input["filePath"]
			fmt.Fprint(os.Stderr, pterm.Gray(fmt.Sprintf("  tool: %s %s\n", e.Part.Tool, filePath)))

		case "edit":
			filePath := e.Part.State.Input["filePath"]
			fmt.Fprint(os.Stderr, pterm.Gray(fmt.Sprintf("  tool: %s %s\n", e.Part.Tool, filePath)))
			fmt.Fprint(os.Stderr, pterm.Red(e.Part.State.Input["oldString"]))
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprint(os.Stderr, pterm.Green(e.Part.State.Input["newString"]))
			fmt.Fprintln(os.Stderr, "")
		}
	}
}
