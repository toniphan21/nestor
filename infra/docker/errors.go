package docker

import (
	"errors"
	"strings"
)

var (
	ErrDockerNotInstalled = errors.New("docker CLI not found in PATH")
	ErrDaemonUnreachable  = errors.New("docker daemon unreachable")
	ErrDaemonPermission   = errors.New("no permission to access docker socket")
	ErrContainerNotFound  = errors.New("container not found")
)

func classifyError(stderr string) error {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "permission denied") &&
		(strings.Contains(s, "docker api") || strings.Contains(s, "docker daemon")):
		return ErrDaemonPermission

	// newer CLI (28+): "failed to connect to the docker API at unix://..."
	// older CLI:       "Cannot connect to the Docker daemon at unix://..."
	case strings.Contains(s, "failed to connect to the docker api"),
		strings.Contains(s, "cannot connect to the docker daemon"):
		return ErrDaemonUnreachable

	case strings.Contains(s, "no such container"):
		return ErrContainerNotFound

	default:
		return errors.New(stderr)
	}
}
