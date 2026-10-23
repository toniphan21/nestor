package nestor

import "log/slog"

type Runtime struct {
	Registry Registry
	Platform Platform
	Template Template
	Logger   *slog.Logger

	newGitFunc    NewGitFunc
	newDockerFunc NewDockerFunc

	git    Git
	docker Docker
}

func (r *Runtime) Git() Git {
	if r.git == nil {
		r.git = r.newGitFunc(r.Logger)
	}
	return r.git
}

func (r *Runtime) Docker() Docker {
	if r.docker == nil {
		r.docker = r.newDockerFunc(r.Logger)
	}
	return r.docker
}

func (r *Runtime) CompatibleHarnesses(profile Profile) []Harness {
	var out []Harness
	for _, v := range r.Registry.Harnesses() {
		rp, err := v.ResolveProfile(*r, profile)
		if rp != nil && err == nil {
			out = append(out, v)
		}
	}
	return out
}
