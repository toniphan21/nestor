package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
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

func Select[T any](list []T, defaultText string, option func(int, T) string) (*SelectResult[T], error) {
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

	selectedOption, err := pterm.DefaultInteractiveSelect.WithOptions(options).WithDefaultText(defaultText).Show()
	if err != nil {
		return nil, err
	}

	idx, err := optionIndex(selectedOption, len(list))
	if err != nil {
		return nil, err
	}
	return &SelectResult[T]{Value: list[idx], Index: idx}, nil
}
