package nestor

import (
	"maps"
	"slices"
	"sort"
)

type auth string

const AuthCredentials = auth("credentials")
const AuthAPIKey = auth("api_key")

const ProfileOptionDockerfile = "dockerfile"
const ProfileOptionContainerHomeDir = "container-home-dir"

type Profile struct {
	Name          string              `yaml:"-"`
	Auth          auth                `yaml:"auth"`
	Proxy         bool                `yaml:"proxy"`
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

func (p *Profile) Model(alias string) string {
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
	if !ok {
		return p.DefaultModel
	}
	return v
}

func (p *Profile) dockerfile(h Harness, r Runtime) string {
	v, ok := p.Options[ProfileOptionDockerfile]
	if !ok {
		return h.DefaultDockerfile(r)
	}
	return v
}
