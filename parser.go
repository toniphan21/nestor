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

func ParseMCPs(yml io.Reader) ([]MCP, error) {
	return nil, nil
}
