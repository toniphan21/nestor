package nestor

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultVersion = "1"

type ymlFile[T any] struct {
	Version string `yaml:"version"`
	Type    string `yaml:"type"`
	Data    T      `yaml:"data"`
}

func parseYML[T, R any](yml io.Reader, typ string, callback func(*ymlFile[T]) (R, error), versions ...string) (R, error) {
	var file ymlFile[T]
	dec := yaml.NewDecoder(yml)
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil {
		dv, _ := callback(nil)
		return dv, err
	}

	if file.Type != typ {
		dv, _ := callback(nil)
		return dv, fmt.Errorf("%w: type %q, want %q", ErrNotSupported, file.Type, typ)
	}

	if len(versions) == 0 {
		versions = []string{defaultVersion}
	}

	hasVersion := slices.Contains(versions, file.Version)

	if !hasVersion {
		dv, _ := callback(nil)
		return dv, fmt.Errorf("%w: version %q, want %q", ErrNotSupported, file.Version, versions)
	}
	return callback(&file)
}

const profileType = "profile"

func ParseProfiles(yml io.Reader) ([]Profile, error) {
	return parseYML(yml, profileType, func(file *ymlFile[map[string]Profile]) ([]Profile, error) {
		if file == nil {
			return nil, nil
		}

		result := make([]Profile, 0, len(file.Data))
		for k, v := range file.Data {
			v.Name = k
			result = append(result, v)
		}
		slices.SortFunc(result, func(a, b Profile) int {
			return strings.Compare(a.Name, b.Name)
		})
		return result, nil
	})
}

const sandboxSpecType = "sandbox-spec"

func ParseSandboxSpecs(runtime Runtime, yml io.Reader) ([]SandboxSpec, error) {
	return parseYML(yml, sandboxSpecType, func(file *ymlFile[map[string]SandboxSpec]) ([]SandboxSpec, error) {
		if file == nil {
			return nil, nil
		}

		result := make([]SandboxSpec, 0, len(file.Data))
		for k, v := range file.Data {
			v.Name = k

			p := v.Profile
			v.profileInYaml = new(p)
			if v.Profile == "" {
				v.Profile = string(v.Harness)
			}

			t := v.Target
			v.targetInYaml = new(t)

			if err := v.Validate(runtime); err != nil {
				return nil, fmt.Errorf("%w: sandbox %q", err, k)
			}
			result = append(result, v)
		}
		slices.SortFunc(result, func(a, b SandboxSpec) int {
			return strings.Compare(a.Name, b.Name)
		})
		return result, nil
	})
}

func MarshallSandboxSpecs(specs []SandboxSpec) ([]byte, error) {
	data := make(map[string]SandboxSpec)
	for _, spec := range specs {
		data[spec.Name] = spec
	}
	f := &ymlFile[map[string]SandboxSpec]{
		Version: defaultVersion,
		Type:    sandboxSpecType,
		Data:    data,
	}
	return yaml.Marshal(f)
}

const mcpType = "mcp"

type mcpServer struct {
	Type     string            `yaml:"type"`
	Command  []string          `yaml:"command,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	URL      string            `yaml:"url,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty"`
	UseTools []string          `yaml:"use_tools,omitempty"`
}

func ParseMCPs(yml io.Reader) ([]MCP, error) {
	return parseYML(yml, mcpType, func(file *ymlFile[map[string]mcpServer]) ([]MCP, error) {
		if file == nil {
			return nil, nil
		}

		result := make([]MCP, 0, len(file.Data))
		for name, v := range file.Data {
			var tp MCPToolPolicy
			if len(v.UseTools) == 0 {
				tp = AllMCPTools()
			} else {
				tp = UseMCPTools(v.UseTools...)
			}

			switch v.Type {
			case "local":
				if len(v.Command) == 0 {
					return nil, fmt.Errorf(`%w: mcp type "local" requires command`, ErrInvalid)
				}
				result = append(result, LocalMCP(name, v.Command, v.Env, tp))

			case "remote":
				url := strings.TrimSpace(v.URL)
				if url == "" {
					return nil, fmt.Errorf(`%w: mcp type "remote" requires url`, ErrInvalid)
				}
				result = append(result, RemoteMCP(name, url, v.Headers, tp))

			default:
				return nil, fmt.Errorf("%w: unknown mcp type %q", ErrNotSupported, v.Type)
			}
		}
		slices.SortFunc(result, func(a, b MCP) int {
			return strings.Compare(a.Name(), b.Name())
		})
		return result, nil
	})
}
