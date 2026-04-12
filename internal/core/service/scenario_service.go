package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
)

// Sentinel errors for ScenarioService.
var (
	ErrScenarioTitleRequired = errors.New("scenario title is required")
	ErrScenarioSpecRequired  = errors.New("parent spec is required")
	ErrSpecNotReady          = errors.New("parent spec must be at least refined to create scenarios")
)

// minSpecStatusForScenario is the set of Spec statuses that allow
// child Scenario creation. The spec must be at least refined.
var minSpecStatusForScenario = map[domain.Status]bool{
	domain.StatusRefined:    true,
	domain.StatusReady:      true,
	domain.StatusInProgress: true,
	domain.StatusReview:     true,
	domain.StatusDone:       true,
}

// Compile-time assertions.
var (
	_ driving.ScenarioCreator = (*ScenarioService)(nil)
	_ driving.ScenarioReader  = (*ScenarioService)(nil)
	_ driving.ScenarioSetter  = (*ScenarioService)(nil)
)

// ScenarioService implements the ScenarioCreator, ScenarioReader,
// and ScenarioSetter use cases.
type ScenarioService struct {
	fs           driven.FileSystem
	scenarioRepo driven.ScenarioRepository
	specRepo     driven.SpecRepository
	history      driven.HistoryRepository
}

// NewScenarioService wires the service with its driven dependencies.
func NewScenarioService(fs driven.FileSystem, scenarioRepo driven.ScenarioRepository, specRepo driven.SpecRepository, history driven.HistoryRepository) *ScenarioService {
	return &ScenarioService{
		fs:           fs,
		scenarioRepo: scenarioRepo,
		specRepo:     specRepo,
		history:      history,
	}
}

// CreateScenario validates the request, checks the parent Spec
// exists and is at least refined, mints the next SCEN-XXX ID, and
// persists the new Scenario with status "draft". Initial steps and
// tags from the request are seeded into the persisted record.
func (s *ScenarioService) CreateScenario(ctx context.Context, req driving.CreateScenarioRequest) (*domain.Scenario, error) {
	if req.SpecID == "" {
		return nil, ErrScenarioSpecRequired
	}
	if req.Title == "" {
		return nil, ErrScenarioTitleRequired
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	// Validate parent spec exists and is at least refined.
	spec, err := s.specRepo.GetSpec(ctx, ws.DBDir, req.SpecID)
	if err != nil {
		return nil, fmt.Errorf("validate parent spec: %w", err)
	}
	if !minSpecStatusForScenario[spec.Status] {
		return nil, fmt.Errorf("%w: %s is %s", ErrSpecNotReady, spec.ID, spec.Status)
	}

	seq, err := s.scenarioRepo.NextScenarioSeq(ctx, ws.DBDir)
	if err != nil {
		return nil, fmt.Errorf("mint scenario ID: %w", err)
	}

	scen := domain.Scenario{
		ID:        domain.FormatScenarioID(seq),
		SpecID:    req.SpecID,
		Title:     req.Title,
		Tags:      req.Tags,
		Given:     req.Given,
		When:      req.When,
		Then:      req.Then,
		Status:    domain.StatusDraft,
		CreatedAt: time.Now(),
	}

	if err := s.scenarioRepo.SaveScenario(ctx, ws.DBDir, scen); err != nil {
		return nil, fmt.Errorf("save scenario: %w", err)
	}

	return &scen, nil
}

// GetScenario resolves the workspace and delegates to the repository.
func (s *ScenarioService) GetScenario(ctx context.Context, rootDir string, id string) (*domain.Scenario, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	scen, err := s.scenarioRepo.GetScenario(ctx, ws.DBDir, id)
	if err != nil {
		return nil, fmt.Errorf("get scenario: %w", err)
	}
	return scen, nil
}

// ListScenarios resolves the workspace and delegates to the
// repository. When specID is empty, all scenarios are returned.
func (s *ScenarioService) ListScenarios(ctx context.Context, rootDir string, specID string) ([]domain.Scenario, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}
	scens, err := s.scenarioRepo.ListScenarios(ctx, ws.DBDir, specID)
	if err != nil {
		return nil, fmt.Errorf("list scenarios: %w", err)
	}
	return scens, nil
}

// SetScenario applies the requested field changes to an existing
// Scenario, validates status transitions, records history entries,
// and persists. Step slices are full-replacement: a non-nil
// *[]Step replaces the section entirely; nil leaves it untouched.
func (s *ScenarioService) SetScenario(ctx context.Context, req driving.SetScenarioRequest) (*domain.Scenario, error) {
	if req.ID == "" {
		return nil, ErrScenarioTitleRequired
	}
	if req.Status == nil && req.Title == nil && req.Tags == nil &&
		req.Given == nil && req.When == nil && req.Then == nil {
		return nil, ErrNoFieldsToSet
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	scen, err := s.scenarioRepo.GetScenario(ctx, ws.DBDir, req.ID)
	if err != nil {
		return nil, fmt.Errorf("get scenario: %w", err)
	}

	now := time.Now()
	var entries []domain.HistoryEntry

	if req.Status != nil && *req.Status != scen.Status {
		newStatus, err := scen.Status.Transition(*req.Status)
		if err != nil {
			return nil, err
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: scen.ID, Field: "status",
			OldValue: string(scen.Status), NewValue: string(newStatus),
			Timestamp: now,
		})
		scen.Status = newStatus
	}

	if req.Title != nil && *req.Title != scen.Title {
		if *req.Title == "" {
			return nil, ErrScenarioTitleRequired
		}
		entries = append(entries, domain.HistoryEntry{
			EntityID: scen.ID, Field: "title",
			OldValue: scen.Title, NewValue: *req.Title,
			Timestamp: now,
		})
		scen.Title = *req.Title
	}

	if req.Tags != nil && !stringSlicesEqual(scen.Tags, *req.Tags) {
		entries = append(entries, domain.HistoryEntry{
			EntityID: scen.ID, Field: "tags",
			OldValue: fmt.Sprintf("%d tags", len(scen.Tags)),
			NewValue: fmt.Sprintf("%d tags", len(*req.Tags)),
			Timestamp: now,
		})
		scen.Tags = *req.Tags
	}

	if req.Given != nil && !stepsEqual(scen.Given, *req.Given) {
		entries = append(entries, domain.HistoryEntry{
			EntityID: scen.ID, Field: "given",
			OldValue: fmt.Sprintf("%d given steps", len(scen.Given)),
			NewValue: fmt.Sprintf("%d given steps", len(*req.Given)),
			Timestamp: now,
		})
		scen.Given = *req.Given
	}

	if req.When != nil && !stepsEqual(scen.When, *req.When) {
		entries = append(entries, domain.HistoryEntry{
			EntityID: scen.ID, Field: "when",
			OldValue: fmt.Sprintf("%d when steps", len(scen.When)),
			NewValue: fmt.Sprintf("%d when steps", len(*req.When)),
			Timestamp: now,
		})
		scen.When = *req.When
	}

	if req.Then != nil && !stepsEqual(scen.Then, *req.Then) {
		entries = append(entries, domain.HistoryEntry{
			EntityID: scen.ID, Field: "then",
			OldValue: fmt.Sprintf("%d then steps", len(scen.Then)),
			NewValue: fmt.Sprintf("%d then steps", len(*req.Then)),
			Timestamp: now,
		})
		scen.Then = *req.Then
	}

	if len(entries) == 0 {
		return scen, nil
	}

	if err := s.scenarioRepo.UpdateScenario(ctx, ws.DBDir, *scen); err != nil {
		return nil, fmt.Errorf("update scenario: %w", err)
	}

	if err := s.history.AppendHistory(ctx, ws.DBDir, entries); err != nil {
		return nil, fmt.Errorf("record history: %w", err)
	}

	return scen, nil
}

// stringSlicesEqual compares two string slices for structural
// equality. Nil and empty are treated as equal.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stepsEqual compares two Step slices for structural equality,
// including optional DataTable and DocString payloads.
func stepsEqual(a, b []domain.Step) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Text != b[i].Text {
			return false
		}
		if !docStringEqual(a[i].DocString, b[i].DocString) {
			return false
		}
		if !dataTableEqual(a[i].DataTable, b[i].DataTable) {
			return false
		}
	}
	return true
}

func docStringEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func dataTableEqual(a, b *domain.DataTable) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !stringSlicesEqual(a.Headers, b.Headers) {
		return false
	}
	if len(a.Rows) != len(b.Rows) {
		return false
	}
	for i := range a.Rows {
		if !stringSlicesEqual(a.Rows[i], b.Rows[i]) {
			return false
		}
	}
	return true
}
