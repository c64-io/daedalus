package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
	"github.com/c64-io/daedalus/internal/core/service"
)

// fakeEpicRepo is an in-memory EpicRepository for testing.
type fakeEpicRepo struct {
	seq       int
	epics     []domain.Epic
	nextErr   error
	saveErr   error
	getErr    error
	listErr   error
	updateErr error
}

func (r *fakeEpicRepo) NextEpicSeq(_ context.Context, _ string) (int, error) {
	if r.nextErr != nil {
		return 0, r.nextErr
	}
	r.seq++
	return r.seq, nil
}

func (r *fakeEpicRepo) SaveEpic(_ context.Context, _ string, epic domain.Epic) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.epics = append(r.epics, epic)
	return nil
}

func (r *fakeEpicRepo) GetEpic(_ context.Context, _ string, id string) (*domain.Epic, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	for i := range r.epics {
		if r.epics[i].ID == id {
			return &r.epics[i], nil
		}
	}
	return nil, port.ErrEpicNotFound
}

func (r *fakeEpicRepo) ListEpics(_ context.Context, _ string, ideaID string) ([]domain.Epic, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	if ideaID == "" {
		return r.epics, nil
	}
	var filtered []domain.Epic
	for _, e := range r.epics {
		if e.IdeaID == ideaID {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

func (r *fakeEpicRepo) UpdateEpic(_ context.Context, _ string, epic domain.Epic) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	for i := range r.epics {
		if r.epics[i].ID == epic.ID {
			r.epics[i] = epic
			return nil
		}
	}
	return port.ErrEpicNotFound
}

// seedRefinedIdea adds a refined idea to the fake idea repo so epic
// creation tests have a valid parent.
func seedRefinedIdea(repo *fakeIdeaRepo, id string) {
	repo.ideas = append(repo.ideas, domain.Idea{
		ID:        id,
		Title:     "Test Idea",
		Status:    domain.StatusRefined,
		CreatedAt: time.Now(),
	})
}

func TestCreateEpic_Success(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	ideaRepo := &fakeIdeaRepo{}
	seedRefinedIdea(ideaRepo, "IDEA-001")
	epicRepo := &fakeEpicRepo{}
	svc := service.NewEpicService(fs, epicRepo, ideaRepo, &fakeHistoryRepo{})

	epic, err := svc.CreateEpic(context.Background(), port.CreateEpicRequest{
		IdeaID:      "IDEA-001",
		Title:       "Billing",
		Description: "Payment processing",
		Priority:    domain.PriorityHigh,
		Size:        domain.Size5,
	})
	if err != nil {
		t.Fatalf("CreateEpic returned unexpected error: %v", err)
	}

	if epic.ID != "EPIC-001" {
		t.Errorf("epic.ID = %q, want %q", epic.ID, "EPIC-001")
	}
	if epic.IdeaID != "IDEA-001" {
		t.Errorf("epic.IdeaID = %q, want %q", epic.IdeaID, "IDEA-001")
	}
	if epic.Title != "Billing" {
		t.Errorf("epic.Title = %q, want %q", epic.Title, "Billing")
	}
	if epic.Status != domain.StatusDraft {
		t.Errorf("epic.Status = %q, want %q", epic.Status, domain.StatusDraft)
	}
	if epic.Priority != domain.PriorityHigh {
		t.Errorf("epic.Priority = %q, want %q", epic.Priority, domain.PriorityHigh)
	}
	if epic.Size != domain.Size5 {
		t.Errorf("epic.Size = %d, want %d", epic.Size, domain.Size5)
	}
	if len(epicRepo.epics) != 1 {
		t.Fatalf("expected 1 saved epic, got %d", len(epicRepo.epics))
	}
}

func TestCreateEpic_MonotonicIDs(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	ideaRepo := &fakeIdeaRepo{}
	seedRefinedIdea(ideaRepo, "IDEA-001")
	epicRepo := &fakeEpicRepo{}
	svc := service.NewEpicService(fs, epicRepo, ideaRepo, &fakeHistoryRepo{})

	for i := 1; i <= 3; i++ {
		epic, err := svc.CreateEpic(context.Background(), port.CreateEpicRequest{
			IdeaID: "IDEA-001",
			Title:  "epic",
		})
		if err != nil {
			t.Fatalf("CreateEpic #%d returned error: %v", i, err)
		}
		want := domain.FormatEpicID(i)
		if epic.ID != want {
			t.Errorf("epic #%d ID = %q, want %q", i, epic.ID, want)
		}
	}
}

func TestCreateEpic_TitleRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewEpicService(fs, &fakeEpicRepo{}, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateEpic(context.Background(), port.CreateEpicRequest{
		IdeaID: "IDEA-001",
	})
	if !errors.Is(err, service.ErrEpicTitleRequired) {
		t.Fatalf("CreateEpic err = %v, want ErrEpicTitleRequired", err)
	}
}

func TestCreateEpic_IdeaRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewEpicService(fs, &fakeEpicRepo{}, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateEpic(context.Background(), port.CreateEpicRequest{
		Title: "Billing",
	})
	if !errors.Is(err, service.ErrEpicIdeaRequired) {
		t.Fatalf("CreateEpic err = %v, want ErrEpicIdeaRequired", err)
	}
}

func TestCreateEpic_IdeaNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewEpicService(fs, &fakeEpicRepo{}, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateEpic(context.Background(), port.CreateEpicRequest{
		IdeaID: "IDEA-999",
		Title:  "Billing",
	})
	if !errors.Is(err, port.ErrIdeaNotFound) {
		t.Fatalf("CreateEpic err = %v, want ErrIdeaNotFound", err)
	}
}

func TestCreateEpic_IdeaNotRefined(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	ideaRepo := &fakeIdeaRepo{}
	ideaRepo.ideas = append(ideaRepo.ideas, domain.Idea{
		ID: "IDEA-001", Title: "Draft Idea", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewEpicService(fs, &fakeEpicRepo{}, ideaRepo, &fakeHistoryRepo{})

	_, err := svc.CreateEpic(context.Background(), port.CreateEpicRequest{
		IdeaID: "IDEA-001",
		Title:  "Billing",
	})
	if !errors.Is(err, service.ErrIdeaNotReady) {
		t.Fatalf("CreateEpic err = %v, want ErrIdeaNotReady", err)
	}
}

func TestCreateEpic_IdeaArchived(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	ideaRepo := &fakeIdeaRepo{}
	ideaRepo.ideas = append(ideaRepo.ideas, domain.Idea{
		ID: "IDEA-001", Title: "Archived Idea", Status: domain.StatusArchived, CreatedAt: time.Now(),
	})
	svc := service.NewEpicService(fs, &fakeEpicRepo{}, ideaRepo, &fakeHistoryRepo{})

	_, err := svc.CreateEpic(context.Background(), port.CreateEpicRequest{
		IdeaID: "IDEA-001",
		Title:  "Billing",
	})
	if !errors.Is(err, service.ErrIdeaNotReady) {
		t.Fatalf("CreateEpic err = %v, want ErrIdeaNotReady", err)
	}
}

func TestCreateEpic_WorkspaceNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/empty")
	svc := service.NewEpicService(fs, &fakeEpicRepo{}, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateEpic(context.Background(), port.CreateEpicRequest{
		IdeaID: "IDEA-001",
		Title:  "x",
	})
	if !errors.Is(err, service.ErrWorkspaceNotFound) {
		t.Fatalf("CreateEpic err = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestCreateEpic_OptionalPrioritySize(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	ideaRepo := &fakeIdeaRepo{}
	seedRefinedIdea(ideaRepo, "IDEA-001")
	epicRepo := &fakeEpicRepo{}
	svc := service.NewEpicService(fs, epicRepo, ideaRepo, &fakeHistoryRepo{})

	epic, err := svc.CreateEpic(context.Background(), port.CreateEpicRequest{
		IdeaID: "IDEA-001",
		Title:  "No priority or size",
	})
	if err != nil {
		t.Fatalf("CreateEpic returned error: %v", err)
	}
	if epic.Priority != "" {
		t.Errorf("expected empty priority, got %q", epic.Priority)
	}
	if epic.Size != 0 {
		t.Errorf("expected zero size, got %d", epic.Size)
	}
}

func TestListEpics_FilterByIdea(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	ideaRepo := &fakeIdeaRepo{}
	seedRefinedIdea(ideaRepo, "IDEA-001")
	seedRefinedIdea(ideaRepo, "IDEA-002")
	epicRepo := &fakeEpicRepo{}
	svc := service.NewEpicService(fs, epicRepo, ideaRepo, &fakeHistoryRepo{})

	_, _ = svc.CreateEpic(context.Background(), port.CreateEpicRequest{IdeaID: "IDEA-001", Title: "A"})
	_, _ = svc.CreateEpic(context.Background(), port.CreateEpicRequest{IdeaID: "IDEA-002", Title: "B"})
	_, _ = svc.CreateEpic(context.Background(), port.CreateEpicRequest{IdeaID: "IDEA-001", Title: "C"})

	all, err := svc.ListEpics(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListEpics (all) returned error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 epics, got %d", len(all))
	}

	filtered, err := svc.ListEpics(context.Background(), "", "IDEA-001")
	if err != nil {
		t.Fatalf("ListEpics (filtered) returned error: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 epics for IDEA-001, got %d", len(filtered))
	}
}

func TestGetEpic_NotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewEpicService(fs, &fakeEpicRepo{}, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	_, err := svc.GetEpic(context.Background(), "", "EPIC-999")
	if !errors.Is(err, port.ErrEpicNotFound) {
		t.Fatalf("GetEpic err = %v, want ErrEpicNotFound", err)
	}
}

func TestSetEpic_StatusTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	epicRepo := &fakeEpicRepo{}
	epicRepo.epics = append(epicRepo.epics, domain.Epic{
		ID: "EPIC-001", IdeaID: "IDEA-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewEpicService(fs, epicRepo, &fakeIdeaRepo{}, histRepo)

	refined := domain.StatusRefined
	epic, err := svc.SetEpic(context.Background(), port.SetEpicRequest{
		ID: "EPIC-001", Status: &refined,
	})
	if err != nil {
		t.Fatalf("SetEpic returned error: %v", err)
	}
	if epic.Status != domain.StatusRefined {
		t.Errorf("epic.Status = %q, want refined", epic.Status)
	}
	if len(histRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(histRepo.entries))
	}
}

func TestSetEpic_PriorityAndSize(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	epicRepo := &fakeEpicRepo{}
	epicRepo.epics = append(epicRepo.epics, domain.Epic{
		ID: "EPIC-001", IdeaID: "IDEA-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewEpicService(fs, epicRepo, &fakeIdeaRepo{}, histRepo)

	p := domain.PriorityCritical
	sz := domain.Size13
	epic, err := svc.SetEpic(context.Background(), port.SetEpicRequest{
		ID: "EPIC-001", Priority: &p, Size: &sz,
	})
	if err != nil {
		t.Fatalf("SetEpic returned error: %v", err)
	}
	if epic.Priority != domain.PriorityCritical {
		t.Errorf("epic.Priority = %q, want critical", epic.Priority)
	}
	if epic.Size != domain.Size13 {
		t.Errorf("epic.Size = %d, want 13", epic.Size)
	}
	if len(histRepo.entries) != 2 {
		t.Fatalf("expected 2 history entries (priority + size), got %d", len(histRepo.entries))
	}
}

func TestSetEpic_InvalidTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	epicRepo := &fakeEpicRepo{}
	epicRepo.epics = append(epicRepo.epics, domain.Epic{
		ID: "EPIC-001", IdeaID: "IDEA-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewEpicService(fs, epicRepo, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	done := domain.StatusDone
	_, err := svc.SetEpic(context.Background(), port.SetEpicRequest{
		ID: "EPIC-001", Status: &done,
	})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("SetEpic err = %v, want ErrInvalidTransition", err)
	}
}

func TestSetEpic_NoFields(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewEpicService(fs, &fakeEpicRepo{}, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	_, err := svc.SetEpic(context.Background(), port.SetEpicRequest{ID: "EPIC-001"})
	if !errors.Is(err, service.ErrNoFieldsToSet) {
		t.Fatalf("SetEpic err = %v, want ErrNoFieldsToSet", err)
	}
}
