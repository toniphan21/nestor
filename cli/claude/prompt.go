package claude

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

	if h, ok := handlers[ev.Type]; ok {
		h(ev, line)
	}
}

var lastAssistantTextMessage string
var lastModel string

// event captures the fields we care about across stream-json event types.
type event struct {
	Type           string `json:"type"`
	Subtype        string `json:"subtype"`
	EstimatedToken int    `json:"estimated_tokens"`
	Message        struct {
		Model   string         `json:"model"`
		Content []contentBlock `json:"content"`
	} `json:"message"`
	Result  string `json:"result"`
	IsError bool   `json:"is_error"`
}

type contentBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// handler renders one event. Register new ones in handlers.
type handler func(e event, line []byte)

var handlers = map[string]handler{
	"assistant": handleAssistant,
	"system":    handleSystem,
	"result":    handleResult,
	// add more types here later, e.g. "user": handleUser
}

func handleAssistant(e event, line []byte) {
	for _, c := range e.Message.Content {
		switch c.Type {
		case "text":
			if lastModel != e.Message.Model {
				lastModel = e.Message.Model
				fmt.Fprintf(os.Stderr, pterm.Gray(fmt.Sprintf(" model: use %s\n", lastModel)))
			}
			lastAssistantTextMessage = c.Text
			fmt.Fprintln(os.Stderr, "")
			fmt.Println(c.Text)
			fmt.Fprintln(os.Stderr, "")

		case "tool_use":
			if e.Message.Model != "" && lastModel != e.Message.Model {
				lastModel = e.Message.Model
				fmt.Fprintf(os.Stderr, pterm.Gray(fmt.Sprintf(" model: use %s\n", lastModel)))
			}

			switch c.Name {
			case "Bash":
				command := c.Input["command"]
				fmt.Fprintf(os.Stderr, pterm.Gray(fmt.Sprintf("  tool: %s %s\n", c.Name, command)))

			case "Read":
				filePath := c.Input["file_path"]
				fmt.Fprintf(os.Stderr, pterm.Gray(fmt.Sprintf("  tool: %s %s\n", c.Name, filePath)))

			case "Edit":
				filePath := c.Input["file_path"]
				fmt.Fprintf(os.Stderr, pterm.Gray(fmt.Sprintf("  tool: %s %s\n", c.Name, filePath)))

			case "Write":
				filePath := c.Input["file_path"]
				if fps, ok := filePath.(string); ok {
					parts := strings.Split(fps, "/")
					if len(parts) > 3 {
						base := strings.Join(parts[0:3], "/") + "/"
						val := strings.TrimPrefix(fps, base)
						fmt.Fprintf(
							os.Stderr,
							fmt.Sprintf("%s%s\n",
								pterm.Gray(fmt.Sprintf("  tool: %s %s", pterm.Blue(c.Name), base)),
								pterm.Blue(val),
							),
						)
					}
					return
				}
				fmt.Fprintf(os.Stderr, pterm.Gray(fmt.Sprintf("  tool: %s %s\n", c.Name, filePath)))

			default:
				fmt.Fprintf(os.Stderr, pterm.Gray(fmt.Sprintf("  tool: %s %s\n", e.Message.Model, c.Name)))

			}
		}
	}
}

func handleSystem(e event, line []byte) {
	switch e.Subtype {
	case "thinking_tokens":
		fmt.Fprint(os.Stderr, pterm.Gray(fmt.Sprintf("system: thinking - estimated tokens %d\n", e.EstimatedToken)))
	default:
		fmt.Fprintln(os.Stderr, pterm.Gray("system: "+e.Subtype))
	}
}

func handleResult(e event, line []byte) {
	if e.Result != lastAssistantTextMessage {
		fmt.Fprintln(os.Stderr, "")
		fmt.Println(e.Result)
		fmt.Fprintln(os.Stderr, "")
	}
}
