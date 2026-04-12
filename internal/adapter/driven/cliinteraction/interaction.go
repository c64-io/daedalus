// Package cliinteraction is the driven adapter for driven.Interaction
// backed by stdin/stdout and $EDITOR. It is the piece of d7 the
// founder actually talks to during AI-assisted commands.
//
// Lifecycle:
//
//   - Ask: print the model's question plus a (Q n/max) hint, read
//     one line of input. Sentinel replies `/done` → AnswerForcePropose
//     and `/quit` → AnswerAbort; everything else is AnswerReply.
//   - Review: print the proposal body, then one-character verdict
//     loop: a(ccept) / c(ritique) / e(dit) / q(uit). `e` opens
//     $EDITOR on the proposal's RawJSON and routes the edited
//     content through DecisionAccept.Edited so the service can
//     re-validate before applying.
//
// Multi-item review (DecisionRetry / PerItem) is not implemented here
// — d7's first AI command is single-item expand-story, and a stdin
// picker will land with the first multi-item command (suggest).
package cliinteraction

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// Interactor is the stdin/stdout implementation. It carries a driven
// Editor so the "edit" review path can reuse the same $EDITOR helper
// every other d7 command uses.
type Interactor struct {
	in     *bufio.Reader
	out    io.Writer
	editor driven.Editor
}

// Compile-time assertion.
var _ driven.Interaction = (*Interactor)(nil)

// New returns an Interactor reading from stdin, writing to stdout,
// and using editor for the "edit proposal" flow.
func New(editor driven.Editor) *Interactor {
	return NewWithIO(os.Stdin, os.Stdout, editor)
}

// NewWithIO is the test constructor — pass any Reader/Writer pair.
func NewWithIO(in io.Reader, out io.Writer, editor driven.Editor) *Interactor {
	return &Interactor{
		in:     bufio.NewReader(in),
		out:    out,
		editor: editor,
	}
}

// Ask relays a question to the founder and reads one line back.
// `/done` and `/quit` are sentinels that translate into
// AnswerForcePropose / AnswerAbort respectively.
func (i *Interactor) Ask(ctx context.Context, q driven.Question) (driven.Answer, error) {
	// A short banner so the model's question visually separates from
	// whatever the status renderer was printing above it.
	fmt.Fprintln(i.out)
	if q.TurnsMax > 0 {
		fmt.Fprintf(i.out, "(Q %d/%d) %s\n", q.TurnsUsed, q.TurnsMax, q.Text)
	} else {
		fmt.Fprintf(i.out, "Q: %s\n", q.Text)
	}
	if q.Why != "" {
		fmt.Fprintf(i.out, "    why: %s\n", q.Why)
	}
	fmt.Fprintf(i.out, "    (type `/done` to have the AI propose now, `/quit` to cancel)\n")
	fmt.Fprint(i.out, "> ")

	line, err := i.readLine(ctx)
	if err != nil {
		return driven.Answer{}, err
	}
	trimmed := strings.TrimSpace(line)

	switch trimmed {
	case "/done":
		return driven.Answer{Kind: driven.AnswerForcePropose}, nil
	case "/quit", "/abort", "/cancel":
		return driven.Answer{Kind: driven.AnswerAbort}, nil
	}
	return driven.Answer{Kind: driven.AnswerReply, Text: trimmed}, nil
}

// Review shows a finished proposal and prompts the founder for a
// verdict. Edit routes through DecisionAccept.Edited so the service
// can re-validate the JSON before applying.
func (i *Interactor) Review(ctx context.Context, p driven.Proposal) (driven.Decision, error) {
	if p.Format != driven.ProposalSingle || p.Single == nil {
		return driven.Decision{}, fmt.Errorf("cliinteraction: only single-item proposals are supported in v1")
	}

	fmt.Fprintln(i.out)
	if p.Single.Title != "" {
		fmt.Fprintf(i.out, "── %s ──\n", p.Single.Title)
	}
	fmt.Fprintln(i.out, p.Single.Body)
	fmt.Fprintln(i.out)
	fmt.Fprintln(i.out, "Accept this proposal? [a]ccept / [c]ritique / [e]dit / [q]uit")

	for {
		fmt.Fprint(i.out, "> ")
		line, err := i.readLine(ctx)
		if err != nil {
			return driven.Decision{}, err
		}
		choice := strings.ToLower(strings.TrimSpace(line))
		switch choice {
		case "a", "accept", "y", "yes":
			return driven.Decision{Kind: driven.DecisionAccept}, nil

		case "c", "critique":
			critique, err := i.readCritique(ctx)
			if err != nil {
				return driven.Decision{}, err
			}
			if strings.TrimSpace(critique) == "" {
				fmt.Fprintln(i.out, "(empty critique; choose again)")
				continue
			}
			return driven.Decision{Kind: driven.DecisionCritique, Critique: critique}, nil

		case "e", "edit":
			if p.Single.JSON == "" && p.RawJSON == "" {
				fmt.Fprintln(i.out, "(this proposal has no editable JSON; choose another option)")
				continue
			}
			buffer := p.Single.JSON
			if buffer == "" {
				buffer = p.RawJSON
			}
			edited, err := i.editor.Edit(buffer)
			if err != nil {
				return driven.Decision{}, fmt.Errorf("open editor: %w", err)
			}
			return driven.Decision{Kind: driven.DecisionAccept, Edited: edited}, nil

		case "q", "quit", "abort", "cancel":
			return driven.Decision{Kind: driven.DecisionAbort}, nil

		case "":
			// Empty input → re-prompt.
			continue

		default:
			fmt.Fprintf(i.out, "unrecognized choice %q — type a, c, e, or q.\n", choice)
		}
	}
}

// readCritique gathers multi-line feedback. The founder ends the
// critique by entering a line that is just "." on its own.
func (i *Interactor) readCritique(ctx context.Context) (string, error) {
	fmt.Fprintln(i.out, "Your feedback (end with a single `.` on its own line):")
	var b strings.Builder
	for {
		line, err := i.readLine(ctx)
		if err != nil {
			return "", err
		}
		if strings.TrimRight(line, "\r\n") == "." {
			return strings.TrimRight(b.String(), "\n"), nil
		}
		b.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			b.WriteByte('\n')
		}
	}
}

// readLine reads one line from stdin, respecting context cancellation
// to the extent the platform allows (cancellation won't unblock a
// blocking read in Go, but honoring ctx before reading still matters
// when an earlier transition already cancelled the context).
func (i *Interactor) readLine(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	line, err := i.in.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	if err == io.EOF && line == "" {
		return "", io.EOF
	}
	return line, nil
}
