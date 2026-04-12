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

// fakeStoryRepo is an in-memory StoryRepository for testing.
type fakeStoryRepo struct {
	seq       int
	stories   []domain.Story
	nextErr   error
	saveErr   error
	getErr    error
	listErr   error
	updateErr error
}

func (r *fakeStoryRepo) NextStorySeq(_ context.Context, _ string) (int, error) {
	if r.nextErr != nil {
		return 0, r.nextErr
	}
	r.seq++
	return r.seq, nil
}

func (r *fakeStoryRepo) SaveStory(_ context.Context, _ string, story domain.Story) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.stories = append(r.stories, story)
	return nil
}

func (r *fakeStoryRepo) GetStory(_ context.Context, _ string, id string) (*domain.Story, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	for i := range r.stories {
		if r.stories[i].ID == id {
			return &r.stories[i], nil
		}
	}
	return nil, driven.ErrStoryNotFound
}

func (r *fakeStoryRepo) ListStories(_ context.Context, _ string, featureID string) ([]domain.Story, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	if featureID == "" {
		return r.stories, nil
	}
	var filtered []domain.Story
	for _, s := range r.stories {
		if s.FeatureID == featureID {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func (r *fakeStoryRepo) UpdateStory(_ context.Context, _ string, story domain.Story) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	for i := range r.stories {
		if r.stories[i].ID == story.ID {
			r.stories[i] = story
			return nil
		}
	}
	return driven.ErrStoryNotFound
}

// seedRefinedFeature adds a refined feature to the fake feature repo
// so story creation tests have a valid parent.
func seedRefinedFeature(repo *fakeFeatureRepo, id string) {
	repo.features = append(repo.features, domain.Feature{
		ID:        id,
		EpicID:    "EPIC-001",
		Title:     "Test Feature",
		Status:    domain.StatusRefined,
		CreatedAt: time.Now(),
	})
}

func TestCreateStory_Success(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	featureRepo := &fakeFeatureRepo{}
	seedRefinedFeature(featureRepo, "FEAT-001")
	storyRepo := &fakeStoryRepo{}
	svc := service.NewStoryService(fs, storyRepo, featureRepo, &fakeHistoryRepo{})

	story, err := svc.CreateStory(context.Background(), driving.CreateStoryRequest{
		FeatureID:   "FEAT-001",
		Title:       "User can log in",
		Description: "OAuth login flow",
		Priority:    domain.PriorityHigh,
		Size:        domain.Size3,
	})
	if err != nil {
		t.Fatalf("CreateStory returned unexpected error: %v", err)
	}

	if story.ID != "STORY-001" {
		t.Errorf("story.ID = %q, want %q", story.ID, "STORY-001")
	}
	if story.FeatureID != "FEAT-001" {
		t.Errorf("story.FeatureID = %q, want %q", story.FeatureID, "FEAT-001")
	}
	if story.Title != "User can log in" {
		t.Errorf("story.Title = %q, want %q", story.Title, "User can log in")
	}
	if story.Status != domain.StatusDraft {
		t.Errorf("story.Status = %q, want %q", story.Status, domain.StatusDraft)
	}
	if story.Priority != domain.PriorityHigh {
		t.Errorf("story.Priority = %q, want %q", story.Priority, domain.PriorityHigh)
	}
	if story.Size != domain.Size3 {
		t.Errorf("story.Size = %d, want %d", story.Size, domain.Size3)
	}
	if len(storyRepo.stories) != 1 {
		t.Fatalf("expected 1 saved story, got %d", len(storyRepo.stories))
	}
}

func TestCreateStory_MonotonicIDs(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	featureRepo := &fakeFeatureRepo{}
	seedRefinedFeature(featureRepo, "FEAT-001")
	storyRepo := &fakeStoryRepo{}
	svc := service.NewStoryService(fs, storyRepo, featureRepo, &fakeHistoryRepo{})

	for i := 1; i <= 3; i++ {
		story, err := svc.CreateStory(context.Background(), driving.CreateStoryRequest{
			FeatureID: "FEAT-001",
			Title:     "story",
		})
		if err != nil {
			t.Fatalf("CreateStory #%d returned error: %v", i, err)
		}
		want := domain.FormatStoryID(i)
		if story.ID != want {
			t.Errorf("story #%d ID = %q, want %q", i, story.ID, want)
		}
	}
}

func TestCreateStory_TitleRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewStoryService(fs, &fakeStoryRepo{}, &fakeFeatureRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateStory(context.Background(), driving.CreateStoryRequest{
		FeatureID: "FEAT-001",
	})
	if !errors.Is(err, service.ErrStoryTitleRequired) {
		t.Fatalf("CreateStory err = %v, want ErrStoryTitleRequired", err)
	}
}

func TestCreateStory_FeatureRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewStoryService(fs, &fakeStoryRepo{}, &fakeFeatureRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateStory(context.Background(), driving.CreateStoryRequest{
		Title: "Login",
	})
	if !errors.Is(err, service.ErrStoryFeatureRequired) {
		t.Fatalf("CreateStory err = %v, want ErrStoryFeatureRequired", err)
	}
}

func TestCreateStory_FeatureNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewStoryService(fs, &fakeStoryRepo{}, &fakeFeatureRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateStory(context.Background(), driving.CreateStoryRequest{
		FeatureID: "FEAT-999",
		Title:     "Login",
	})
	if !errors.Is(err, driven.ErrFeatureNotFound) {
		t.Fatalf("CreateStory err = %v, want ErrFeatureNotFound", err)
	}
}

func TestCreateStory_FeatureNotRefined(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	featureRepo := &fakeFeatureRepo{}
	featureRepo.features = append(featureRepo.features, domain.Feature{
		ID: "FEAT-001", EpicID: "EPIC-001", Title: "Draft Feature", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewStoryService(fs, &fakeStoryRepo{}, featureRepo, &fakeHistoryRepo{})

	_, err := svc.CreateStory(context.Background(), driving.CreateStoryRequest{
		FeatureID: "FEAT-001",
		Title:     "Login",
	})
	if !errors.Is(err, service.ErrFeatureNotReady) {
		t.Fatalf("CreateStory err = %v, want ErrFeatureNotReady", err)
	}
}

func TestCreateStory_FeatureArchived(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	featureRepo := &fakeFeatureRepo{}
	featureRepo.features = append(featureRepo.features, domain.Feature{
		ID: "FEAT-001", EpicID: "EPIC-001", Title: "Archived Feature", Status: domain.StatusArchived, CreatedAt: time.Now(),
	})
	svc := service.NewStoryService(fs, &fakeStoryRepo{}, featureRepo, &fakeHistoryRepo{})

	_, err := svc.CreateStory(context.Background(), driving.CreateStoryRequest{
		FeatureID: "FEAT-001",
		Title:     "Login",
	})
	if !errors.Is(err, service.ErrFeatureNotReady) {
		t.Fatalf("CreateStory err = %v, want ErrFeatureNotReady", err)
	}
}

func TestCreateStory_WorkspaceNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/empty")
	svc := service.NewStoryService(fs, &fakeStoryRepo{}, &fakeFeatureRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateStory(context.Background(), driving.CreateStoryRequest{
		FeatureID: "FEAT-001",
		Title:     "x",
	})
	if !errors.Is(err, service.ErrWorkspaceNotFound) {
		t.Fatalf("CreateStory err = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestCreateStory_OptionalPrioritySize(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	featureRepo := &fakeFeatureRepo{}
	seedRefinedFeature(featureRepo, "FEAT-001")
	storyRepo := &fakeStoryRepo{}
	svc := service.NewStoryService(fs, storyRepo, featureRepo, &fakeHistoryRepo{})

	story, err := svc.CreateStory(context.Background(), driving.CreateStoryRequest{
		FeatureID: "FEAT-001",
		Title:     "No priority or size",
	})
	if err != nil {
		t.Fatalf("CreateStory returned error: %v", err)
	}
	if story.Priority != "" {
		t.Errorf("expected empty priority, got %q", story.Priority)
	}
	if story.Size != 0 {
		t.Errorf("expected zero size, got %d", story.Size)
	}
}

func TestListStories_FilterByFeature(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	featureRepo := &fakeFeatureRepo{}
	seedRefinedFeature(featureRepo, "FEAT-001")
	seedRefinedFeature(featureRepo, "FEAT-002")
	storyRepo := &fakeStoryRepo{}
	svc := service.NewStoryService(fs, storyRepo, featureRepo, &fakeHistoryRepo{})

	_, _ = svc.CreateStory(context.Background(), driving.CreateStoryRequest{FeatureID: "FEAT-001", Title: "A"})
	_, _ = svc.CreateStory(context.Background(), driving.CreateStoryRequest{FeatureID: "FEAT-002", Title: "B"})
	_, _ = svc.CreateStory(context.Background(), driving.CreateStoryRequest{FeatureID: "FEAT-001", Title: "C"})

	all, err := svc.ListStories(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListStories (all) returned error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 stories, got %d", len(all))
	}

	filtered, err := svc.ListStories(context.Background(), "", "FEAT-001")
	if err != nil {
		t.Fatalf("ListStories (filtered) returned error: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 stories for FEAT-001, got %d", len(filtered))
	}
}

func TestGetStory_NotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewStoryService(fs, &fakeStoryRepo{}, &fakeFeatureRepo{}, &fakeHistoryRepo{})

	_, err := svc.GetStory(context.Background(), "", "STORY-999")
	if !errors.Is(err, driven.ErrStoryNotFound) {
		t.Fatalf("GetStory err = %v, want ErrStoryNotFound", err)
	}
}

func TestSetStory_StatusTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	storyRepo := &fakeStoryRepo{}
	storyRepo.stories = append(storyRepo.stories, domain.Story{
		ID: "STORY-001", FeatureID: "FEAT-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewStoryService(fs, storyRepo, &fakeFeatureRepo{}, histRepo)

	refined := domain.StatusRefined
	story, err := svc.SetStory(context.Background(), driving.SetStoryRequest{
		ID: "STORY-001", Status: &refined,
	})
	if err != nil {
		t.Fatalf("SetStory returned error: %v", err)
	}
	if story.Status != domain.StatusRefined {
		t.Errorf("story.Status = %q, want refined", story.Status)
	}
	if len(histRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(histRepo.entries))
	}
}

func TestSetStory_PriorityAndSize(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	storyRepo := &fakeStoryRepo{}
	storyRepo.stories = append(storyRepo.stories, domain.Story{
		ID: "STORY-001", FeatureID: "FEAT-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewStoryService(fs, storyRepo, &fakeFeatureRepo{}, histRepo)

	p := domain.PriorityCritical
	sz := domain.Size13
	story, err := svc.SetStory(context.Background(), driving.SetStoryRequest{
		ID: "STORY-001", Priority: &p, Size: &sz,
	})
	if err != nil {
		t.Fatalf("SetStory returned error: %v", err)
	}
	if story.Priority != domain.PriorityCritical {
		t.Errorf("story.Priority = %q, want critical", story.Priority)
	}
	if story.Size != domain.Size13 {
		t.Errorf("story.Size = %d, want 13", story.Size)
	}
	if len(histRepo.entries) != 2 {
		t.Fatalf("expected 2 history entries (priority + size), got %d", len(histRepo.entries))
	}
}

func TestSetStory_InvalidTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	storyRepo := &fakeStoryRepo{}
	storyRepo.stories = append(storyRepo.stories, domain.Story{
		ID: "STORY-001", FeatureID: "FEAT-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewStoryService(fs, storyRepo, &fakeFeatureRepo{}, &fakeHistoryRepo{})

	done := domain.StatusDone
	_, err := svc.SetStory(context.Background(), driving.SetStoryRequest{
		ID: "STORY-001", Status: &done,
	})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("SetStory err = %v, want ErrInvalidTransition", err)
	}
}

func TestSetStory_NoFields(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewStoryService(fs, &fakeStoryRepo{}, &fakeFeatureRepo{}, &fakeHistoryRepo{})

	_, err := svc.SetStory(context.Background(), driving.SetStoryRequest{ID: "STORY-001"})
	if !errors.Is(err, service.ErrNoFieldsToSet) {
		t.Fatalf("SetStory err = %v, want ErrNoFieldsToSet", err)
	}
}
