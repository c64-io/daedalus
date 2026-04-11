package driving

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// CreateScenarioRequest is the input shape for the ScenarioCreator
// use case. Given/When/Then and Tags are optional at creation time —
// they let `d7 scenario new` seed simple plain-text steps from
// repeatable CLI flags without requiring the editor flow for
// quick/scripted authoring.
type CreateScenarioRequest struct {
	RootDir string        // workspace root; empty = cwd
	SpecID  string        // parent spec ID (required)
	Title   string        // required
	Tags    []string      // optional; stored without leading @
	Given   []domain.Step // optional; plain-text steps only via flags
	When    []domain.Step // optional
	Then    []domain.Step // optional
}

// ScenarioCreator is the driving port exposing the "create a new
// scenario" use case to inbound adapters.
type ScenarioCreator interface {
	CreateScenario(ctx context.Context, req CreateScenarioRequest) (*domain.Scenario, error)
}

// ScenarioReader is the driving port exposing the "read scenarios"
// use cases.
type ScenarioReader interface {
	GetScenario(ctx context.Context, rootDir string, id string) (*domain.Scenario, error)
	ListScenarios(ctx context.Context, rootDir string, specID string) ([]domain.Scenario, error)
}

// SetScenarioRequest describes which fields to update on a
// Scenario. Only non-nil pointer fields are applied; nil means
// "leave unchanged." The step slices use pointer-to-slice for full
// replacement semantics: a non-nil *[]Step replaces the entire
// section, a nil pointer leaves it alone.
type SetScenarioRequest struct {
	RootDir string          // workspace root; empty = cwd
	ID      string          // scenario to update (required)
	Status  *domain.Status  // new status (validated against state machine)
	Title   *string         // new title
	Tags    *[]string       // full replacement of tag list
	Given   *[]domain.Step  // full replacement of Given steps
	When    *[]domain.Step  // full replacement of When steps
	Then    *[]domain.Step  // full replacement of Then steps
}

// ScenarioSetter is the driving port for updating Scenario fields.
type ScenarioSetter interface {
	SetScenario(ctx context.Context, req SetScenarioRequest) (*domain.Scenario, error)
}
