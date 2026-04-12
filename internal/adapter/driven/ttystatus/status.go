// Package ttystatus is the driven adapter for driven.StatusRenderer.
// It prints short phase updates to a writer (typically stderr) so the
// founder can see what an AI-assisted command is doing between
// interactive prompts.
//
// v1 keeps this deliberately unfancy: one status line per phase
// transition, rewritten in place on TTYs with a carriage return so
// the scrollback stays clean, and appended as plain lines when
// stdout is piped. No ANSI scroll regions, no spinners.
package ttystatus

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// Renderer prints status lines to w. When isTTY is true each Update
// rewrites the previous line via \r + ANSI clear-to-end-of-line.
// When false, each Update is appended on its own line.
type Renderer struct {
	w     io.Writer
	isTTY bool

	mu      sync.Mutex
	started bool
	stopped bool
	lastLen int
}

// Compile-time assertion.
var _ driven.StatusRenderer = (*Renderer)(nil)

// New returns a Renderer writing to stderr, detecting TTY via the
// stderr file descriptor.
func New() *Renderer {
	return NewWithWriter(os.Stderr, isTerminal(os.Stderr))
}

// NewWithWriter returns a Renderer writing to w. isTTY controls the
// in-place overwrite behavior; set it to false in tests to get a
// plain-line log.
func NewWithWriter(w io.Writer, isTTY bool) *Renderer {
	return &Renderer{w: w, isTTY: isTTY}
}

// Start prints the first status line.
func (r *Renderer) Start(_ context.Context, s driven.Status) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return
	}
	r.started = true
	r.writeLine(r.format(s))
}

// Update replaces the last status line.
func (r *Renderer) Update(s driven.Status) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.started || r.stopped {
		return
	}
	r.writeLine(r.format(s))
}

// Stop ends the status line with a newline so subsequent output
// starts on a fresh row.
func (r *Renderer) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.started || r.stopped {
		return
	}
	r.stopped = true
	if r.isTTY {
		// Clear whatever the last status line held so it doesn't
		// linger above the next CLI output.
		fmt.Fprint(r.w, "\r\x1b[2K")
	}
}

// format renders a Status into a one-line header.
func (r *Renderer) format(s driven.Status) string {
	parts := []string{"d7"}
	if s.Command != "" {
		parts = append(parts, s.Command)
	}
	if s.Target != "" {
		parts = append(parts, s.Target)
	}

	phase := s.Phase.String()
	line := fmt.Sprintf("%s · %s", joinWith(parts, " "), phase)

	if s.Turn > 0 {
		line += fmt.Sprintf(" · turn %d", s.Turn)
	}
	if !s.StartedAt.IsZero() {
		elapsed := time.Since(s.StartedAt).Round(time.Second)
		line += fmt.Sprintf(" · %s", elapsed)
	}
	if s.Extra != "" {
		line += fmt.Sprintf(" · %s", s.Extra)
	}
	return line
}

func (r *Renderer) writeLine(line string) {
	if r.isTTY {
		// \r to return to column 0, \x1b[2K to clear the line, then
		// write the new content. No trailing newline so the next
		// Update overwrites in place.
		fmt.Fprintf(r.w, "\r\x1b[2K%s", line)
		r.lastLen = len(line)
		return
	}
	fmt.Fprintln(r.w, line)
}

// joinWith is a tiny strings.Join wrapper that skips empty segments.
func joinWith(parts []string, sep string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out == "" {
			out = p
		} else {
			out += sep + p
		}
	}
	return out
}

// isTerminal reports whether f is attached to an interactive
// terminal. A Stat() that returns a character-device mode is the
// portable POSIX signal; on Windows this would need a different
// check, but d7 is Linux/macOS-only in v1.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
