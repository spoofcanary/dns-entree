package migrate

import (
	"context"

	"golang.org/x/time/rate"
)

// WriteLimiter throttles outbound writes to a target provider API (D-26,
// T-05b-08). Defaults to 10/sec with a burst of 1 when created via
// NewWriteLimiter with a non-positive value.
type WriteLimiter struct {
	lim *rate.Limiter
}

// NewWriteLimiter returns a token-bucket limiter. Zero or negative values
// yield the default 10/sec.
func NewWriteLimiter(perSecond float64) *WriteLimiter {
	if perSecond <= 0 {
		perSecond = 10
	}
	return &WriteLimiter{lim: rate.NewLimiter(rate.Limit(perSecond), 1)}
}

// Wait blocks until the limiter permits one more event or ctx is done.
func (w *WriteLimiter) Wait(ctx context.Context) error {
	if w == nil || w.lim == nil {
		return nil
	}
	return w.lim.Wait(ctx)
}
