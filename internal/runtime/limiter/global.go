package limiter

import "sync"

var (
	globalOnce  sync.Once
	globalLimit *ModelLimiter
)

// DefaultLimiter returns the singleton ModelLimiter instance.
func DefaultLimiter() *ModelLimiter {
	globalOnce.Do(func() {
		globalLimit = NewModelLimiter()
	})
	return globalLimit
}
