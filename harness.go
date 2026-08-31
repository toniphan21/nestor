package nestor

import (
	"log/slog"
)

type harness string

const HarnessClaudeCode = harness("claude")
const HarnessOpenCode = harness("opencode")

type HarnessSpec struct {
	Name          string              `yaml:"-"`
	Dockerfile    string              `yaml:"dockerfile,omitempty"`
	Targets       []string            `yaml:"targets"`
	Models        map[string][]string `yaml:"models"` // name -> aliases
	DefaultTarget string              `yaml:"default_target"`
	DefaultModel  string              `yaml:"default_model"`
	Options       map[string]string   `yaml:"options,omitempty"`

	modelAliases map[string]string
}

func (s *HarnessSpec) Model(alias string) string {
	if s.modelAliases == nil {
		s.modelAliases = map[string]string{}
		for m, aliases := range s.Models {
			for _, v := range aliases {
				s.modelAliases[v] = m
			}
		}
	}

	v, ok := s.modelAliases[alias]
	if !ok {
		return s.DefaultModel
	}
	return v
}

type Harness interface {
	Name() string

	Spec(ctx Context) (HarnessSpec, error)

	Init(ctx Context, dir string, log *slog.Logger) error
}
