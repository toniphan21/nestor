package nestor

func newRegistry() Registry {
	return &registry{
		sandboxSpecs: make(map[string]SandboxSpec),
		profiles:     make(map[string]Profile),
		harnesses: map[string]Harness{
			string(HarnessClaudeCode): newHarnessClaude(),
		},
	}
}

type Registry interface {
	RegisterSandboxSpec(spec SandboxSpec) Registry

	RegisterProfile(profile Profile) Registry

	RegisterHarness(harness Harness) Registry

	SandboxSpec(name string) (SandboxSpec, bool)

	SandboxSpecs() []SandboxSpec

	Profile(name string) (Profile, bool)

	Profiles() []Profile

	Harness(name string) (Harness, bool)

	Harnesses() []Harness
}

type registry struct {
	sandboxSpecs map[string]SandboxSpec
	harnesses    map[string]Harness
	profiles     map[string]Profile
}

func (r *registry) RegisterSandboxSpec(spec SandboxSpec) Registry {
	r.sandboxSpecs[spec.Name] = spec
	return r
}

func (r *registry) RegisterProfile(spec Profile) Registry {
	r.profiles[spec.Name] = spec
	return r
}

func (r *registry) RegisterHarness(harness Harness) Registry {
	r.harnesses[harness.Name()] = harness
	return r
}

func (r *registry) SandboxSpec(name string) (SandboxSpec, bool) {
	s, ok := r.sandboxSpecs[name]
	return s, ok
}

func (r *registry) SandboxSpecs() []SandboxSpec {
	out := make([]SandboxSpec, 0, len(r.sandboxSpecs))
	for _, v := range r.sandboxSpecs {
		out = append(out, v)
	}
	return out
}

func (r *registry) Profile(name string) (Profile, bool) {
	s, ok := r.profiles[name]
	return s, ok
}

func (r *registry) Profiles() []Profile {
	out := make([]Profile, 0, len(r.profiles))
	for _, v := range r.profiles {
		out = append(out, v)
	}
	return out
}

func (r *registry) Harness(name string) (Harness, bool) {
	h, ok := r.harnesses[name]
	return h, ok
}

func (r *registry) Harnesses() []Harness {
	out := make([]Harness, 0, len(r.harnesses))
	for _, v := range r.harnesses {
		out = append(out, v)
	}
	return out
}

var _ Registry = (*registry)(nil)
