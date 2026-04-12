package driven

import "time"

// Clock is a driven port over wall time and sleep. Introduced to make
// the AI dialog state machine's backoff logic testable without actual
// sleeps; production wires a trivial adapter over the time package.
//
// Other services may adopt this port over time to make history
// timestamps deterministic under test; until then they continue to
// use time.Now directly.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}
