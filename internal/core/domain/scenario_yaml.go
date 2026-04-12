package domain

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// scenarioYAMLStep is the wire shape of a Step in the YAML document.
// Gopkg.yaml.v3 will marshal/unmarshal this via the struct tags.
type scenarioYAMLStep struct {
	Text      string              `yaml:"text"`
	DataTable *scenarioYAMLTable  `yaml:"data_table,omitempty"`
	DocString *string             `yaml:"doc_string,omitempty"`
}

// scenarioYAMLTable is the wire shape of a DataTable.
type scenarioYAMLTable struct {
	Headers []string   `yaml:"headers,omitempty"`
	Rows    [][]string `yaml:"rows"`
}

// scenarioYAMLDoc is the wire shape of the whole editable YAML
// document. Read-only fields (id, spec, created) are rendered as
// YAML comments above the document, not as real fields, so the
// parser never sees them.
type scenarioYAMLDoc struct {
	Status string             `yaml:"status"`
	Title  string             `yaml:"title"`
	Tags   []string           `yaml:"tags,omitempty"`
	Given  []scenarioYAMLStep `yaml:"given,omitempty"`
	When   []scenarioYAMLStep `yaml:"when,omitempty"`
	Then   []scenarioYAMLStep `yaml:"then,omitempty"`
}

// FormatScenarioYAML renders a Scenario as an editable YAML
// document. The identifying fields (id, spec, created) are
// emitted as YAML comments so they survive the round-trip as
// context but are not parsed back as data.
func FormatScenarioYAML(s Scenario) (string, error) {
	doc := scenarioYAMLDoc{
		Status: string(s.Status),
		Title:  s.Title,
		Tags:   s.Tags,
		Given:  stepsToYAML(s.Given),
		When:   stepsToYAML(s.When),
		Then:   stepsToYAML(s.Then),
	}

	payload, err := yaml.Marshal(&doc)
	if err != nil {
		return "", fmt.Errorf("marshal scenario yaml: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# id: %s        # read-only\n", s.ID)
	fmt.Fprintf(&b, "# spec: %s      # read-only\n", s.SpecID)
	fmt.Fprintf(&b, "# created: %s   # read-only\n", s.CreatedAt.Format("2006-01-02"))
	b.WriteString("\n")
	b.Write(payload)
	return b.String(), nil
}

// ScenarioYAMLResult is the parsed form of the editable YAML
// document. The CLI layer compares these against the loaded
// Scenario to compute which fields actually changed.
type ScenarioYAMLResult struct {
	Status string
	Title  string
	Tags   []string
	Given  []Step
	When   []Step
	Then   []Step
}

// ErrMalformedScenarioYAML is returned when the YAML body cannot
// be parsed into the expected structure.
var ErrMalformedScenarioYAML = errors.New("malformed scenario yaml")

// ParseScenarioYAML parses the YAML produced by FormatScenarioYAML
// back into a ScenarioYAMLResult. Comment lines (including the
// read-only id/spec/created markers) are ignored by yaml.v3.
func ParseScenarioYAML(content string) (ScenarioYAMLResult, error) {
	var doc scenarioYAMLDoc
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return ScenarioYAMLResult{}, fmt.Errorf("%w: %v", ErrMalformedScenarioYAML, err)
	}

	return ScenarioYAMLResult{
		Status: strings.TrimSpace(doc.Status),
		Title:  strings.TrimSpace(doc.Title),
		Tags:   normalizeTags(doc.Tags),
		Given:  yamlToSteps(doc.Given),
		When:   yamlToSteps(doc.When),
		Then:   yamlToSteps(doc.Then),
	}, nil
}

// normalizeTags trims whitespace and strips any leading @ sign
// users may have typed out of habit.
func normalizeTags(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.TrimSpace(t)
		t = strings.TrimPrefix(t, "@")
		if t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// stepsToYAML converts domain Steps to their YAML wire form.
func stepsToYAML(steps []Step) []scenarioYAMLStep {
	if len(steps) == 0 {
		return nil
	}
	out := make([]scenarioYAMLStep, len(steps))
	for i, s := range steps {
		ys := scenarioYAMLStep{Text: s.Text}
		if s.DataTable != nil {
			ys.DataTable = &scenarioYAMLTable{
				Headers: s.DataTable.Headers,
				Rows:    s.DataTable.Rows,
			}
		}
		if s.DocString != nil {
			ds := *s.DocString
			ys.DocString = &ds
		}
		out[i] = ys
	}
	return out
}

// yamlToSteps converts YAML wire steps back into domain Steps.
func yamlToSteps(steps []scenarioYAMLStep) []Step {
	if len(steps) == 0 {
		return nil
	}
	out := make([]Step, len(steps))
	for i, ys := range steps {
		s := Step{Text: ys.Text}
		if ys.DataTable != nil {
			s.DataTable = &DataTable{
				Headers: ys.DataTable.Headers,
				Rows:    ys.DataTable.Rows,
			}
		}
		if ys.DocString != nil {
			ds := *ys.DocString
			s.DocString = &ds
		}
		out[i] = s
	}
	return out
}
