// Package clockexec is the production adapter for the driven.Clock
// port. It wraps the standard library's time package so services that
// depend on Clock (the AI dialog state machine's backoff, for one)
// get real wall time in main.go while tests inject a fake.
package clockexec

import (
	"time"

	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// Compile-time assertion that Clock satisfies the port.
var _ driven.Clock = (*Clock)(nil)

// Clock is a zero-size adapter over the time package.
type Clock struct{}

// New returns a Clock.
func New() *Clock { return &Clock{} }

// Now returns the current wall-clock time.
func (Clock) Now() time.Time { return time.Now() }

// Sleep blocks for d. Negative or zero d returns immediately.
func (Clock) Sleep(d time.Duration) {
	if d <= 0 {
		return
	}
	time.Sleep(d)
}
