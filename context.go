package nestor

import (
	"context"
)

type Context interface {
	context.Context

	Registry() Registry

	Platform() Platform

	Template() Template
}

func newContext(ctx context.Context, platform Platform, registry Registry, template Template) Context {
	return &contextImpl{
		Context:  ctx,
		platform: platform,
		registry: registry,
		template: template,
	}
}

type contextImpl struct {
	context.Context

	platform Platform
	registry Registry
	template Template
}

func (c *contextImpl) Template() Template {
	return c.template
}

func (c *contextImpl) Platform() Platform {
	return c.platform
}

func (c *contextImpl) Registry() Registry {
	return c.registry
}
