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

// fakeSpecRepo is an in-memory SpecRepository for testing.
type fakeSpecRepo struct {
	seq       int
	specs     []domain.Spec
	nextErr   error
	saveErr   error
	getErr    error
	listErr   error
	updateErr error
}

func (r *fakeSpecRepo) NextSpecSeq(_ context.Context, _ string) (int, error) {
	if r.nextErr != nil {
		return 0, r.nextErr
	}
	r.seq++
	return r.seq, nil
}

func (r *fakeSpecRepo) SaveSpec(_ context.Context, _ string, spec domain.Spec) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.specs = append(r.specs, spec)
	return nil
}

func (r *fakeSpecRepo) GetSpec(_ context.Context, _ string, id string) (*domain.Spec, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	for i := range r.specs {
		if r.specs[i].ID == id {
			return &r.specs[i], nil
		}
	}
	return nil, driven.ErrSpecNotFound
}

func (r *fakeSpecRepo) ListSpecs(_ context.Context, _ string, storyID string) ([]domain.Spec, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	if storyID == "" {
		return r.specs, nil
	}
	var filtered []domain.Spec
	for _, s := range r.specs {
		if s.StoryID == storyID {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func (r *fakeSpecRepo) UpdateSpec(_ context.Context, _ string, spec domain.Spec) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	for i := range r.specs {
		if r.specs[i].ID == spec.ID {
			r.specs[i] = spec
			return nil
		}
	}
	return driven.ErrSpecNotFound
}

// seedRefinedStory adds a refined story to the fake story repo
// so spec creation tests have a valid parent.
func seedRefinedStory(repo *fakeStoryRepo, id string) {
	repo.stories = append(repo.stories, domain.Story{
		ID:        id,
		FeatureID: "FEAT-001",
		Title:     "Test Story",
		Status:    domain.StatusRefined,
		CreatedAt: time.Now(),
	})
}

func TestCreateSpec_Success(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	storyRepo := &fakeStoryRepo{}
	seedRefinedStory(storyRepo, "STORY-001")
	specRepo := &fakeSpecRepo{}
	svc := service.NewSpecService(fs, specRepo, storyRepo, &fakeHistoryRepo{})

	spec, err := svc.CreateSpec(context.Background(), driving.CreateSpecRequest{
		StoryID:     "STORY-001",
		Title:       "Coupon redemption rules",
		Description: "Coupons can only be redeemed once.",
	})
	if err != nil {
		t.Fatalf("CreateSpec returned unexpected error: %v", err)
	}

	if spec.ID != "SPEC-001" {
		t.Errorf("spec.ID = %q, want %q", spec.ID, "SPEC-001")
	}
	if spec.StoryID != "STORY-001" {
		t.Errorf("spec.StoryID = %q, want %q", spec.StoryID, "STORY-001")
	}
	if spec.Title != "Coupon redemption rules" {
		t.Errorf("spec.Title = %q, want %q", spec.Title, "Coupon redemption rules")
	}
	if spec.Status != domain.StatusDraft {
		t.Errorf("spec.Status = %q, want %q", spec.Status, domain.StatusDraft)
	}
	if len(specRepo.specs) != 1 {
		t.Fatalf("expected 1 saved spec, got %d", len(specRepo.specs))
	}
}

func TestCreateSpec_MonotonicIDs(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	storyRepo := &fakeStoryRepo{}
	seedRefinedStory(storyRepo, "STORY-001")
	specRepo := &fakeSpecRepo{}
	svc := service.NewSpecService(fs, specRepo, storyRepo, &fakeHistoryRepo{})

	for i := 1; i <= 3; i++ {
		spec, err := svc.CreateSpec(context.Background(), driving.CreateSpecRequest{
			StoryID: "STORY-001",
			Title:   "rule",
		})
		if err != nil {
			t.Fatalf("CreateSpec #%d returned error: %v", i, err)
		}
		want := domain.FormatSpecID(i)
		if spec.ID != want {
			t.Errorf("spec #%d ID = %q, want %q", i, spec.ID, want)
		}
	}
}

func TestCreateSpec_TitleRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewSpecService(fs, &fakeSpecRepo{}, &fakeStoryRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateSpec(context.Background(), driving.CreateSpecRequest{
		StoryID: "STORY-001",
	})
	if !errors.Is(err, service.ErrSpecTitleRequired) {
		t.Fatalf("CreateSpec err = %v, want ErrSpecTitleRequired", err)
	}
}

func TestCreateSpec_StoryRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewSpecService(fs, &fakeSpecRepo{}, &fakeStoryRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateSpec(context.Background(), driving.CreateSpecRequest{
		Title: "rule",
	})
	if !errors.Is(err, service.ErrSpecStoryRequired) {
		t.Fatalf("CreateSpec err = %v, want ErrSpecStoryRequired", err)
	}
}

func TestCreateSpec_StoryNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewSpecService(fs, &fakeSpecRepo{}, &fakeStoryRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateSpec(context.Background(), driving.CreateSpecRequest{
		StoryID: "STORY-999",
		Title:   "rule",
	})
	if !errors.Is(err, driven.ErrStoryNotFound) {
		t.Fatalf("CreateSpec err = %v, want ErrStoryNotFound", err)
	}
}

func TestCreateSpec_StoryNotRefined(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	storyRepo := &fakeStoryRepo{}
	storyRepo.stories = append(storyRepo.stories, domain.Story{
		ID: "STORY-001", FeatureID: "FEAT-001", Title: "Draft Story", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewSpecService(fs, &fakeSpecRepo{}, storyRepo, &fakeHistoryRepo{})

	_, err := svc.CreateSpec(context.Background(), driving.CreateSpecRequest{
		StoryID: "STORY-001",
		Title:   "rule",
	})
	if !errors.Is(err, service.ErrStoryNotReady) {
		t.Fatalf("CreateSpec err = %v, want ErrStoryNotReady", err)
	}
}

func TestCreateSpec_StoryArchived(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	storyRepo := &fakeStoryRepo{}
	storyRepo.stories = append(storyRepo.stories, domain.Story{
		ID: "STORY-001", FeatureID: "FEAT-001", Title: "Archived Story", Status: domain.StatusArchived, CreatedAt: time.Now(),
	})
	svc := service.NewSpecService(fs, &fakeSpecRepo{}, storyRepo, &fakeHistoryRepo{})

	_, err := svc.CreateSpec(context.Background(), driving.CreateSpecRequest{
		StoryID: "STORY-001",
		Title:   "rule",
	})
	if !errors.Is(err, service.ErrStoryNotReady) {
		t.Fatalf("CreateSpec err = %v, want ErrStoryNotReady", err)
	}
}

func TestCreateSpec_WorkspaceNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/empty")
	svc := service.NewSpecService(fs, &fakeSpecRepo{}, &fakeStoryRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateSpec(context.Background(), driving.CreateSpecRequest{
		StoryID: "STORY-001",
		Title:   "x",
	})
	if !errors.Is(err, service.ErrWorkspaceNotFound) {
		t.Fatalf("CreateSpec err = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestListSpecs_FilterByStory(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	storyRepo := &fakeStoryRepo{}
	seedRefinedStory(storyRepo, "STORY-001")
	seedRefinedStory(storyRepo, "STORY-002")
	specRepo := &fakeSpecRepo{}
	svc := service.NewSpecService(fs, specRepo, storyRepo, &fakeHistoryRepo{})

	_, _ = svc.CreateSpec(context.Background(), driving.CreateSpecRequest{StoryID: "STORY-001", Title: "A"})
	_, _ = svc.CreateSpec(context.Background(), driving.CreateSpecRequest{StoryID: "STORY-002", Title: "B"})
	_, _ = svc.CreateSpec(context.Background(), driving.CreateSpecRequest{StoryID: "STORY-001", Title: "C"})

	all, err := svc.ListSpecs(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListSpecs (all) returned error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 specs, got %d", len(all))
	}

	filtered, err := svc.ListSpecs(context.Background(), "", "STORY-001")
	if err != nil {
		t.Fatalf("ListSpecs (filtered) returned error: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 specs for STORY-001, got %d", len(filtered))
	}
}

func TestGetSpec_NotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewSpecService(fs, &fakeSpecRepo{}, &fakeStoryRepo{}, &fakeHistoryRepo{})

	_, err := svc.GetSpec(context.Background(), "", "SPEC-999")
	if !errors.Is(err, driven.ErrSpecNotFound) {
		t.Fatalf("GetSpec err = %v, want ErrSpecNotFound", err)
	}
}

func TestSetSpec_StatusTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	specRepo := &fakeSpecRepo{}
	specRepo.specs = append(specRepo.specs, domain.Spec{
		ID: "SPEC-001", StoryID: "STORY-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewSpecService(fs, specRepo, &fakeStoryRepo{}, histRepo)

	refined := domain.StatusRefined
	spec, err := svc.SetSpec(context.Background(), driving.SetSpecRequest{
		ID: "SPEC-001", Status: &refined,
	})
	if err != nil {
		t.Fatalf("SetSpec returned error: %v", err)
	}
	if spec.Status != domain.StatusRefined {
		t.Errorf("spec.Status = %q, want refined", spec.Status)
	}
	if len(histRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(histRepo.entries))
	}
}

func TestSetSpec_TitleAndDescription(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	specRepo := &fakeSpecRepo{}
	specRepo.specs = append(specRepo.specs, domain.Spec{
		ID: "SPEC-001", StoryID: "STORY-001", Title: "Old", Description: "old body", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewSpecService(fs, specRepo, &fakeStoryRepo{}, histRepo)

	newTitle := "New"
	newDesc := "new body"
	spec, err := svc.SetSpec(context.Background(), driving.SetSpecRequest{
		ID: "SPEC-001", Title: &newTitle, Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("SetSpec returned error: %v", err)
	}
	if spec.Title != "New" {
		t.Errorf("spec.Title = %q, want New", spec.Title)
	}
	if spec.Description != "new body" {
		t.Errorf("spec.Description = %q, want new body", spec.Description)
	}
	if len(histRepo.entries) != 2 {
		t.Fatalf("expected 2 history entries (title + description), got %d", len(histRepo.entries))
	}
}

func TestSetSpec_InvalidTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	specRepo := &fakeSpecRepo{}
	specRepo.specs = append(specRepo.specs, domain.Spec{
		ID: "SPEC-001", StoryID: "STORY-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewSpecService(fs, specRepo, &fakeStoryRepo{}, &fakeHistoryRepo{})

	done := domain.StatusDone
	_, err := svc.SetSpec(context.Background(), driving.SetSpecRequest{
		ID: "SPEC-001", Status: &done,
	})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("SetSpec err = %v, want ErrInvalidTransition", err)
	}
}

func TestSetSpec_NoFields(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewSpecService(fs, &fakeSpecRepo{}, &fakeStoryRepo{}, &fakeHistoryRepo{})

	_, err := svc.SetSpec(context.Background(), driving.SetSpecRequest{ID: "SPEC-001"})
	if !errors.Is(err, service.ErrNoFieldsToSet) {
		t.Fatalf("SetSpec err = %v, want ErrNoFieldsToSet", err)
	}
}
