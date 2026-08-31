package nestor

import (
	"context"
)

type Context interface {
	context.Context

	Registry() Registry

	Platform() Platform
}

func newContext(ctx context.Context, platform Platform, registry Registry) Context {
	return &contextImpl{
		Context:  ctx,
		platform: platform,
		registry: registry,
	}
}

type contextImpl struct {
	context.Context

	platform Platform
	registry Registry
}

func (c *contextImpl) Platform() Platform {
	return c.platform
}

func (c *contextImpl) Registry() Registry {
	return c.registry
}
