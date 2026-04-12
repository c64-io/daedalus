package service_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
	"github.com/c64-io/daedalus/internal/core/service"
)

// fakeScenarioRepo is an in-memory ScenarioRepository for testing.
type fakeScenarioRepo struct {
	seq       int
	scenarios []domain.Scenario
	nextErr   error
	saveErr   error
	getErr    error
	listErr   error
	updateErr error
}

func (r *fakeScenarioRepo) NextScenarioSeq(_ context.Context, _ string) (int, error) {
	if r.nextErr != nil {
		return 0, r.nextErr
	}
	r.seq++
	return r.seq, nil
}

func (r *fakeScenarioRepo) SaveScenario(_ context.Context, _ string, s domain.Scenario) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.scenarios = append(r.scenarios, s)
	return nil
}

func (r *fakeScenarioRepo) GetScenario(_ context.Context, _ string, id string) (*domain.Scenario, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	for i := range r.scenarios {
		if r.scenarios[i].ID == id {
			return &r.scenarios[i], nil
		}
	}
	return nil, driven.ErrScenarioNotFound
}

func (r *fakeScenarioRepo) ListScenarios(_ context.Context, _ string, specID string) ([]domain.Scenario, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	if specID == "" {
		return r.scenarios, nil
	}
	var filtered []domain.Scenario
	for _, s := range r.scenarios {
		if s.SpecID == specID {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func (r *fakeScenarioRepo) UpdateScenario(_ context.Context, _ string, s domain.Scenario) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	for i := range r.scenarios {
		if r.scenarios[i].ID == s.ID {
			r.scenarios[i] = s
			return nil
		}
	}
	return driven.ErrScenarioNotFound
}

// seedRefinedSpec adds a refined spec to the fake spec repo so
// scenario creation tests have a valid parent.
func seedRefinedSpec(repo *fakeSpecRepo, id string) {
	repo.specs = append(repo.specs, domain.Spec{
		ID:        id,
		StoryID:   "STORY-001",
		Title:     "Test Spec",
		Status:    domain.StatusRefined,
		CreatedAt: time.Now(),
	})
}

func TestCreateScenario_Success(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	specRepo := &fakeSpecRepo{}
	seedRefinedSpec(specRepo, "SPEC-001")
	scenRepo := &fakeScenarioRepo{}
	svc := service.NewScenarioService(fs, scenRepo, specRepo, &fakeHistoryRepo{})

	scen, err := svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{
		SpecID: "SPEC-001",
		Title:  "User redeems a valid coupon",
	})
	if err != nil {
		t.Fatalf("CreateScenario returned unexpected error: %v", err)
	}

	if scen.ID != "SCEN-001" {
		t.Errorf("scen.ID = %q, want %q", scen.ID, "SCEN-001")
	}
	if scen.SpecID != "SPEC-001" {
		t.Errorf("scen.SpecID = %q, want %q", scen.SpecID, "SPEC-001")
	}
	if scen.Status != domain.StatusDraft {
		t.Errorf("scen.Status = %q, want %q", scen.Status, domain.StatusDraft)
	}
	if len(scenRepo.scenarios) != 1 {
		t.Fatalf("expected 1 saved scenario, got %d", len(scenRepo.scenarios))
	}
}

func TestCreateScenario_MonotonicIDs(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	specRepo := &fakeSpecRepo{}
	seedRefinedSpec(specRepo, "SPEC-001")
	scenRepo := &fakeScenarioRepo{}
	svc := service.NewScenarioService(fs, scenRepo, specRepo, &fakeHistoryRepo{})

	for i := 1; i <= 3; i++ {
		scen, err := svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{
			SpecID: "SPEC-001",
			Title:  "t",
		})
		if err != nil {
			t.Fatalf("CreateScenario #%d returned error: %v", i, err)
		}
		want := domain.FormatScenarioID(i)
		if scen.ID != want {
			t.Errorf("scen #%d ID = %q, want %q", i, scen.ID, want)
		}
	}
}

func TestCreateScenario_TitleRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewScenarioService(fs, &fakeScenarioRepo{}, &fakeSpecRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{
		SpecID: "SPEC-001",
	})
	if !errors.Is(err, service.ErrScenarioTitleRequired) {
		t.Fatalf("CreateScenario err = %v, want ErrScenarioTitleRequired", err)
	}
}

func TestCreateScenario_SpecRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewScenarioService(fs, &fakeScenarioRepo{}, &fakeSpecRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{
		Title: "t",
	})
	if !errors.Is(err, service.ErrScenarioSpecRequired) {
		t.Fatalf("CreateScenario err = %v, want ErrScenarioSpecRequired", err)
	}
}

func TestCreateScenario_SpecNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewScenarioService(fs, &fakeScenarioRepo{}, &fakeSpecRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{
		SpecID: "SPEC-999",
		Title:  "t",
	})
	if !errors.Is(err, driven.ErrSpecNotFound) {
		t.Fatalf("CreateScenario err = %v, want ErrSpecNotFound", err)
	}
}

func TestCreateScenario_SpecNotRefined(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	specRepo := &fakeSpecRepo{}
	specRepo.specs = append(specRepo.specs, domain.Spec{
		ID: "SPEC-001", StoryID: "STORY-001", Title: "Draft Spec", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewScenarioService(fs, &fakeScenarioRepo{}, specRepo, &fakeHistoryRepo{})

	_, err := svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{
		SpecID: "SPEC-001",
		Title:  "t",
	})
	if !errors.Is(err, service.ErrSpecNotReady) {
		t.Fatalf("CreateScenario err = %v, want ErrSpecNotReady", err)
	}
}

func TestCreateScenario_SpecArchived(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	specRepo := &fakeSpecRepo{}
	specRepo.specs = append(specRepo.specs, domain.Spec{
		ID: "SPEC-001", StoryID: "STORY-001", Title: "Archived Spec", Status: domain.StatusArchived, CreatedAt: time.Now(),
	})
	svc := service.NewScenarioService(fs, &fakeScenarioRepo{}, specRepo, &fakeHistoryRepo{})

	_, err := svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{
		SpecID: "SPEC-001",
		Title:  "t",
	})
	if !errors.Is(err, service.ErrSpecNotReady) {
		t.Fatalf("CreateScenario err = %v, want ErrSpecNotReady", err)
	}
}

func TestCreateScenario_WorkspaceNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/empty")
	svc := service.NewScenarioService(fs, &fakeScenarioRepo{}, &fakeSpecRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{
		SpecID: "SPEC-001",
		Title:  "t",
	})
	if !errors.Is(err, service.ErrWorkspaceNotFound) {
		t.Fatalf("CreateScenario err = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestCreateScenario_WithInitialSteps(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	specRepo := &fakeSpecRepo{}
	seedRefinedSpec(specRepo, "SPEC-001")
	scenRepo := &fakeScenarioRepo{}
	svc := service.NewScenarioService(fs, scenRepo, specRepo, &fakeHistoryRepo{})

	given := []domain.Step{{Text: "a user has a coupon"}}
	when := []domain.Step{{Text: "the user applies it"}}
	then := []domain.Step{{Text: "the total is reduced"}}

	scen, err := svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{
		SpecID: "SPEC-001",
		Title:  "Redeem coupon",
		Tags:   []string{"happy-path"},
		Given:  given,
		When:   when,
		Then:   then,
	})
	if err != nil {
		t.Fatalf("CreateScenario returned error: %v", err)
	}
	if !reflect.DeepEqual(scen.Given, given) {
		t.Errorf("Given = %#v, want %#v", scen.Given, given)
	}
	if !reflect.DeepEqual(scen.When, when) {
		t.Errorf("When = %#v, want %#v", scen.When, when)
	}
	if !reflect.DeepEqual(scen.Then, then) {
		t.Errorf("Then = %#v, want %#v", scen.Then, then)
	}
	if !reflect.DeepEqual(scen.Tags, []string{"happy-path"}) {
		t.Errorf("Tags = %#v, want [happy-path]", scen.Tags)
	}
}

func TestListScenarios_FilterBySpec(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	specRepo := &fakeSpecRepo{}
	seedRefinedSpec(specRepo, "SPEC-001")
	seedRefinedSpec(specRepo, "SPEC-002")
	scenRepo := &fakeScenarioRepo{}
	svc := service.NewScenarioService(fs, scenRepo, specRepo, &fakeHistoryRepo{})

	_, _ = svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{SpecID: "SPEC-001", Title: "A"})
	_, _ = svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{SpecID: "SPEC-002", Title: "B"})
	_, _ = svc.CreateScenario(context.Background(), driving.CreateScenarioRequest{SpecID: "SPEC-001", Title: "C"})

	all, err := svc.ListScenarios(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListScenarios (all) returned error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 scenarios, got %d", len(all))
	}

	filtered, err := svc.ListScenarios(context.Background(), "", "SPEC-001")
	if err != nil {
		t.Fatalf("ListScenarios (filtered) returned error: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 scenarios for SPEC-001, got %d", len(filtered))
	}
}

func TestGetScenario_NotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewScenarioService(fs, &fakeScenarioRepo{}, &fakeSpecRepo{}, &fakeHistoryRepo{})

	_, err := svc.GetScenario(context.Background(), "", "SCEN-999")
	if !errors.Is(err, driven.ErrScenarioNotFound) {
		t.Fatalf("GetScenario err = %v, want ErrScenarioNotFound", err)
	}
}

func TestSetScenario_StatusTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	scenRepo := &fakeScenarioRepo{}
	scenRepo.scenarios = append(scenRepo.scenarios, domain.Scenario{
		ID: "SCEN-001", SpecID: "SPEC-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewScenarioService(fs, scenRepo, &fakeSpecRepo{}, histRepo)

	refined := domain.StatusRefined
	scen, err := svc.SetScenario(context.Background(), driving.SetScenarioRequest{
		ID: "SCEN-001", Status: &refined,
	})
	if err != nil {
		t.Fatalf("SetScenario returned error: %v", err)
	}
	if scen.Status != domain.StatusRefined {
		t.Errorf("scen.Status = %q, want refined", scen.Status)
	}
	if len(histRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(histRepo.entries))
	}
}

func TestSetScenario_Title(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	scenRepo := &fakeScenarioRepo{}
	scenRepo.scenarios = append(scenRepo.scenarios, domain.Scenario{
		ID: "SCEN-001", SpecID: "SPEC-001", Title: "Old", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewScenarioService(fs, scenRepo, &fakeSpecRepo{}, histRepo)

	newTitle := "New"
	scen, err := svc.SetScenario(context.Background(), driving.SetScenarioRequest{
		ID: "SCEN-001", Title: &newTitle,
	})
	if err != nil {
		t.Fatalf("SetScenario returned error: %v", err)
	}
	if scen.Title != "New" {
		t.Errorf("scen.Title = %q, want New", scen.Title)
	}
	if len(histRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(histRepo.entries))
	}
}

func TestSetScenario_Tags(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	scenRepo := &fakeScenarioRepo{}
	scenRepo.scenarios = append(scenRepo.scenarios, domain.Scenario{
		ID: "SCEN-001", SpecID: "SPEC-001", Title: "T", Tags: []string{"old"}, Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewScenarioService(fs, scenRepo, &fakeSpecRepo{}, histRepo)

	newTags := []string{"happy-path", "checkout"}
	scen, err := svc.SetScenario(context.Background(), driving.SetScenarioRequest{
		ID: "SCEN-001", Tags: &newTags,
	})
	if err != nil {
		t.Fatalf("SetScenario returned error: %v", err)
	}
	if !reflect.DeepEqual(scen.Tags, newTags) {
		t.Errorf("scen.Tags = %#v, want %#v", scen.Tags, newTags)
	}
}

func TestSetScenario_ReplaceGiven(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	scenRepo := &fakeScenarioRepo{}
	scenRepo.scenarios = append(scenRepo.scenarios, domain.Scenario{
		ID: "SCEN-001", SpecID: "SPEC-001", Title: "T",
		Given:     []domain.Step{{Text: "old"}},
		Status:    domain.StatusDraft,
		CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewScenarioService(fs, scenRepo, &fakeSpecRepo{}, histRepo)

	newGiven := []domain.Step{{Text: "new 1"}, {Text: "new 2"}}
	scen, err := svc.SetScenario(context.Background(), driving.SetScenarioRequest{
		ID: "SCEN-001", Given: &newGiven,
	})
	if err != nil {
		t.Fatalf("SetScenario returned error: %v", err)
	}
	if !reflect.DeepEqual(scen.Given, newGiven) {
		t.Errorf("scen.Given = %#v, want %#v", scen.Given, newGiven)
	}
	if len(histRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(histRepo.entries))
	}
	if histRepo.entries[0].Field != "given" {
		t.Errorf("history entry field = %q, want %q", histRepo.entries[0].Field, "given")
	}
}

func TestSetScenario_InvalidTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	scenRepo := &fakeScenarioRepo{}
	scenRepo.scenarios = append(scenRepo.scenarios, domain.Scenario{
		ID: "SCEN-001", SpecID: "SPEC-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewScenarioService(fs, scenRepo, &fakeSpecRepo{}, &fakeHistoryRepo{})

	done := domain.StatusDone
	_, err := svc.SetScenario(context.Background(), driving.SetScenarioRequest{
		ID: "SCEN-001", Status: &done,
	})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("SetScenario err = %v, want ErrInvalidTransition", err)
	}
}

func TestSetScenario_NoFields(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewScenarioService(fs, &fakeScenarioRepo{}, &fakeSpecRepo{}, &fakeHistoryRepo{})

	_, err := svc.SetScenario(context.Background(), driving.SetScenarioRequest{ID: "SCEN-001"})
	if !errors.Is(err, service.ErrNoFieldsToSet) {
		t.Fatalf("SetScenario err = %v, want ErrNoFieldsToSet", err)
	}
}
