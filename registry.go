package nestor

func newRegistry() Registry {
	return &registry{
		sandboxSpecs:   make(map[string]SandboxSpec),
		harnessesSpecs: make(map[string]HarnessSpec),
		harnesses: map[string]Harness{
			string(HarnessClaudeCode): newHarnessClaude(),
		},
	}
}

type Registry interface {
	RegisterSandboxSpec(spec SandboxSpec) Registry

	RegisterHardnessSpec(spec HarnessSpec) Registry

	RegisterHarness(harness Harness) Registry

	SandboxSpec(name string) (SandboxSpec, bool)

	SandboxSpecs() []SandboxSpec

	HarnessSpec(name string) (HarnessSpec, bool)

	HarnessSpecs() []HarnessSpec

	Harness(name string) (Harness, bool)

	Harnesses() []Harness
}

type registry struct {
	harnesses      map[string]Harness
	sandboxSpecs   map[string]SandboxSpec
	harnessesSpecs map[string]HarnessSpec
}

func (r *registry) RegisterSandboxSpec(spec SandboxSpec) Registry {
	r.sandboxSpecs[spec.Name] = spec
	return r
}

func (r *registry) RegisterHardnessSpec(spec HarnessSpec) Registry {
	r.harnessesSpecs[spec.Name] = spec
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

func (r *registry) HarnessSpec(name string) (HarnessSpec, bool) {
	s, ok := r.harnessesSpecs[name]
	return s, ok
}

func (r *registry) HarnessSpecs() []HarnessSpec {
	out := make([]HarnessSpec, 0, len(r.harnessesSpecs))
	for _, v := range r.harnessesSpecs {
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
