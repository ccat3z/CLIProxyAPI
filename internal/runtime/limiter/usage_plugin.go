package limiter

import (
	"context"

	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

// limiterPlugin is a usage.Plugin that records token consumption in the
// global ModelLimiter. It only records for authID+model keys that have
// limits configured, so keys without limits incur zero overhead.
type limiterPlugin struct{}

func (p *limiterPlugin) HandleUsage(ctx context.Context, record usage.Record) {
	if record.Failed {
		return
	}
	l := DefaultLimiter()
	if !l.HasLimits(record.AuthID, record.Model) {
		return
	}
	l.Record(record.AuthID, record.Model, record.RequestedAt,
		record.Detail.InputTokens, record.Detail.OutputTokens)
}

func init() {
	usage.RegisterPlugin(&limiterPlugin{})
}
