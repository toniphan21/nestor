package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/infra/fs"
)

var ErrInterrupted = errors.New("interrupted")

func Input(label string, initial string) (string, error) {
	if label == "" {
		label = "Input text"
	}

	interrupted := false
	textInput := pterm.DefaultInteractiveTextInput.WithMultiLine().
		WithDefaultText(label).
		WithOnInterruptFunc(func() { interrupted = true }).
		WithDefaultValue(initial)

	result, err := textInput.Show()
	if err != nil {
		return "", err
	}
	if interrupted {
		return "", ErrInterrupted
	}
	return strings.TrimSpace(result), nil
}

func SelectSandboxSpec(specs []nestor.SandboxSpec) (nestor.SandboxSpec, error) {
	optionText := func(i int, spec nestor.SandboxSpec) string {
		return fmt.Sprintf(
			"%d. %s - harness %s - profile %s",
			i+1,
			pterm.Green(spec.Name),
			pterm.Magenta(spec.Harness),
			pterm.Red(spec.Profile),
		)
	}

	result, err := Select(specs, "Select sandbox spec", optionText)
	if err != nil {
		return nestor.SandboxSpec{}, err
	}
	return result.Value, nil
}

type SelectResult[T any] struct {
	Value T
	Index int
}

const defaultSelectHeight = 10

func SingleSelect[T any](list []T, printer *pterm.InteractiveSelectPrinter, option func(int, T) string) (*SelectResult[T], error) {
	optionIndex := func(text string, n int) (int, error) {
		i := strings.IndexByte(text, '.')
		if i < 0 {
			return 0, fmt.Errorf("no number found in %q", text)
		}
		num, err := strconv.Atoi(strings.TrimSpace(text[:i]))
		if err != nil {
			return 0, err
		}
		if num < 1 || num > n {
			return 0, fmt.Errorf("out of range: %d", num)
		}
		return num - 1, nil
	}

	var options []string
	for i, v := range list {
		options = append(options, option(i, v))
	}

	selectedOption, err := printer.WithOptions(options).Show()
	if err != nil {
		return nil, err
	}

	idx, err := optionIndex(selectedOption, len(list))
	if err != nil {
		return nil, err
	}
	return &SelectResult[T]{Value: list[idx], Index: idx}, nil
}

func Select[T any](list []T, text string, option func(int, T) string) (*SelectResult[T], error) {
	printer := pterm.DefaultInteractiveSelect.WithDefaultText(text).WithMaxHeight(defaultSelectHeight)

	return SingleSelect(list, printer, option)
}

func collectSpec(api nestor.API, filteredName string) (*nestor.SandboxSpec, error) {
	allSpecs := api.Runtime().Registry.SandboxSpecs()
	if len(allSpecs) == 0 {
		return nil, fmt.Errorf("%w: no sandbox specs", nestor.ErrNotFound)
	}

	if filteredName != "" {
		for _, v := range allSpecs {
			if v.Name != filteredName {
				continue
			}
			return &v, nil
		}
		return nil, fmt.Errorf("%w: sandbox spec %q", nestor.ErrNotFound, filteredName)
	}

	slices.SortFunc(allSpecs, func(a nestor.SandboxSpec, b nestor.SandboxSpec) int {
		return strings.Compare(a.Name, b.Name)
	})

	var err error
	var spec nestor.SandboxSpec
	if len(allSpecs) > 1 {
		spec, err = SelectSandboxSpec(allSpecs)
		if err != nil {
			return nil, err
		}
	} else {
		spec = allSpecs[0]
		fmt.Printf("use sandbox spec %s - harness %s - profile %s\n", pterm.Green(spec.Name), pterm.Magenta(spec.Harness), pterm.Red(spec.Profile))
	}
	return &spec, nil
}

func collectPath(spec nestor.SandboxSpec, filteredPath string) (*string, error) {
	paths := spec.Paths()
	if len(paths) == 0 {
		return nil, fmt.Errorf("%w: no mounts in spec %q", nestor.ErrNotFound, spec.Name)
	}

	if filteredPath != "" {
		for _, v := range paths {
			if v != filteredPath {
				continue
			}
			return &v, nil
		}
		return nil, fmt.Errorf("%w: path %q", nestor.ErrNotFound, filteredPath)
	}

	var selectedPath string
	if len(paths) > 1 {
		r, err := Select(paths, "Select path", func(i int, s string) string {
			return fmt.Sprintf("%d. %s", i+1, s)
		})
		if err != nil {
			return nil, err
		}
		selectedPath = r.Value
	} else {
		selectedPath = paths[0]
		fmt.Printf("use path %s\n", pterm.Cyan(selectedPath))
	}
	return &selectedPath, nil
}

func collectSpecAndPath(api nestor.API, filteredSpec, filteredPath string) (nestor.SandboxSpec, string, error) {
	var spec nestor.SandboxSpec
	var path string

	if v, err := collectSpec(api, filteredSpec); err != nil || v == nil {
		return spec, path, err
	} else {
		spec = *v
	}

	if v, err := collectPath(spec, filteredPath); err != nil || v == nil {
		return spec, path, err
	} else {
		path = *v
	}
	return spec, path, nil
}

func freeSandboxes(api nestor.API, spec nestor.SandboxSpec, path string) ([]nestor.Sandbox, error) {
	allSandboxes, err := spec.ListSandboxes(api.Runtime())
	if err != nil {
		return nil, err
	}

	var sandboxes []nestor.Sandbox
	for _, v := range allSandboxes {
		if !v.IsFree(path) {
			continue
		}
		sandboxes = append(sandboxes, v)
	}

	if len(sandboxes) == 0 {
		return nil, fmt.Errorf("%w: no free sandbox", nestor.ErrNotAvailable)
	}
	return sandboxes, nil
}

func collectFreeSandboxes(api nestor.API, spec nestor.SandboxSpec, path string, filtered ...string) ([]nestor.Sandbox, error) {
	sandboxes, err := freeSandboxes(api, spec, path)
	if err != nil {
		return nil, err
	}

	if len(filtered) > 0 {
		allowed := make(map[string]bool)
		for _, v := range filtered {
			if v != "" {
				allowed[v] = true
			}
		}

		if len(allowed) > 0 {
			var out []nestor.Sandbox
			for _, v := range sandboxes {
				if allowed[v.ID()] {
					out = append(out, v)
				}
			}

			if len(out) == 0 {
				return nil, fmt.Errorf("%w: no free sandbox matched filtered", nestor.ErrNotFound)
			}
			return out, nil
		}
	}

	if len(sandboxes) == 1 {
		return sandboxes, nil
	}

	options := []string{pterm.Cyan("any sandbox")}
	for _, v := range sandboxes {
		options = append(options, v.ID())
	}

	r, err := Select(options, "Select sandbox", func(i int, s string) string {
		return fmt.Sprintf("%d. %s", i+1, s)
	})
	if err != nil {
		return nil, err
	}

	if r.Index == 0 {
		return sandboxes, nil
	}
	return []nestor.Sandbox{sandboxes[r.Index-1]}, nil
}

func collectModel(api nestor.API, spec nestor.SandboxSpec, label, filteredModel string) (string, error) {
	profile, have := api.Runtime().Registry.Profile(spec.Profile)
	if !have {
		return "", fmt.Errorf("%w: profile %q", nestor.ErrNotFound, spec.Profile)
	}

	models := profile.SupportedModelsWithoutDefault()
	var selectedModel string
	if len(models) == 0 {
		selectedModel = profile.DefaultModel
		fmt.Printf("use model %s\n", pterm.Cyan(selectedModel))
		return selectedModel, nil
	}

	models = slices.Insert(models, 0, profile.DefaultModel)
	if filteredModel != "" {
		for _, v := range models {
			check := filteredModel
			mn, ok := profile.ModelByAlias(filteredModel)
			if ok {
				check = mn
			}

			if v != check {
				continue
			}
			selectedModel = v
		}

		if selectedModel == "" {
			return "", fmt.Errorf("%w: model %q", nestor.ErrNotFound, filteredModel)
		}
		return selectedModel, nil
	}

	if label == "" {
		label = "Select model"
	}
	printer := pterm.DefaultInteractiveSelect.
		WithDefaultText(label).
		WithMaxHeight(len(models) + 1)

	r, err := SingleSelect(models, printer, func(i int, s string) string {
		return fmt.Sprintf("%d. %s", i+1, s)
	})
	if err != nil {
		return "", err
	}
	selectedModel = r.Value

	return selectedModel, nil
}

func collectPrompt(sandbox nestor.Sandbox, path string, promptFile string) (string, error) {
	if promptFile != "" {
		b, err := fs.AtomicReadFile(promptFile)
		if err != nil {
			return "", err
		}

		if err = fs.RemoveFile(promptFile); err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}

	editor := defaultEditor()
	if editor == "" {
		return Input("Prompt", "")
	}

	_, hwd, have := sandbox.ResolveWorkDir(path)
	if !have {
		return Input("Prompt", "")
	}

	f, err := os.CreateTemp(path, "nestor-prompt-*.md")
	if err != nil {
		return "", err
	}
	fp := f.Name()

	cmd := exec.Command("sh", "-c", editor+` "$@"`, "--", fp)
	cmd.Dir = hwd
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil { // blocks until the editor exits
		return "", fmt.Errorf("editor: %w", err)
	}

	b, err := fs.AtomicReadFile(fp)
	if err != nil {
		return "", err
	}

	if err = fs.RemoveFile(fp); err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func escapeCLIFlagValue(val string) string {
	if val == "" {
		return "''"
	}
	if !strings.ContainsFunc(val, needsQuoting) {
		return val
	}
	// Single quotes are fully literal in POSIX shells. Nothing inside them
	// needs escaping, but a single quote can't appear inside them either.
	if !strings.Contains(val, "'") {
		return "'" + val + "'"
	}
	// Value contains ' → use double quotes and escape the characters
	// that stay special inside them: \ " $ `
	var b strings.Builder
	b.Grow(len(val) + 2)
	b.WriteByte('"')
	for _, r := range val {
		switch r {
		case '\\', '"', '$', '`':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

func needsQuoting(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	switch r {
	case '-', '_', '.', '/', ':', '=', '@', '%', '+', ',':
		return false
	}
	return true
}

type sessionOptionItem struct {
	sandbox nestor.Sandbox
	session nestor.HarnessSession
	text    string
}

type sessionCollector struct {
	path               string
	sandboxes          []nestor.Sandbox
	displaySandboxName bool
	timeout            time.Duration
	filtered           string
	newSessionTitle    string
	selectLabel        string
	titleInputLabel    string
}

func (c *sessionCollector) collect() (sandbox nestor.Sandbox, sessionID string, title string, err error) {
	if len(c.sandboxes) == 0 {
		err = fmt.Errorf("%w: no sandboxes to pick session", nestor.ErrInvalid)
		return
	}

	if c.filtered == "-" {
		sandbox = c.sandboxes[0]
		sessionID = ""
		title = c.newSessionTitle
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	var items []sessionOptionItem
	for _, v := range c.sandboxes {
		sessions := v.Harness().ListSessions(ctx, c.path, v)
		for _, vv := range sessions {
			t := vv.Title
			if t == "" {
				t = "last opened: " + vv.UpdatedAt.Local().Format("2006-01-02 15:04")
			}
			var text string
			if c.displaySandboxName {
				text = fmt.Sprintf("sandbox %s - %s - %s", pterm.Green(v.ID()), vv.ID, t)
			} else {
				text = fmt.Sprintf("%s - %s", vv.ID, t)
			}
			items = append(items, sessionOptionItem{
				sandbox: v,
				session: vv,
				text:    text,
			})
		}
	}

	if c.filtered != "" {
		for _, v := range items {
			if v.session.ID != c.filtered {
				continue
			}

			sandbox = v.sandbox
			sessionID = v.session.ID
			title = v.session.Title
			return
		}

		err = fmt.Errorf("%w: session %q", nestor.ErrNotFound, c.filtered)
		return
	}

	if len(items) > 0 {
		if c.displaySandboxName {
			var nsi []sessionOptionItem
			for _, v := range c.sandboxes {
				newSessionText := fmt.Sprintf("sandbox %s - New session", pterm.Green(v.ID()))
				nsi = append(nsi, sessionOptionItem{text: newSessionText, sandbox: v, session: nestor.HarnessSession{}})
			}
			items = slices.Insert(items, 0, nsi...)
		} else {
			items = slices.Insert(items, 0,
				sessionOptionItem{text: "New session", sandbox: c.sandboxes[0], session: nestor.HarnessSession{}},
			)
		}

		printer := pterm.DefaultInteractiveSelect.
			WithDefaultText(c.selectLabel).
			WithMaxHeight(len(items) + 1)

		var selectResult *SelectResult[sessionOptionItem]
		selectResult, err = SingleSelect(items, printer, func(i int, s sessionOptionItem) string {
			return fmt.Sprintf("%3d. %s", i+1, s.text)
		})
		if err != nil {
			return
		}

		sandbox = selectResult.Value.sandbox
		sessionID = selectResult.Value.session.ID
		title = selectResult.Value.session.Title
	}

	if sessionID == "" {
		t := c.titleInputLabel
		var input string
		input, err = pterm.DefaultInteractiveTextInput.WithDefaultText(t).Show()
		if err != nil {
			return
		}
		title = input
	}
	return
}
