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
// Multi-item review is handled via Proposal.Multiple: each item is
// printed with its 1-based index, and the founder picks with one of
// `a` (accept all), `n` (accept none), a comma-separated subset like
// `1,3,5`, `c` (critique whole batch), `e` (edit whole list in
// $EDITOR), or `q` (quit).
package cliinteraction

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// Interactor is the stdin/stdout implementation. It carries a driven
// Editor so the "edit" review path can reuse the same $EDITOR helper
// every other d7 command uses. The styler colorizes headers and
// labels when out is an interactive terminal.
type Interactor struct {
	in     *bufio.Reader
	out    io.Writer
	editor driven.Editor
	s      styler
}

// Compile-time assertion.
var _ driven.Interaction = (*Interactor)(nil)

// New returns an Interactor reading from stdin, writing to stdout,
// and using editor for the "edit proposal" flow.
func New(editor driven.Editor) *Interactor {
	return NewWithIO(os.Stdin, os.Stdout, editor)
}

// NewWithIO is the test constructor — pass any Reader/Writer pair.
// Color is auto-disabled when out is not a character device.
func NewWithIO(in io.Reader, out io.Writer, editor driven.Editor) *Interactor {
	return &Interactor{
		in:     bufio.NewReader(in),
		out:    out,
		editor: editor,
		s:      newStyler(out),
	}
}

// Ask relays a question to the founder and reads one line back.
// `/done` and `/quit` are sentinels that translate into
// AnswerForcePropose / AnswerAbort respectively.
func (i *Interactor) Ask(ctx context.Context, q driven.Question) (driven.Answer, error) {
	// Two blank lines plus a colored header give each question
	// enough vertical breathing room that sequential Q&A doesn't
	// visually blur. The header is bold+cyan so a quick glance
	// down the scrollback immediately spots each new turn.
	fmt.Fprintln(i.out)
	fmt.Fprintln(i.out)
	var badge string
	if q.TurnsMax > 0 {
		badge = fmt.Sprintf("(Q %d/%d)", q.TurnsUsed, q.TurnsMax)
	} else {
		badge = "Q:"
	}
	fmt.Fprintf(i.out, "%s %s\n", i.s.boldCyan(badge), i.s.bold(q.Text))
	if q.Why != "" {
		fmt.Fprintf(i.out, "    %s\n", i.s.dim("why: "+q.Why))
	}
	fmt.Fprintf(i.out, "    %s\n", i.s.dim("(type `/done` to have the AI propose now, `/quit` to cancel)"))
	fmt.Fprintf(i.out, "%s ", i.s.boldGreen(">"))

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
	switch p.Format {
	case driven.ProposalSingle:
		if p.Single == nil {
			return driven.Decision{}, fmt.Errorf("cliinteraction: single proposal has no body")
		}
		return i.reviewSingle(ctx, p)
	case driven.ProposalMultiple:
		return i.reviewMultiple(ctx, p)
	default:
		return driven.Decision{}, fmt.Errorf("cliinteraction: unsupported proposal format %d", p.Format)
	}
}

// reviewSingle handles the expand / refine flow: one proposal, yes/no/
// critique/edit.
func (i *Interactor) reviewSingle(ctx context.Context, p driven.Proposal) (driven.Decision, error) {
	fmt.Fprintln(i.out)
	fmt.Fprintln(i.out)
	if p.Single.Title != "" {
		fmt.Fprintln(i.out, i.s.boldMagenta(fmt.Sprintf("── %s ──", p.Single.Title)))
	}
	fmt.Fprintln(i.out, p.Single.Body)
	fmt.Fprintln(i.out)
	fmt.Fprintf(i.out, "%s %s\n",
		i.s.bold("Accept this proposal?"),
		i.s.dim("[")+i.s.yellow("a")+i.s.dim("]ccept / [")+i.s.yellow("c")+i.s.dim("]ritique / [")+i.s.yellow("e")+i.s.dim("]dit / [")+i.s.yellow("q")+i.s.dim("]uit"),
	)

	for {
		fmt.Fprintf(i.out, "%s ", i.s.boldGreen(">"))
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

// reviewMultiple handles the suggest flow: N proposed children, each
// previewed, picker at the bottom. See parseMultiSelect for the input
// grammar.
func (i *Interactor) reviewMultiple(ctx context.Context, p driven.Proposal) (driven.Decision, error) {
	n := len(p.Multiple)
	if n == 0 {
		// An empty batch is semantically nothing to accept. Treat as
		// abort so the caller prints "nothing was saved".
		return driven.Decision{Kind: driven.DecisionAbort}, nil
	}

	fmt.Fprintln(i.out)
	fmt.Fprintln(i.out)
	fmt.Fprintln(i.out, i.s.bold(fmt.Sprintf("The AI has proposed %d item%s:", n, plural(n))))
	for idx, it := range p.Multiple {
		fmt.Fprintln(i.out)
		title := it.Title
		if title == "" {
			title = fmt.Sprintf("Item %d", idx+1)
		}
		rule := fmt.Sprintf("── [%d] %s ──", idx+1, title)
		fmt.Fprintln(i.out, i.s.boldMagenta(rule))
		if it.Body != "" {
			fmt.Fprintln(i.out, it.Body)
		}
	}
	fmt.Fprintln(i.out)
	fmt.Fprintln(i.out, i.s.bold("Which items would you like to accept?"))
	fmt.Fprintf(i.out, "  %s  %s  %s  %s  %s  %s\n",
		i.s.dim("[")+i.s.yellow("a")+i.s.dim("]ll"),
		i.s.dim("[")+i.s.yellow("n")+i.s.dim("]one"),
		i.s.dim("[")+i.s.yellow("1,3,5")+i.s.dim("] subset"),
		i.s.dim("[")+i.s.yellow("c")+i.s.dim("]ritique"),
		i.s.dim("[")+i.s.yellow("e")+i.s.dim("]dit list"),
		i.s.dim("[")+i.s.yellow("q")+i.s.dim("]uit"),
	)

	for {
		fmt.Fprintf(i.out, "%s ", i.s.boldGreen(">"))
		line, err := i.readLine(ctx)
		if err != nil {
			return driven.Decision{}, err
		}
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch lower {
		case "":
			continue

		case "a", "all", "accept":
			return decisionAcceptAll(n), nil

		case "n", "none":
			// None accepted is effectively abort at the ai_loop layer,
			// but we still go through DecisionAccept + PerItem so the
			// "nothing was saved" path in the service is exercised.
			return decisionAcceptNone(n), nil

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
			if p.RawJSON == "" {
				fmt.Fprintln(i.out, "(this proposal has no editable JSON; choose another option)")
				continue
			}
			edited, err := i.editor.Edit(p.RawJSON)
			if err != nil {
				return driven.Decision{}, fmt.Errorf("open editor: %w", err)
			}
			return driven.Decision{Kind: driven.DecisionEditList, Edited: edited}, nil

		case "q", "quit", "abort", "cancel":
			return driven.Decision{Kind: driven.DecisionAbort}, nil

		default:
			picks, perr := parseMultiSelect(trimmed, n)
			if perr != nil {
				fmt.Fprintf(i.out, "(%s)\n", perr.Error())
				continue
			}
			return decisionAcceptSubset(n, picks), nil
		}
	}
}

// plural returns "" for 1 and "s" otherwise.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// parseMultiSelect converts an input like "1,3,5" into a de-duplicated
// slice of 0-based indices bounded by n. Empty entries are ignored;
// out-of-range or non-numeric entries produce a descriptive error that
// the caller shows to the founder before re-prompting.
func parseMultiSelect(s string, n int) ([]int, error) {
	parts := strings.Split(s, ",")
	seen := map[int]struct{}{}
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		v, err := strconv.Atoi(t)
		if err != nil {
			return nil, fmt.Errorf("not a number: %q", t)
		}
		if v < 1 || v > n {
			return nil, fmt.Errorf("out of range: %d (valid: 1..%d)", v, n)
		}
		idx := v - 1
		if _, dup := seen[idx]; dup {
			continue
		}
		seen[idx] = struct{}{}
		out = append(out, idx)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no items selected; type 'a', 'n', or a comma list like 1,3")
	}
	return out, nil
}

// decisionAcceptAll builds a PerItem slice of all-Yes for the whole
// batch of n items.
func decisionAcceptAll(n int) driven.Decision {
	per := make([]driven.ItemDecision, n)
	for i := range per {
		per[i] = driven.ItemDecision{Kind: driven.ItemYes}
	}
	return driven.Decision{Kind: driven.DecisionAccept, PerItem: per}
}

// decisionAcceptNone builds a PerItem slice of all-No; the ai_loop
// routes this to stateAborted.
func decisionAcceptNone(n int) driven.Decision {
	per := make([]driven.ItemDecision, n)
	for i := range per {
		per[i] = driven.ItemDecision{Kind: driven.ItemNo}
	}
	return driven.Decision{Kind: driven.DecisionAccept, PerItem: per}
}

// decisionAcceptSubset marks the listed 0-based indices Yes and the
// rest No.
func decisionAcceptSubset(n int, picks []int) driven.Decision {
	yes := map[int]struct{}{}
	for _, p := range picks {
		yes[p] = struct{}{}
	}
	per := make([]driven.ItemDecision, n)
	for i := range per {
		if _, ok := yes[i]; ok {
			per[i] = driven.ItemDecision{Kind: driven.ItemYes}
		} else {
			per[i] = driven.ItemDecision{Kind: driven.ItemNo}
		}
	}
	return driven.Decision{Kind: driven.DecisionAccept, PerItem: per}
}

// readCritique gathers multi-line feedback. The founder ends the
// critique by entering a line that is just "." on its own.
func (i *Interactor) readCritique(ctx context.Context) (string, error) {
	fmt.Fprintf(i.out, "%s %s\n",
		i.s.bold("Your feedback"),
		i.s.dim("(end with a single `.` on its own line):"))
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
