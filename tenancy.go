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

func Tenancies(ctx context.Context, api API, spec, path string, interval time.Duration) <-chan Tenancy {
	stream := make(chan Tenancy)
	if interval <= 0 {
		interval = 1 * time.Second
	}

	go func() {
		defer close(stream)

		log := api.Runtime().Logger.With("layer", "tenancy")

		for {
			lease, err := api.Acquire(ctx, spec, path)

			switch {
			case errors.Is(err, ErrNotAvailable):
				log.Debug("lease is not available")
				select {
				case <-ctx.Done():
					return
				case <-time.After(interval):
					// no lease available — back off before retrying, to avoid hammering the nestor files
				}

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

func (t *failedTenancy) RequestedPath() string { return "" }

func (t *failedTenancy) WorkDir() string { return "" }

func (t *failedTenancy) HostWorkDir() string { return "" }

func (t *failedTenancy) ProxyAddr() string { return "" }

func (t *failedTenancy) ExpiresAt() time.Time { return time.Time{} }

func (t *failedTenancy) Extend(ctx context.Context) error { return t.err }

func (t *failedTenancy) Release(ctx context.Context) error { return t.err }

func (t *failedTenancy) Run(ctx context.Context, param RunParam) (RunResult, error) {
	return RunResult{ExitCode: -1}, t.err
}

func (t *failedTenancy) SessionID() string { return "" }

func (t *failedTenancy) ListSessions() []HarnessSession { return nil }

func (t *failedTenancy) Err() error {
	return t.err
}

var _ Tenancy = (*failedTenancy)(nil)

type leaseTenancy struct {
	lease *Lease
}

func (t *leaseTenancy) ID() string { return t.lease.ID() }

func (t *leaseTenancy) Sandbox() Sandbox { return t.lease.Sandbox() }

func (t *leaseTenancy) RequestedPath() string { return t.lease.RequestedPath() }

func (t *leaseTenancy) WorkDir() string { return t.lease.WorkDir() }

func (t *leaseTenancy) HostWorkDir() string { return t.lease.HostWorkDir() }

func (t *leaseTenancy) ProxyAddr() string { return t.lease.ProxyAddr() }

func (t *leaseTenancy) ExpiresAt() time.Time { return t.lease.ExpiresAt() }

func (t *leaseTenancy) Extend(ctx context.Context) error { return t.lease.Extend(ctx) }

func (t *leaseTenancy) Release(ctx context.Context) error { return t.lease.Release(ctx) }

func (t *leaseTenancy) Run(ctx context.Context, param RunParam) (RunResult, error) {
	return t.lease.Run(ctx, param)
}

func (t *leaseTenancy) SessionID() string { return t.lease.SessionID() }

func (t *leaseTenancy) ListSessions() []HarnessSession { return t.lease.ListSessions() }

func (t *leaseTenancy) Err() error {
	return nil
}

var _ Tenancy = (*leaseTenancy)(nil)
