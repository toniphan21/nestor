package nestor

import "log/slog"

type Runtime struct {
	Registry Registry
	Platform Platform
	Template Template
	Logger   *slog.Logger
}
