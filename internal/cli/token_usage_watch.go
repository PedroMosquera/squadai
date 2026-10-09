package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

// writerIsTTY reports whether w is a terminal. Only an *os.File can be one.
func writerIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// watchKey is what counts as "changed" between ticks. TotalTokens excludes
// cache tokens, so cache reads and writes are tracked separately.
type watchKey struct {
	total, cacheRead, cacheWrite int
}

func watchKeyOf(agg *session.Aggregation) watchKey {
	return watchKey{agg.Total.TotalTokens, agg.Total.CacheReadTokens, agg.Total.CacheCreationTokens}
}

// runTokenUsageWatch renders aggregate() immediately, then re-aggregates every
// interval and re-renders only when usage changed, until ctx is done (returns
// nil). On a TTY each render clears the screen and prints the full table plus
// a footer; otherwise it prints one compact line per change with no escape
// codes. A failed aggregation prints a warning and the loop keeps going.
func runTokenUsageWatch(ctx context.Context, stdout io.Writer, aggregate func() (*session.Aggregation, error), interval time.Duration, isTTY bool) error {
	startTokens := -1
	var last *watchKey

	tick := func() {
		agg, err := aggregate()
		if err != nil {
			fmt.Fprintf(stdout, "warning: aggregate failed: %v\n", err)
			return
		}
		key := watchKeyOf(agg)
		if last != nil && *last == key {
			return
		}
		last = &key
		if startTokens < 0 {
			startTokens = key.total
		}
		delta := key.total - startTokens
		now := time.Now().Format("15:04:05")
		if isTTY {
			fmt.Fprint(stdout, "\x1b[2J\x1b[H")
			printTokenUsageHuman(stdout, agg)
			fmt.Fprintln(stdout)
			fmt.Fprintf(stdout, "Last updated: %s  Ctrl+C to stop\n", now)
			fmt.Fprintf(stdout, "Delta since watch start: %+d tokens\n", delta)
			return
		}
		fmt.Fprintf(stdout, "%s  total=%d cache_read=%d cache_write=%d delta=%+d\n",
			now, key.total, key.cacheRead, key.cacheWrite, delta)
	}

	tick()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			tick()
		}
	}
}
