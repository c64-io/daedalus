package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
	"github.com/c64-io/daedalus/internal/core/service"
)

// fakeIdeaRepo is an in-memory IdeaRepository that tracks calls and
// stores ideas for round-trip testing.
type fakeIdeaRepo struct {
	seq     int
	ideas   []domain.Idea
	nextErr error
	saveErr error
	getErr  error
	listErr error
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

func TestCreateIdea_Success(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	repo := &fakeIdeaRepo{}
	svc := service.NewIdeaService(fs, repo)

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
	svc := service.NewIdeaService(fs, repo)

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
	svc := service.NewIdeaService(fs, &fakeIdeaRepo{})

	_, err := svc.CreateIdea(context.Background(), port.CreateIdeaRequest{})
	if !errors.Is(err, service.ErrIdeaTitleRequired) {
		t.Fatalf("CreateIdea err = %v, want ErrIdeaTitleRequired", err)
	}
}

func TestCreateIdea_WorkspaceNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/empty")
	svc := service.NewIdeaService(fs, &fakeIdeaRepo{})

	_, err := svc.CreateIdea(context.Background(), port.CreateIdeaRequest{Title: "x"})
	if !errors.Is(err, service.ErrWorkspaceNotFound) {
		t.Fatalf("CreateIdea err = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestListIdeas_Empty(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/proj")
	fs.existing["/proj/d7/.db"] = true
	svc := service.NewIdeaService(fs, &fakeIdeaRepo{})

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
	svc := service.NewIdeaService(fs, &fakeIdeaRepo{})

	_, err := svc.GetIdea(context.Background(), "", "IDEA-999")
	if !errors.Is(err, port.ErrIdeaNotFound) {
		t.Fatalf("GetIdea err = %v, want ErrIdeaNotFound", err)
	}
}
