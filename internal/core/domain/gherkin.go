package domain

import (
	"fmt"
	"strings"
)

// FormatScenarioAsGherkin renders a Scenario as a Gherkin Scenario
// block: any user tags on the line above the title, then the
// Scenario header, then the Given/When/Then sections with And
// continuations. Data tables and doc strings are indented under
// their parent step.
//
// This function does NOT prepend the mandatory @d7:<ID> tag — that
// is the responsibility of d7 export gherkin, which adds it when
// the Scenario leaves the DB. Storing it would let users edit it
// and break the SCEN-XXX mapping the verify loop relies on.
func FormatScenarioAsGherkin(s Scenario) string {
	var b strings.Builder

	if len(s.Tags) > 0 {
		for i, t := range s.Tags {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString("@")
			b.WriteString(t)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "Scenario: %s\n", s.Title)

	writeSection(&b, "Given", s.Given)
	writeSection(&b, "When", s.When)
	writeSection(&b, "Then", s.Then)

	return b.String()
}

// writeSection writes a Given/When/Then section. The first step
// uses the provided keyword; subsequent steps use And. Data tables
// and doc strings are indented under their parent step.
func writeSection(b *strings.Builder, keyword string, steps []Step) {
	for i, step := range steps {
		kw := keyword
		if i > 0 {
			kw = "And"
		}
		fmt.Fprintf(b, "  %s %s\n", kw, step.Text)

		if step.DocString != nil {
			writeDocString(b, *step.DocString)
		}
		if step.DataTable != nil {
			writeDataTable(b, *step.DataTable)
		}
	}
}

// writeDocString writes a Gherkin doc string block indented under
// its parent step. Each line of the payload is indented by four
// spaces; the """ delimiters are indented by four spaces as well
// (one extra indent beyond the step's two spaces). A single
// trailing newline on the payload is stripped so YAML block
// scalars (which preserve a final "\n") do not render a spurious
// blank line before the closing """.
func writeDocString(b *strings.Builder, payload string) {
	payload = strings.TrimSuffix(payload, "\n")
	b.WriteString("    \"\"\"\n")
	for _, line := range strings.Split(payload, "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString("    ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("    \"\"\"\n")
}

// writeDataTable writes a Gherkin data table indented under its
// parent step. Columns are padded to the widest cell in each
// column so the table is readable.
func writeDataTable(b *strings.Builder, t DataTable) {
	rows := make([][]string, 0, len(t.Rows)+1)
	if len(t.Headers) > 0 {
		rows = append(rows, t.Headers)
	}
	rows = append(rows, t.Rows...)
	if len(rows) == 0 {
		return
	}

	// Compute per-column widths.
	ncols := 0
	for _, r := range rows {
		if len(r) > ncols {
			ncols = len(r)
		}
	}
	widths := make([]int, ncols)
	for _, r := range rows {
		for i, cell := range r {
			if n := len(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}

	for _, r := range rows {
		b.WriteString("    ")
		for i := 0; i < ncols; i++ {
			cell := ""
			if i < len(r) {
				cell = r[i]
			}
			if i == 0 {
				b.WriteString("| ")
			}
			fmt.Fprintf(b, "%-*s | ", widths[i], cell)
		}
		b.WriteString("\n")
	}
}
