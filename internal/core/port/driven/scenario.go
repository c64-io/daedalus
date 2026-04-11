package driven

import (
	"context"
	"errors"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// ScenarioRepository is the driven port for persistent storage of
// Scenario entities.
type ScenarioRepository interface {
	// NextScenarioSeq atomically increments and returns the next
	// monotonic sequence number for Scenario IDs.
	NextScenarioSeq(ctx context.Context, dbDir string) (int, error)

	// SaveScenario persists a fully constructed Scenario.
	SaveScenario(ctx context.Context, dbDir string, s domain.Scenario) error

	// GetScenario returns the Scenario with the given human-readable
	// ID, or a wrapped ErrScenarioNotFound if it does not exist.
	GetScenario(ctx context.Context, dbDir string, id string) (*domain.Scenario, error)

	// ListScenarios returns all Scenarios in creation order. If
	// specID is non-empty, only Scenarios belonging to that Spec
	// are returned.
	ListScenarios(ctx context.Context, dbDir string, specID string) ([]domain.Scenario, error)

	// UpdateScenario replaces the stored Scenario identified by
	// s.ID with the provided values. Returns ErrScenarioNotFound if
	// it does not exist.
	UpdateScenario(ctx context.Context, dbDir string, s domain.Scenario) error
}

// ErrScenarioNotFound is returned by ScenarioRepository.GetScenario
// when the requested Scenario does not exist.
var ErrScenarioNotFound = errors.New("scenario not found")
