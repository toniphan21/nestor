package nestor

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

func (s *Profile) Model(alias string) string {
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

func (s *Profile) dockerfile(h Harness, r Runtime) string {
	v, ok := s.Options[ProfileOptionDockerfile]
	if !ok {
		return h.DefaultDockerfile(r)
	}
	return v
}
