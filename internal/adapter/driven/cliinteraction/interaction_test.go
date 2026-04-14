package cliinteraction

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// ---------------------------------------------------------------------
// multi-select parser (pure function)
// ---------------------------------------------------------------------

func TestParseMultiSelect_HappyPaths(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want []int
	}{
		{"1", 3, []int{0}},
		{"1,2,3", 3, []int{0, 1, 2}},
		{"3,1", 3, []int{2, 0}},         // preserves first-seen order
		{"1, 3 , 5", 5, []int{0, 2, 4}}, // whitespace tolerant
		{"2,2,2", 3, []int{1}},          // de-duplicates
		{"1,,2", 3, []int{0, 1}},        // empty entries ignored
	}
	for _, c := range cases {
		got, err := parseMultiSelect(c.in, c.n)
		if err != nil {
			t.Errorf("parseMultiSelect(%q, %d): unexpected err %v", c.in, c.n, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("parseMultiSelect(%q): len got %d, want %d", c.in, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("parseMultiSelect(%q)[%d]: got %d, want %d", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestParseMultiSelect_Errors(t *testing.T) {
	cases := []struct {
		in     string
		n      int
		errSub string
	}{
		{"", 3, "no items selected"},
		{",,,", 3, "no items selected"},
		{"abc", 3, "not a number"},
		{"0", 3, "out of range"},
		{"4", 3, "out of range"},
		{"1,x", 3, "not a number"},
	}
	for _, c := range cases {
		_, err := parseMultiSelect(c.in, c.n)
		if err == nil {
			t.Errorf("parseMultiSelect(%q): expected error containing %q", c.in, c.errSub)
			continue
		}
		if !strings.Contains(err.Error(), c.errSub) {
			t.Errorf("parseMultiSelect(%q): err %q does not contain %q", c.in, err.Error(), c.errSub)
		}
	}
}

// ---------------------------------------------------------------------
// decision builders
// ---------------------------------------------------------------------

func TestDecisionAcceptAll(t *testing.T) {
	d := decisionAcceptAll(3)
	if d.Kind != driven.DecisionAccept {
		t.Fatalf("Kind: got %d, want DecisionAccept", d.Kind)
	}
	if len(d.PerItem) != 3 {
		t.Fatalf("PerItem len: got %d, want 3", len(d.PerItem))
	}
	for i, it := range d.PerItem {
		if it.Kind != driven.ItemYes {
			t.Errorf("PerItem[%d]: got %d, want ItemYes", i, it.Kind)
		}
	}
}

func TestDecisionAcceptNone(t *testing.T) {
	d := decisionAcceptNone(2)
	if d.Kind != driven.DecisionAccept {
		t.Fatalf("Kind: got %d, want DecisionAccept", d.Kind)
	}
	if len(d.PerItem) != 2 {
		t.Fatalf("PerItem len: got %d, want 2", len(d.PerItem))
	}
	for i, it := range d.PerItem {
		if it.Kind != driven.ItemNo {
			t.Errorf("PerItem[%d]: got %d, want ItemNo", i, it.Kind)
		}
	}
}

func TestDecisionAcceptSubset(t *testing.T) {
	d := decisionAcceptSubset(5, []int{0, 2, 4})
	if d.Kind != driven.DecisionAccept {
		t.Fatalf("Kind: got %d, want DecisionAccept", d.Kind)
	}
	want := []driven.ItemDecisionKind{
		driven.ItemYes, driven.ItemNo, driven.ItemYes, driven.ItemNo, driven.ItemYes,
	}
	for i, it := range d.PerItem {
		if it.Kind != want[i] {
			t.Errorf("PerItem[%d]: got %d, want %d", i, it.Kind, want[i])
		}
	}
}

// ---------------------------------------------------------------------
// Multi-item Review — end-to-end with scripted stdin
// ---------------------------------------------------------------------

// fakeEditor records the buffer it was asked to edit and returns a
// pre-scripted replacement.
type fakeEditor struct {
	seen   string
	reply  string
	replyErr error
}

func (f *fakeEditor) Edit(buffer string) (string, error) {
	f.seen = buffer
	if f.replyErr != nil {
		return "", f.replyErr
	}
	return f.reply, nil
}

// newMultiProposal is the fixture the tests below review against.
func newMultiProposal() driven.Proposal {
	return driven.Proposal{
		Format: driven.ProposalMultiple,
		Multiple: []driven.ProposalItem{
			{Title: "One", Body: "the first"},
			{Title: "Two", Body: "the second"},
			{Title: "Three", Body: "the third"},
		},
		RawJSON: `{"items":[{"title":"One"},{"title":"Two"},{"title":"Three"}]}`,
	}
}

func runReviewMulti(t *testing.T, stdin string, editor *fakeEditor) (driven.Decision, string, error) {
	t.Helper()
	var out bytes.Buffer
	in := strings.NewReader(stdin)
	i := NewWithIO(in, &out, editor)
	dec, err := i.Review(context.Background(), newMultiProposal())
	return dec, out.String(), err
}

func TestReviewMulti_AcceptAll(t *testing.T) {
	dec, out, err := runReviewMulti(t, "a\n", &fakeEditor{})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if dec.Kind != driven.DecisionAccept {
		t.Fatalf("Kind: got %d, want DecisionAccept", dec.Kind)
	}
	if len(dec.PerItem) != 3 {
		t.Fatalf("PerItem len: got %d, want 3", len(dec.PerItem))
	}
	for i, it := range dec.PerItem {
		if it.Kind != driven.ItemYes {
			t.Errorf("PerItem[%d]: got %d, want Yes", i, it.Kind)
		}
	}
	if !strings.Contains(out, "[1] One") || !strings.Contains(out, "[3] Three") {
		t.Errorf("expected all three items rendered in output; got:\n%s", out)
	}
}

func TestReviewMulti_AcceptSubset(t *testing.T) {
	dec, _, err := runReviewMulti(t, "1,3\n", &fakeEditor{})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if dec.Kind != driven.DecisionAccept {
		t.Fatalf("Kind: got %d, want DecisionAccept", dec.Kind)
	}
	want := []driven.ItemDecisionKind{driven.ItemYes, driven.ItemNo, driven.ItemYes}
	for i, it := range dec.PerItem {
		if it.Kind != want[i] {
			t.Errorf("PerItem[%d]: got %d, want %d", i, it.Kind, want[i])
		}
	}
}

func TestReviewMulti_AcceptNone(t *testing.T) {
	dec, _, err := runReviewMulti(t, "n\n", &fakeEditor{})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if dec.Kind != driven.DecisionAccept {
		t.Fatalf("Kind: got %d, want DecisionAccept (ai_loop routes none→abort)", dec.Kind)
	}
	for i, it := range dec.PerItem {
		if it.Kind != driven.ItemNo {
			t.Errorf("PerItem[%d]: got %d, want No", i, it.Kind)
		}
	}
}

func TestReviewMulti_Quit(t *testing.T) {
	dec, _, err := runReviewMulti(t, "q\n", &fakeEditor{})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if dec.Kind != driven.DecisionAbort {
		t.Fatalf("Kind: got %d, want DecisionAbort", dec.Kind)
	}
}

func TestReviewMulti_Critique(t *testing.T) {
	stdin := "c\nneeds more concrete outcomes\n.\n"
	dec, _, err := runReviewMulti(t, stdin, &fakeEditor{})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if dec.Kind != driven.DecisionCritique {
		t.Fatalf("Kind: got %d, want DecisionCritique", dec.Kind)
	}
	if !strings.Contains(dec.Critique, "needs more concrete outcomes") {
		t.Fatalf("critique text: got %q", dec.Critique)
	}
}

func TestReviewMulti_EditRoutesThroughEditor(t *testing.T) {
	editor := &fakeEditor{reply: `{"items":[{"title":"only one"}]}`}
	dec, _, err := runReviewMulti(t, "e\n", editor)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if dec.Kind != driven.DecisionEditList {
		t.Fatalf("Kind: got %d, want DecisionEditList", dec.Kind)
	}
	if !strings.Contains(editor.seen, "One") || !strings.Contains(editor.seen, "Two") {
		t.Fatalf("editor did not receive RawJSON with original items; got %q", editor.seen)
	}
	if dec.Edited != editor.reply {
		t.Fatalf("Edited: got %q, want %q", dec.Edited, editor.reply)
	}
}

func TestReviewMulti_InvalidThenAccept(t *testing.T) {
	// First input "xyz" should re-prompt with a descriptive message;
	// second input accepts all.
	stdin := "xyz\na\n"
	dec, out, err := runReviewMulti(t, stdin, &fakeEditor{})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if dec.Kind != driven.DecisionAccept {
		t.Fatalf("Kind: got %d, want DecisionAccept", dec.Kind)
	}
	if !strings.Contains(out, "not a number") {
		t.Errorf("expected 'not a number' hint; got:\n%s", out)
	}
}

func TestReviewMulti_EmptyBatchAborts(t *testing.T) {
	// Degenerate case: an empty proposal returns DecisionAbort without
	// reading stdin.
	p := driven.Proposal{Format: driven.ProposalMultiple}
	var out bytes.Buffer
	i := NewWithIO(strings.NewReader(""), &out, &fakeEditor{})
	dec, err := i.Review(context.Background(), p)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if dec.Kind != driven.DecisionAbort {
		t.Fatalf("Kind: got %d, want DecisionAbort", dec.Kind)
	}
}

// ---------------------------------------------------------------------
// Single-item review still works after the refactor
// ---------------------------------------------------------------------

func TestReviewSingle_Accept(t *testing.T) {
	p := driven.Proposal{
		Format: driven.ProposalSingle,
		Single: &driven.ProposalItem{Title: "T", Body: "B", JSON: `{"k":"v"}`},
	}
	var out bytes.Buffer
	i := NewWithIO(strings.NewReader("a\n"), &out, &fakeEditor{})
	dec, err := i.Review(context.Background(), p)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if dec.Kind != driven.DecisionAccept {
		t.Fatalf("Kind: got %d, want DecisionAccept", dec.Kind)
	}
}

func TestReviewSingle_Edit(t *testing.T) {
	editor := &fakeEditor{reply: `{"k":"edited"}`}
	p := driven.Proposal{
		Format: driven.ProposalSingle,
		Single: &driven.ProposalItem{JSON: `{"k":"v"}`},
	}
	var out bytes.Buffer
	i := NewWithIO(strings.NewReader("e\n"), &out, editor)
	dec, err := i.Review(context.Background(), p)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if dec.Kind != driven.DecisionAccept {
		t.Fatalf("Kind: got %d, want DecisionAccept", dec.Kind)
	}
	if dec.Edited != editor.reply {
		t.Fatalf("Edited: got %q, want %q", dec.Edited, editor.reply)
	}
}

func TestReviewSingle_EditorError(t *testing.T) {
	editor := &fakeEditor{replyErr: errors.New("boom")}
	p := driven.Proposal{
		Format: driven.ProposalSingle,
		Single: &driven.ProposalItem{JSON: `{"k":"v"}`},
	}
	var out bytes.Buffer
	i := NewWithIO(strings.NewReader("e\n"), &out, editor)
	_, err := i.Review(context.Background(), p)
	if err == nil || !strings.Contains(err.Error(), "open editor") {
		t.Fatalf("expected open editor error, got %v", err)
	}
}
