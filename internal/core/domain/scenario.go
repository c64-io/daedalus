package domain

import "time"

// DataTable is a rectangular grid attached to a Step. In a Gherkin
// export it is rendered as a pipe-delimited table under its parent
// step line. Headers is optional (an empty Headers means the first
// row is a data row, not a header row).
type DataTable struct {
	Headers []string
	Rows    [][]string
}

// Step is one line of a Scenario (a Given, When, or Then). The
// kind is not stored here — it is implicit in which slice of
// Scenario the Step lives in (Scenario.Given, Scenario.When, or
// Scenario.Then). The first step of each section renders as
// Given/When/Then; subsequent steps render as And.
//
// A Step may optionally carry either a DataTable or a DocString
// payload. Both at once is not a normal Gherkin construct and is
// not supported in v1.
type Step struct {
	Text      string     // human-readable step text (required)
	DataTable *DataTable // optional pipe-delimited grid
	DocString *string    // optional multi-line string (Gherkin """...""")
}

// Scenario is a Given/When/Then specification attached to a Spec.
// It is the sixth and terminal level of the d7 hierarchy:
// Idea → Epic → Feature → Story → Spec → Scenario.
//
// Scenarios are stored as structured records and round-trip to real
// Gherkin .feature files via d7 export gherkin; the mandatory
// @d7:<ID> tag is appended at export time, never stored.
//
// Scenarios carry user tags (without the leading @), a Title, an
// ordered Given/When/Then slice for each Gherkin section, and a
// lifecycle Status. They do not carry a Description field: the
// steps are the content, and narrative context lives on the parent
// Spec.
type Scenario struct {
	ID        string    // e.g. "SCEN-001", stable and immutable
	SpecID    string    // parent Spec ID (e.g. "SPEC-001"), required
	Title     string    // short label, required, editable
	Tags      []string  // user tags, stored without leading @
	Given     []Step    // ordered; renders as Given / And / And ...
	When      []Step    // ordered; renders as When / And / And ...
	Then      []Step    // ordered; renders as Then / And / And ...
	Status    Status    // lifecycle state; starts as StatusDraft
	CreatedAt time.Time // set once at creation, never mutated
}

// ScenarioIDPrefix is the prefix used for Scenario human-readable
// IDs.
const ScenarioIDPrefix = "SCEN"

// FormatScenarioID builds a human-readable Scenario ID from a
// sequence number: FormatScenarioID(1) → "SCEN-001".
func FormatScenarioID(seq int) string {
	return formatID(ScenarioIDPrefix, seq)
}
