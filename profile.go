package nestor

import (
	"maps"
	"slices"
	"sort"
)

type auth string

const (
	ProfileOptionDockerfile       = "dockerfile"
	ProfileOptionContainerHomeDir = "container-home-dir"
)

type Profile struct {
	Name          string              `yaml:"-"`
	Settings      map[string]string   `yaml:"settings,omitempty"`
	Targets       []string            `yaml:"targets"`
	Models        map[string][]string `yaml:"models"` // name -> aliases
	DefaultTarget string              `yaml:"default_target"`
	DefaultModel  string              `yaml:"default_model"`
	Options       map[string]string   `yaml:"options,omitempty"`

	modelAliases map[string]string
}

func (p *Profile) SupportedModels() []string {
	models := slices.Collect(maps.Keys(p.Models))
	sort.Strings(models)

	return models
}

func (p *Profile) SupportedModelsWithoutDefault() []string {
	var out []string
	for _, v := range p.SupportedModels() {
		if v == p.DefaultModel {
			continue
		}
		out = append(out, v)
	}
	return out
}

func (p *Profile) ModelByAlias(alias string) (string, bool) {
	if p.modelAliases == nil {
		p.modelAliases = map[string]string{}
		for m, aliases := range p.Models {
			p.modelAliases[m] = m
			for _, v := range aliases {
				p.modelAliases[v] = m
			}
		}
	}

	v, ok := p.modelAliases[alias]
	return v, ok
}

func (p *Profile) Model(alias string) string {
	v, ok := p.ModelByAlias(alias)
	if !ok {
		return p.DefaultModel
	}
	return v
}

func (p *Profile) GetOption(name string, fn func() string) string {
	if p.Options == nil {
		if fn != nil {
			return fn()
		}
		return ""
	}
	v, ok := p.Options[name]
	if !ok && fn != nil {
		return fn()
	}
	if v == "" && fn != nil {
		return fn()
	}
	return v
}

func (p *Profile) GetSetting(name string) (string, bool) {
	if p.Settings == nil {
		return "", false
	}
	v, ok := p.Settings[name]
	return v, ok
}
