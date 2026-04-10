package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
	"github.com/c64-io/daedalus/internal/core/service"
)

// fakeFeatureRepo is an in-memory FeatureRepository for testing.
type fakeFeatureRepo struct {
	seq       int
	features  []domain.Feature
	nextErr   error
	saveErr   error
	getErr    error
	listErr   error
	updateErr error
}

func (r *fakeFeatureRepo) NextFeatureSeq(_ context.Context, _ string) (int, error) {
	if r.nextErr != nil {
		return 0, r.nextErr
	}
	r.seq++
	return r.seq, nil
}

func (r *fakeFeatureRepo) SaveFeature(_ context.Context, _ string, feature domain.Feature) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.features = append(r.features, feature)
	return nil
}

func (r *fakeFeatureRepo) GetFeature(_ context.Context, _ string, id string) (*domain.Feature, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	for i := range r.features {
		if r.features[i].ID == id {
			return &r.features[i], nil
		}
	}
	return nil, driven.ErrFeatureNotFound
}

func (r *fakeFeatureRepo) ListFeatures(_ context.Context, _ string, epicID string) ([]domain.Feature, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	if epicID == "" {
		return r.features, nil
	}
	var filtered []domain.Feature
	for _, f := range r.features {
		if f.EpicID == epicID {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}

func (r *fakeFeatureRepo) UpdateFeature(_ context.Context, _ string, feature domain.Feature) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	for i := range r.features {
		if r.features[i].ID == feature.ID {
			r.features[i] = feature
			return nil
		}
	}
	return driven.ErrFeatureNotFound
}

// seedRefinedEpic adds a refined epic to the fake epic repo so feature
// creation tests have a valid parent.
func seedRefinedEpic(repo *fakeEpicRepo, id string) {
	repo.epics = append(repo.epics, domain.Epic{
		ID:        id,
		IdeaID:    "IDEA-001",
		Title:     "Test Epic",
		Status:    domain.StatusRefined,
		CreatedAt: time.Now(),
	})
}

func TestCreateFeature_Success(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	epicRepo := &fakeEpicRepo{}
	seedRefinedEpic(epicRepo, "EPIC-001")
	featureRepo := &fakeFeatureRepo{}
	svc := service.NewFeatureService(fs, featureRepo, epicRepo, &fakeHistoryRepo{})

	feature, err := svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{
		EpicID:      "EPIC-001",
		Title:       "User Login",
		Description: "OAuth-based login",
		Priority:    domain.PriorityHigh,
		Size:        domain.Size5,
	})
	if err != nil {
		t.Fatalf("CreateFeature returned unexpected error: %v", err)
	}

	if feature.ID != "FEAT-001" {
		t.Errorf("feature.ID = %q, want %q", feature.ID, "FEAT-001")
	}
	if feature.EpicID != "EPIC-001" {
		t.Errorf("feature.EpicID = %q, want %q", feature.EpicID, "EPIC-001")
	}
	if feature.Title != "User Login" {
		t.Errorf("feature.Title = %q, want %q", feature.Title, "User Login")
	}
	if feature.Status != domain.StatusDraft {
		t.Errorf("feature.Status = %q, want %q", feature.Status, domain.StatusDraft)
	}
	if feature.Priority != domain.PriorityHigh {
		t.Errorf("feature.Priority = %q, want %q", feature.Priority, domain.PriorityHigh)
	}
	if feature.Size != domain.Size5 {
		t.Errorf("feature.Size = %d, want %d", feature.Size, domain.Size5)
	}
	if len(featureRepo.features) != 1 {
		t.Fatalf("expected 1 saved feature, got %d", len(featureRepo.features))
	}
}

func TestCreateFeature_MonotonicIDs(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	epicRepo := &fakeEpicRepo{}
	seedRefinedEpic(epicRepo, "EPIC-001")
	featureRepo := &fakeFeatureRepo{}
	svc := service.NewFeatureService(fs, featureRepo, epicRepo, &fakeHistoryRepo{})

	for i := 1; i <= 3; i++ {
		feature, err := svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{
			EpicID: "EPIC-001",
			Title:  "feature",
		})
		if err != nil {
			t.Fatalf("CreateFeature #%d returned error: %v", i, err)
		}
		want := domain.FormatFeatureID(i)
		if feature.ID != want {
			t.Errorf("feature #%d ID = %q, want %q", i, feature.ID, want)
		}
	}
}

func TestCreateFeature_TitleRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewFeatureService(fs, &fakeFeatureRepo{}, &fakeEpicRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{
		EpicID: "EPIC-001",
	})
	if !errors.Is(err, service.ErrFeatureTitleRequired) {
		t.Fatalf("CreateFeature err = %v, want ErrFeatureTitleRequired", err)
	}
}

func TestCreateFeature_EpicRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewFeatureService(fs, &fakeFeatureRepo{}, &fakeEpicRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{
		Title: "Login",
	})
	if !errors.Is(err, service.ErrFeatureEpicRequired) {
		t.Fatalf("CreateFeature err = %v, want ErrFeatureEpicRequired", err)
	}
}

func TestCreateFeature_EpicNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewFeatureService(fs, &fakeFeatureRepo{}, &fakeEpicRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{
		EpicID: "EPIC-999",
		Title:  "Login",
	})
	if !errors.Is(err, driven.ErrEpicNotFound) {
		t.Fatalf("CreateFeature err = %v, want ErrEpicNotFound", err)
	}
}

func TestCreateFeature_EpicNotRefined(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	epicRepo := &fakeEpicRepo{}
	epicRepo.epics = append(epicRepo.epics, domain.Epic{
		ID: "EPIC-001", IdeaID: "IDEA-001", Title: "Draft Epic", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewFeatureService(fs, &fakeFeatureRepo{}, epicRepo, &fakeHistoryRepo{})

	_, err := svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{
		EpicID: "EPIC-001",
		Title:  "Login",
	})
	if !errors.Is(err, service.ErrEpicNotReady) {
		t.Fatalf("CreateFeature err = %v, want ErrEpicNotReady", err)
	}
}

func TestCreateFeature_EpicArchived(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	epicRepo := &fakeEpicRepo{}
	epicRepo.epics = append(epicRepo.epics, domain.Epic{
		ID: "EPIC-001", IdeaID: "IDEA-001", Title: "Archived Epic", Status: domain.StatusArchived, CreatedAt: time.Now(),
	})
	svc := service.NewFeatureService(fs, &fakeFeatureRepo{}, epicRepo, &fakeHistoryRepo{})

	_, err := svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{
		EpicID: "EPIC-001",
		Title:  "Login",
	})
	if !errors.Is(err, service.ErrEpicNotReady) {
		t.Fatalf("CreateFeature err = %v, want ErrEpicNotReady", err)
	}
}

func TestCreateFeature_WorkspaceNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/empty")
	svc := service.NewFeatureService(fs, &fakeFeatureRepo{}, &fakeEpicRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{
		EpicID: "EPIC-001",
		Title:  "x",
	})
	if !errors.Is(err, service.ErrWorkspaceNotFound) {
		t.Fatalf("CreateFeature err = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestCreateFeature_OptionalPrioritySize(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	epicRepo := &fakeEpicRepo{}
	seedRefinedEpic(epicRepo, "EPIC-001")
	featureRepo := &fakeFeatureRepo{}
	svc := service.NewFeatureService(fs, featureRepo, epicRepo, &fakeHistoryRepo{})

	feature, err := svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{
		EpicID: "EPIC-001",
		Title:  "No priority or size",
	})
	if err != nil {
		t.Fatalf("CreateFeature returned error: %v", err)
	}
	if feature.Priority != "" {
		t.Errorf("expected empty priority, got %q", feature.Priority)
	}
	if feature.Size != 0 {
		t.Errorf("expected zero size, got %d", feature.Size)
	}
}

func TestListFeatures_FilterByEpic(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	epicRepo := &fakeEpicRepo{}
	seedRefinedEpic(epicRepo, "EPIC-001")
	seedRefinedEpic(epicRepo, "EPIC-002")
	featureRepo := &fakeFeatureRepo{}
	svc := service.NewFeatureService(fs, featureRepo, epicRepo, &fakeHistoryRepo{})

	_, _ = svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{EpicID: "EPIC-001", Title: "A"})
	_, _ = svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{EpicID: "EPIC-002", Title: "B"})
	_, _ = svc.CreateFeature(context.Background(), driving.CreateFeatureRequest{EpicID: "EPIC-001", Title: "C"})

	all, err := svc.ListFeatures(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListFeatures (all) returned error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 features, got %d", len(all))
	}

	filtered, err := svc.ListFeatures(context.Background(), "", "EPIC-001")
	if err != nil {
		t.Fatalf("ListFeatures (filtered) returned error: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 features for EPIC-001, got %d", len(filtered))
	}
}

func TestGetFeature_NotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewFeatureService(fs, &fakeFeatureRepo{}, &fakeEpicRepo{}, &fakeHistoryRepo{})

	_, err := svc.GetFeature(context.Background(), "", "FEAT-999")
	if !errors.Is(err, driven.ErrFeatureNotFound) {
		t.Fatalf("GetFeature err = %v, want ErrFeatureNotFound", err)
	}
}

func TestSetFeature_StatusTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	featureRepo := &fakeFeatureRepo{}
	featureRepo.features = append(featureRepo.features, domain.Feature{
		ID: "FEAT-001", EpicID: "EPIC-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewFeatureService(fs, featureRepo, &fakeEpicRepo{}, histRepo)

	refined := domain.StatusRefined
	feature, err := svc.SetFeature(context.Background(), driving.SetFeatureRequest{
		ID: "FEAT-001", Status: &refined,
	})
	if err != nil {
		t.Fatalf("SetFeature returned error: %v", err)
	}
	if feature.Status != domain.StatusRefined {
		t.Errorf("feature.Status = %q, want refined", feature.Status)
	}
	if len(histRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(histRepo.entries))
	}
}

func TestSetFeature_PriorityAndSize(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	featureRepo := &fakeFeatureRepo{}
	featureRepo.features = append(featureRepo.features, domain.Feature{
		ID: "FEAT-001", EpicID: "EPIC-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewFeatureService(fs, featureRepo, &fakeEpicRepo{}, histRepo)

	p := domain.PriorityCritical
	sz := domain.Size13
	feature, err := svc.SetFeature(context.Background(), driving.SetFeatureRequest{
		ID: "FEAT-001", Priority: &p, Size: &sz,
	})
	if err != nil {
		t.Fatalf("SetFeature returned error: %v", err)
	}
	if feature.Priority != domain.PriorityCritical {
		t.Errorf("feature.Priority = %q, want critical", feature.Priority)
	}
	if feature.Size != domain.Size13 {
		t.Errorf("feature.Size = %d, want 13", feature.Size)
	}
	if len(histRepo.entries) != 2 {
		t.Fatalf("expected 2 history entries (priority + size), got %d", len(histRepo.entries))
	}
}

func TestSetFeature_InvalidTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	featureRepo := &fakeFeatureRepo{}
	featureRepo.features = append(featureRepo.features, domain.Feature{
		ID: "FEAT-001", EpicID: "EPIC-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewFeatureService(fs, featureRepo, &fakeEpicRepo{}, &fakeHistoryRepo{})

	done := domain.StatusDone
	_, err := svc.SetFeature(context.Background(), driving.SetFeatureRequest{
		ID: "FEAT-001", Status: &done,
	})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("SetFeature err = %v, want ErrInvalidTransition", err)
	}
}

func TestSetFeature_NoFields(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewFeatureService(fs, &fakeFeatureRepo{}, &fakeEpicRepo{}, &fakeHistoryRepo{})

	_, err := svc.SetFeature(context.Background(), driving.SetFeatureRequest{ID: "FEAT-001"})
	if !errors.Is(err, service.ErrNoFieldsToSet) {
		t.Fatalf("SetFeature err = %v, want ErrNoFieldsToSet", err)
	}
}
