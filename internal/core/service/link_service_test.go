package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
	"github.com/c64-io/daedalus/internal/core/port/driving"
	"github.com/c64-io/daedalus/internal/core/service"
)

// --- Fake LinkRepository ---

type fakeLinkRepo struct {
	links     []domain.Link
	saveErr   error
	deleteErr error
}

func (r *fakeLinkRepo) SaveLink(_ context.Context, _ string, link domain.Link) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.links = append(r.links, link)
	return nil
}

func (r *fakeLinkRepo) DeleteLink(_ context.Context, _ string, fromID string, kind domain.LinkKind, toID string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	for i, l := range r.links {
		if l.FromID == fromID && l.Kind == kind && l.ToID == toID {
			r.links = append(r.links[:i], r.links[i+1:]...)
			return nil
		}
	}
	return driven.ErrLinkNotFound
}

func (r *fakeLinkRepo) GetLink(_ context.Context, _ string, fromID string, kind domain.LinkKind, toID string) (*domain.Link, error) {
	for i, l := range r.links {
		if l.FromID == fromID && l.Kind == kind && l.ToID == toID {
			return &r.links[i], nil
		}
	}
	return nil, nil
}

func (r *fakeLinkRepo) ListLinksByEntity(_ context.Context, _ string, entityID string) ([]domain.Link, error) {
	if entityID == "" {
		return r.links, nil
	}
	var out []domain.Link
	for _, l := range r.links {
		if l.FromID == entityID || l.ToID == entityID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (r *fakeLinkRepo) ListAllLinksByKind(_ context.Context, _ string, kind domain.LinkKind) ([]domain.Link, error) {
	var out []domain.Link
	for _, l := range r.links {
		if l.Kind == kind {
			out = append(out, l)
		}
	}
	return out, nil
}

// --- Fake RefRepository ---

type fakeRefRepo struct {
	refs      []domain.Ref
	saveErr   error
	deleteErr error
}

func (r *fakeRefRepo) SaveRef(_ context.Context, _ string, ref domain.Ref) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.refs = append(r.refs, ref)
	return nil
}

func (r *fakeRefRepo) DeleteRef(_ context.Context, _ string, entityID string, url string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	for i, ref := range r.refs {
		if ref.EntityID == entityID && ref.URL == url {
			r.refs = append(r.refs[:i], r.refs[i+1:]...)
			return nil
		}
	}
	return driven.ErrRefNotFound
}

func (r *fakeRefRepo) GetRef(_ context.Context, _ string, entityID string, url string) (*domain.Ref, error) {
	for i, ref := range r.refs {
		if ref.EntityID == entityID && ref.URL == url {
			return &r.refs[i], nil
		}
	}
	return nil, nil
}

func (r *fakeRefRepo) ListRefsByEntity(_ context.Context, _ string, entityID string) ([]domain.Ref, error) {
	if entityID == "" {
		return r.refs, nil
	}
	var out []domain.Ref
	for _, ref := range r.refs {
		if ref.EntityID == entityID {
			out = append(out, ref)
		}
	}
	return out, nil
}

// --- Fake EntityResolver ---

type fakeEntityResolver struct {
	entities map[string]string // id → title
}

func (r *fakeEntityResolver) ResolveEntity(_ context.Context, _ string, id string) (string, error) {
	title, ok := r.entities[id]
	if !ok {
		return "", driven.ErrEntityNotFound
	}
	return title, nil
}

// --- Helper to build a wired LinkService ---

func newTestLinkService() (*service.LinkService, *fakeLinkRepo, *fakeRefRepo, *fakeEntityResolver, *fakeHistoryRepo) {
	fs := newFakeFS("/tmp/project")
	fs.existing["/tmp/project/d7/.db"] = true

	linkRepo := &fakeLinkRepo{}
	refRepo := &fakeRefRepo{}
	resolver := &fakeEntityResolver{
		entities: map[string]string{
			"STORY-001": "User logs in",
			"STORY-002": "User logs out",
			"STORY-003": "User resets password",
			"EPIC-001":  "Authentication",
			"FEAT-001":  "Login flow",
			"SPEC-001":  "Login rules",
		},
	}
	history := &fakeHistoryRepo{}

	svc := service.NewLinkService(fs, linkRepo, refRepo, resolver, history)
	return svc, linkRepo, refRepo, resolver, history
}

// --- Link tests ---

func TestAddLink_Success(t *testing.T) {
	t.Parallel()
	svc, linkRepo, _, _, history := newTestLinkService()

	link, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkBlockedBy,
		ToID:   "STORY-002",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.FromID != "STORY-001" || link.ToID != "STORY-002" || link.Kind != domain.LinkBlockedBy {
		t.Errorf("unexpected link: %+v", link)
	}
	if len(linkRepo.links) != 1 {
		t.Errorf("expected 1 stored link, got %d", len(linkRepo.links))
	}
	if len(history.entries) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history.entries))
	}
}

func TestAddLink_SelfLink(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	_, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkBlockedBy,
		ToID:   "STORY-001",
	})
	if !errors.Is(err, service.ErrLinkSelfLink) {
		t.Errorf("expected ErrLinkSelfLink, got %v", err)
	}
}

func TestAddLink_FromNotFound(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	_, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-999",
		Kind:   domain.LinkRelatesTo,
		ToID:   "STORY-001",
	})
	if err == nil {
		t.Fatal("expected error for unknown from entity")
	}
}

func TestAddLink_ToNotFound(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	_, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkRelatesTo,
		ToID:   "STORY-999",
	})
	if err == nil {
		t.Fatal("expected error for unknown to entity")
	}
}

func TestAddLink_Duplicate_Idempotent(t *testing.T) {
	t.Parallel()
	svc, linkRepo, _, _, _ := newTestLinkService()

	req := driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkBlockedBy,
		ToID:   "STORY-002",
	}

	_, err := svc.AddLink(context.Background(), req)
	if err != nil {
		t.Fatalf("first add: %v", err)
	}

	// Second add should be idempotent.
	_, err = svc.AddLink(context.Background(), req)
	if err != nil {
		t.Fatalf("second add: %v", err)
	}

	if len(linkRepo.links) != 1 {
		t.Errorf("expected 1 stored link after duplicate, got %d", len(linkRepo.links))
	}
}

func TestAddLink_SymmetricDuplicate_Idempotent(t *testing.T) {
	t.Parallel()
	svc, linkRepo, _, _, _ := newTestLinkService()

	// Add A relates-to B.
	_, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkRelatesTo,
		ToID:   "STORY-002",
	})
	if err != nil {
		t.Fatalf("first add: %v", err)
	}

	// Add B relates-to A — should be idempotent (same logical link).
	_, err = svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-002",
		Kind:   domain.LinkRelatesTo,
		ToID:   "STORY-001",
	})
	if err != nil {
		t.Fatalf("reverse add: %v", err)
	}

	if len(linkRepo.links) != 1 {
		t.Errorf("expected 1 stored link after symmetric duplicate, got %d", len(linkRepo.links))
	}
}

func TestAddLink_BlockedByCycleDetection(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	// STORY-001 blocked-by STORY-002
	_, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkBlockedBy,
		ToID:   "STORY-002",
	})
	if err != nil {
		t.Fatalf("first link: %v", err)
	}

	// STORY-002 blocked-by STORY-003
	_, err = svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-002",
		Kind:   domain.LinkBlockedBy,
		ToID:   "STORY-003",
	})
	if err != nil {
		t.Fatalf("second link: %v", err)
	}

	// STORY-003 blocked-by STORY-001 — would create a cycle.
	_, err = svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-003",
		Kind:   domain.LinkBlockedBy,
		ToID:   "STORY-001",
	})
	if !errors.Is(err, service.ErrLinkCycle) {
		t.Errorf("expected ErrLinkCycle, got %v", err)
	}
}

func TestAddLink_CrossType(t *testing.T) {
	t.Parallel()
	svc, linkRepo, _, _, _ := newTestLinkService()

	link, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkRelatesTo,
		ToID:   "EPIC-001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.FromID != "STORY-001" || link.ToID != "EPIC-001" {
		t.Errorf("unexpected link: %+v", link)
	}
	if len(linkRepo.links) != 1 {
		t.Errorf("expected 1 stored link, got %d", len(linkRepo.links))
	}
}

func TestRemoveLink_Success(t *testing.T) {
	t.Parallel()
	svc, linkRepo, _, _, _ := newTestLinkService()

	_, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkBlockedBy,
		ToID:   "STORY-002",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	err = svc.RemoveLink(context.Background(), driving.RemoveLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkBlockedBy,
		ToID:   "STORY-002",
	})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}

	if len(linkRepo.links) != 0 {
		t.Errorf("expected 0 links after removal, got %d", len(linkRepo.links))
	}
}

func TestRemoveLink_SymmetricReverse(t *testing.T) {
	t.Parallel()
	svc, linkRepo, _, _, _ := newTestLinkService()

	// Add A relates-to B (stored as A→B).
	_, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkRelatesTo,
		ToID:   "STORY-002",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// Remove B relates-to A (reverse of stored direction).
	err = svc.RemoveLink(context.Background(), driving.RemoveLinkRequest{
		FromID: "STORY-002",
		Kind:   domain.LinkRelatesTo,
		ToID:   "STORY-001",
	})
	if err != nil {
		t.Fatalf("remove reverse: %v", err)
	}

	if len(linkRepo.links) != 0 {
		t.Errorf("expected 0 links after symmetric removal, got %d", len(linkRepo.links))
	}
}

func TestListLinks(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	// Add two links touching STORY-001.
	_, _ = svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkBlockedBy,
		ToID:   "STORY-002",
	})
	_, _ = svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-003",
		Kind:   domain.LinkRelatesTo,
		ToID:   "STORY-001",
	})

	links, err := svc.ListLinks(context.Background(), "", "STORY-001")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}

	// Verify titles are resolved.
	for _, rl := range links {
		if rl.FromTitle == "" || rl.ToTitle == "" {
			t.Errorf("expected resolved titles, got from=%q to=%q", rl.FromTitle, rl.ToTitle)
		}
	}
}

func TestAddLink_MissingFromID(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	_, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		Kind: domain.LinkBlockedBy,
		ToID: "STORY-001",
	})
	if !errors.Is(err, service.ErrLinkFromRequired) {
		t.Errorf("expected ErrLinkFromRequired, got %v", err)
	}
}

func TestAddLink_MissingToID(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	_, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkBlockedBy,
	})
	if !errors.Is(err, service.ErrLinkToRequired) {
		t.Errorf("expected ErrLinkToRequired, got %v", err)
	}
}

// --- Ref tests ---

func TestAddRef_Success(t *testing.T) {
	t.Parallel()
	svc, _, refRepo, _, history := newTestLinkService()

	ref, err := svc.AddRef(context.Background(), driving.AddRefRequest{
		EntityID: "STORY-001",
		URL:      "https://figma.com/login-mockup",
		Label:    "Login mockup",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.EntityID != "STORY-001" || ref.URL != "https://figma.com/login-mockup" || ref.Label != "Login mockup" {
		t.Errorf("unexpected ref: %+v", ref)
	}
	if len(refRepo.refs) != 1 {
		t.Errorf("expected 1 stored ref, got %d", len(refRepo.refs))
	}
	if len(history.entries) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history.entries))
	}
}

func TestAddRef_Duplicate_Idempotent(t *testing.T) {
	t.Parallel()
	svc, _, refRepo, _, _ := newTestLinkService()

	req := driving.AddRefRequest{
		EntityID: "STORY-001",
		URL:      "https://figma.com/login-mockup",
	}

	_, err := svc.AddRef(context.Background(), req)
	if err != nil {
		t.Fatalf("first add: %v", err)
	}

	_, err = svc.AddRef(context.Background(), req)
	if err != nil {
		t.Fatalf("second add: %v", err)
	}

	if len(refRepo.refs) != 1 {
		t.Errorf("expected 1 stored ref after duplicate, got %d", len(refRepo.refs))
	}
}

func TestAddRef_EntityNotFound(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	_, err := svc.AddRef(context.Background(), driving.AddRefRequest{
		EntityID: "STORY-999",
		URL:      "https://example.com",
	})
	if err == nil {
		t.Fatal("expected error for unknown entity")
	}
}

func TestAddRef_MissingEntityID(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	_, err := svc.AddRef(context.Background(), driving.AddRefRequest{
		URL: "https://example.com",
	})
	if !errors.Is(err, service.ErrRefEntityRequired) {
		t.Errorf("expected ErrRefEntityRequired, got %v", err)
	}
}

func TestAddRef_MissingURL(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	_, err := svc.AddRef(context.Background(), driving.AddRefRequest{
		EntityID: "STORY-001",
	})
	if !errors.Is(err, service.ErrRefURLRequired) {
		t.Errorf("expected ErrRefURLRequired, got %v", err)
	}
}

func TestRemoveRef_Success(t *testing.T) {
	t.Parallel()
	svc, _, refRepo, _, _ := newTestLinkService()

	_, err := svc.AddRef(context.Background(), driving.AddRefRequest{
		EntityID: "STORY-001",
		URL:      "https://figma.com/login-mockup",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	err = svc.RemoveRef(context.Background(), driving.RemoveRefRequest{
		EntityID: "STORY-001",
		URL:      "https://figma.com/login-mockup",
	})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}

	if len(refRepo.refs) != 0 {
		t.Errorf("expected 0 refs after removal, got %d", len(refRepo.refs))
	}
}

func TestListRefs(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestLinkService()

	_, _ = svc.AddRef(context.Background(), driving.AddRefRequest{
		EntityID: "STORY-001",
		URL:      "https://figma.com/login",
	})
	_, _ = svc.AddRef(context.Background(), driving.AddRefRequest{
		EntityID: "STORY-001",
		URL:      "https://github.com/issue/42",
		Label:    "OAuth bug",
	})
	_, _ = svc.AddRef(context.Background(), driving.AddRefRequest{
		EntityID: "STORY-002",
		URL:      "https://other.com",
	})

	refs, err := svc.ListRefs(context.Background(), "", "STORY-001")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(refs) != 2 {
		t.Errorf("expected 2 refs for STORY-001, got %d", len(refs))
	}
}

func TestAddLink_WorkspaceNotFound(t *testing.T) {
	t.Parallel()
	fs := newFakeFS("/tmp/project")
	// Don't add d7/.db to existing → workspace not found.
	svc := service.NewLinkService(fs, &fakeLinkRepo{}, &fakeRefRepo{}, &fakeEntityResolver{entities: map[string]string{}}, &fakeHistoryRepo{})

	_, err := svc.AddLink(context.Background(), driving.AddLinkRequest{
		FromID: "STORY-001",
		Kind:   domain.LinkBlockedBy,
		ToID:   "STORY-002",
	})
	if err == nil {
		t.Fatal("expected workspace not found error")
	}
}
