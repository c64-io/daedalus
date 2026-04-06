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

// fakeIdeaRepo is an in-memory IdeaRepository that tracks calls and
// stores ideas for round-trip testing.
type fakeIdeaRepo struct {
	seq       int
	ideas     []domain.Idea
	nextErr   error
	saveErr   error
	getErr    error
	listErr   error
	updateErr error
}

func (r *fakeIdeaRepo) NextIdeaSeq(_ context.Context, _ string) (int, error) {
	if r.nextErr != nil {
		return 0, r.nextErr
	}
	r.seq++
	return r.seq, nil
}

func (r *fakeIdeaRepo) SaveIdea(_ context.Context, _ string, idea domain.Idea) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.ideas = append(r.ideas, idea)
	return nil
}

func (r *fakeIdeaRepo) GetIdea(_ context.Context, _ string, id string) (*domain.Idea, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	for i := range r.ideas {
		if r.ideas[i].ID == id {
			return &r.ideas[i], nil
		}
	}
	return nil, port.ErrIdeaNotFound
}

func (r *fakeIdeaRepo) ListIdeas(_ context.Context, _ string) ([]domain.Idea, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.ideas, nil
}

func (r *fakeIdeaRepo) UpdateIdea(_ context.Context, _ string, idea domain.Idea) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	for i := range r.ideas {
		if r.ideas[i].ID == idea.ID {
			r.ideas[i] = idea
			return nil
		}
	}
	return port.ErrIdeaNotFound
}

// fakeHistoryRepo records appended history entries for assertions.
type fakeHistoryRepo struct {
	entries   []domain.HistoryEntry
	appendErr error
}

func (r *fakeHistoryRepo) AppendHistory(_ context.Context, _ string, entries []domain.HistoryEntry) error {
	if r.appendErr != nil {
		return r.appendErr
	}
	r.entries = append(r.entries, entries...)
	return nil
}

func TestCreateIdea_Success(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	repo := &fakeIdeaRepo{}
	svc := service.NewIdeaService(fs, repo, &fakeHistoryRepo{})

	idea, err := svc.CreateIdea(context.Background(), port.CreateIdeaRequest{
		Title:       "My SaaS",
		Description: "A subscription platform",
	})
	if err != nil {
		t.Fatalf("CreateIdea returned unexpected error: %v", err)
	}

	if idea.ID != "IDEA-001" {
		t.Errorf("idea.ID = %q, want %q", idea.ID, "IDEA-001")
	}
	if idea.Title != "My SaaS" {
		t.Errorf("idea.Title = %q, want %q", idea.Title, "My SaaS")
	}
	if idea.Status != domain.StatusDraft {
		t.Errorf("idea.Status = %q, want %q", idea.Status, domain.StatusDraft)
	}
	if len(repo.ideas) != 1 {
		t.Fatalf("expected 1 saved idea, got %d", len(repo.ideas))
	}
}

func TestCreateIdea_MonotonicIDs(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	repo := &fakeIdeaRepo{}
	svc := service.NewIdeaService(fs, repo, &fakeHistoryRepo{})

	for i := 1; i <= 3; i++ {
		idea, err := svc.CreateIdea(context.Background(), port.CreateIdeaRequest{
			Title: "idea",
		})
		if err != nil {
			t.Fatalf("CreateIdea #%d returned error: %v", i, err)
		}
		want := domain.FormatIdeaID(i)
		if idea.ID != want {
			t.Errorf("idea #%d ID = %q, want %q", i, idea.ID, want)
		}
	}
}

func TestCreateIdea_TitleRequired(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewIdeaService(fs, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateIdea(context.Background(), port.CreateIdeaRequest{})
	if !errors.Is(err, service.ErrIdeaTitleRequired) {
		t.Fatalf("CreateIdea err = %v, want ErrIdeaTitleRequired", err)
	}
}

func TestCreateIdea_WorkspaceNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/empty")
	svc := service.NewIdeaService(fs, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	_, err := svc.CreateIdea(context.Background(), port.CreateIdeaRequest{Title: "x"})
	if !errors.Is(err, service.ErrWorkspaceNotFound) {
		t.Fatalf("CreateIdea err = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestListIdeas_Empty(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewIdeaService(fs, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	ideas, err := svc.ListIdeas(context.Background(), "")
	if err != nil {
		t.Fatalf("ListIdeas returned error: %v", err)
	}
	if len(ideas) != 0 {
		t.Errorf("expected 0 ideas, got %d", len(ideas))
	}
}

func TestGetIdea_NotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewIdeaService(fs, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	_, err := svc.GetIdea(context.Background(), "", "IDEA-999")
	if !errors.Is(err, port.ErrIdeaNotFound) {
		t.Fatalf("GetIdea err = %v, want ErrIdeaNotFound", err)
	}
}

func TestSetIdea_StatusTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	ideaRepo := &fakeIdeaRepo{}
	ideaRepo.ideas = append(ideaRepo.ideas, domain.Idea{
		ID: "IDEA-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewIdeaService(fs, ideaRepo, histRepo)

	refined := domain.StatusRefined
	idea, err := svc.SetIdea(context.Background(), port.SetIdeaRequest{
		ID: "IDEA-001", Status: &refined,
	})
	if err != nil {
		t.Fatalf("SetIdea returned error: %v", err)
	}
	if idea.Status != domain.StatusRefined {
		t.Errorf("idea.Status = %q, want refined", idea.Status)
	}
	if len(histRepo.entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(histRepo.entries))
	}
	if histRepo.entries[0].Field != "status" {
		t.Errorf("history field = %q, want status", histRepo.entries[0].Field)
	}
}

func TestSetIdea_InvalidTransition(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	ideaRepo := &fakeIdeaRepo{}
	ideaRepo.ideas = append(ideaRepo.ideas, domain.Idea{
		ID: "IDEA-001", Title: "Test", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	svc := service.NewIdeaService(fs, ideaRepo, &fakeHistoryRepo{})

	done := domain.StatusDone
	_, err := svc.SetIdea(context.Background(), port.SetIdeaRequest{
		ID: "IDEA-001", Status: &done,
	})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("SetIdea err = %v, want ErrInvalidTransition", err)
	}
}

func TestSetIdea_MultipleFields(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	ideaRepo := &fakeIdeaRepo{}
	ideaRepo.ideas = append(ideaRepo.ideas, domain.Idea{
		ID: "IDEA-001", Title: "Old", Description: "old desc", Status: domain.StatusDraft, CreatedAt: time.Now(),
	})
	histRepo := &fakeHistoryRepo{}
	svc := service.NewIdeaService(fs, ideaRepo, histRepo)

	newTitle := "New Title"
	newDesc := "new desc"
	idea, err := svc.SetIdea(context.Background(), port.SetIdeaRequest{
		ID: "IDEA-001", Title: &newTitle, Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("SetIdea returned error: %v", err)
	}
	if idea.Title != "New Title" {
		t.Errorf("idea.Title = %q, want %q", idea.Title, "New Title")
	}
	if idea.Description != "new desc" {
		t.Errorf("idea.Description = %q, want %q", idea.Description, "new desc")
	}
	if len(histRepo.entries) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(histRepo.entries))
	}
}

func TestSetIdea_NoFields(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewIdeaService(fs, &fakeIdeaRepo{}, &fakeHistoryRepo{})

	_, err := svc.SetIdea(context.Background(), port.SetIdeaRequest{ID: "IDEA-001"})
	if !errors.Is(err, service.ErrNoFieldsToSet) {
		t.Fatalf("SetIdea err = %v, want ErrNoFieldsToSet", err)
	}
}

func TestSetIdea_Blocked(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	ideaRepo := &fakeIdeaRepo{}
	ideaRepo.ideas = append(ideaRepo.ideas, domain.Idea{
		ID: "IDEA-001", Title: "Test", Status: domain.StatusRefined, CreatedAt: time.Now(),
	})
	svc := service.NewIdeaService(fs, ideaRepo, &fakeHistoryRepo{})

	blocked := domain.StatusBlocked
	idea, err := svc.SetIdea(context.Background(), port.SetIdeaRequest{
		ID: "IDEA-001", Status: &blocked,
	})
	if err != nil {
		t.Fatalf("SetIdea returned error: %v", err)
	}
	if idea.Status != domain.StatusBlocked {
		t.Errorf("idea.Status = %q, want blocked", idea.Status)
	}
}
