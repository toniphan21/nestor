package nestor

import (
	"context"
	"errors"
	"time"
)

type Tenancy interface {
	leaseAPI

	Err() error
}

func Tenancies(ctx context.Context, api API, spec, path string) <-chan Tenancy {
	stream := make(chan Tenancy)

	go func() {
		defer close(stream)

		log := api.Runtime().Logger.With("layer", "tenancy")

		for {
			lease, err := api.Acquire(ctx, spec, path)

			switch {
			case errors.Is(err, ErrNotAvailable):
				log.Debug("lease is not available")

			case err != nil:
				if ctx.Err() != nil {
					return // cancelled, not a real failure
				}

				select {
				case stream <- &failedTenancy{err}:
				case <-ctx.Done():
				}
				return

			default:
				select {
				case stream <- &leaseTenancy{lease}:
					continue // capacity may remain, acquire again

				case <-ctx.Done():
					if re := lease.Release(ctx); re != nil { // never orphan lease, release it
						log.Warn("cannot release lease", "lease", lease.ID())
					}
					return
				}
			}
		}
	}()

	return stream
}

type failedTenancy struct {
	err error
}

func (t *failedTenancy) ID() string { return "" }

func (t *failedTenancy) Sandbox() Sandbox { return nil }

func (t *failedTenancy) HostPath() string { return "" }

func (t *failedTenancy) WorkDir() string { return "" }

func (t *failedTenancy) ProxyAddr() string { return "" }

func (t *failedTenancy) ExpiresAt() time.Time { return time.Time{} }

func (t *failedTenancy) Extend(ctx context.Context) error { return t.err }

func (t *failedTenancy) Release(ctx context.Context) error { return t.err }

func (t *failedTenancy) Run(ctx context.Context, prompt string, opt RunOption) (RunResult, error) {
	return RunResult{ExitCode: -1}, t.err
}

func (t *failedTenancy) Err() error {
	return t.err
}

var _ Tenancy = (*failedTenancy)(nil)

type leaseTenancy struct {
	lease *Lease
}

func (t *leaseTenancy) ID() string { return t.lease.ID() }

func (t *leaseTenancy) Sandbox() Sandbox { return t.lease.Sandbox() }

func (t *leaseTenancy) HostPath() string { return t.lease.HostPath() }

func (t *leaseTenancy) WorkDir() string { return t.lease.WorkDir() }

func (t *leaseTenancy) ProxyAddr() string { return t.lease.ProxyAddr() }

func (t *leaseTenancy) ExpiresAt() time.Time { return t.lease.ExpiresAt() }

func (t *leaseTenancy) Extend(ctx context.Context) error { return t.lease.Extend(ctx) }

func (t *leaseTenancy) Release(ctx context.Context) error { return t.lease.Release(ctx) }

func (t *leaseTenancy) Run(ctx context.Context, prompt string, opt RunOption) (RunResult, error) {
	return t.lease.Run(ctx, prompt, opt)
}

func (t *leaseTenancy) Err() error {
	return nil
}

var _ Tenancy = (*leaseTenancy)(nil)
