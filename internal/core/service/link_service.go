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

// Sentinel errors for LinkService.
var (
	ErrLinkFromRequired = errors.New("from entity ID is required")
	ErrLinkToRequired   = errors.New("to entity ID is required")
	ErrLinkSelfLink     = errors.New("cannot link an entity to itself")
	ErrLinkCycle        = errors.New("adding this blocked-by link would create a cycle")
	ErrRefEntityRequired = errors.New("entity ID is required")
	ErrRefURLRequired    = errors.New("URL is required")
)

// Compile-time assertions.
var (
	_ driving.LinkAdder   = (*LinkService)(nil)
	_ driving.LinkRemover = (*LinkService)(nil)
	_ driving.LinkReader  = (*LinkService)(nil)
	_ driving.RefAdder    = (*LinkService)(nil)
	_ driving.RefRemover  = (*LinkService)(nil)
	_ driving.RefReader   = (*LinkService)(nil)
)

// LinkService implements the link and ref use cases.
type LinkService struct {
	fs       driven.FileSystem
	linkRepo driven.LinkRepository
	refRepo  driven.RefRepository
	resolver driven.EntityResolver
	history  driven.HistoryRepository
}

// NewLinkService wires the service with its driven dependencies.
func NewLinkService(
	fs driven.FileSystem,
	linkRepo driven.LinkRepository,
	refRepo driven.RefRepository,
	resolver driven.EntityResolver,
	history driven.HistoryRepository,
) *LinkService {
	return &LinkService{
		fs:       fs,
		linkRepo: linkRepo,
		refRepo:  refRepo,
		resolver: resolver,
		history:  history,
	}
}

// AddLink validates both endpoints, checks for duplicates and cycles
// (for blocked-by), and persists the link.
func (s *LinkService) AddLink(ctx context.Context, req driving.AddLinkRequest) (*domain.Link, error) {
	if req.FromID == "" {
		return nil, ErrLinkFromRequired
	}
	if req.ToID == "" {
		return nil, ErrLinkToRequired
	}
	if req.FromID == req.ToID {
		return nil, ErrLinkSelfLink
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	// Validate both endpoints exist.
	if _, err := s.resolver.ResolveEntity(ctx, ws.DBDir, req.FromID); err != nil {
		return nil, fmt.Errorf("resolve from entity: %w", err)
	}
	if _, err := s.resolver.ResolveEntity(ctx, ws.DBDir, req.ToID); err != nil {
		return nil, fmt.Errorf("resolve to entity: %w", err)
	}

	// Check for duplicate. For symmetric kinds, also check reverse.
	existing, err := s.linkRepo.GetLink(ctx, ws.DBDir, req.FromID, req.Kind, req.ToID)
	if err != nil {
		return nil, fmt.Errorf("check duplicate link: %w", err)
	}
	if existing != nil {
		return existing, nil // idempotent
	}
	if req.Kind.IsSymmetric() {
		reverse, err := s.linkRepo.GetLink(ctx, ws.DBDir, req.ToID, req.Kind, req.FromID)
		if err != nil {
			return nil, fmt.Errorf("check reverse link: %w", err)
		}
		if reverse != nil {
			return reverse, nil // idempotent
		}
	}

	// Cycle detection for blocked-by.
	if req.Kind == domain.LinkBlockedBy {
		if err := s.detectCycle(ctx, ws.DBDir, req.FromID, req.ToID); err != nil {
			return nil, err
		}
	}

	link := domain.Link{
		FromID:    req.FromID,
		ToID:      req.ToID,
		Kind:      req.Kind,
		CreatedAt: time.Now(),
	}

	if err := s.linkRepo.SaveLink(ctx, ws.DBDir, link); err != nil {
		return nil, fmt.Errorf("save link: %w", err)
	}

	// Record history on both endpoints.
	now := time.Now()
	entries := []domain.HistoryEntry{
		{
			EntityID:  req.FromID,
			Field:     "link." + string(req.Kind),
			OldValue:  "",
			NewValue:  req.ToID,
			Timestamp: now,
		},
		{
			EntityID:  req.ToID,
			Field:     "link." + string(req.Kind),
			OldValue:  "",
			NewValue:  req.FromID,
			Timestamp: now,
		},
	}
	if err := s.history.AppendHistory(ctx, ws.DBDir, entries); err != nil {
		return nil, fmt.Errorf("record link history: %w", err)
	}

	return &link, nil
}

// RemoveLink validates and deletes a link. For symmetric kinds, also
// checks the reverse direction.
func (s *LinkService) RemoveLink(ctx context.Context, req driving.RemoveLinkRequest) error {
	if req.FromID == "" {
		return ErrLinkFromRequired
	}
	if req.ToID == "" {
		return ErrLinkToRequired
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return err
	}

	// Try the exact direction first.
	err = s.linkRepo.DeleteLink(ctx, ws.DBDir, req.FromID, req.Kind, req.ToID)
	if err != nil && errors.Is(err, driven.ErrLinkNotFound) && req.Kind.IsSymmetric() {
		// For symmetric kinds, try the reverse.
		err = s.linkRepo.DeleteLink(ctx, ws.DBDir, req.ToID, req.Kind, req.FromID)
	}
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}

	// Record history on both endpoints.
	now := time.Now()
	entries := []domain.HistoryEntry{
		{
			EntityID:  req.FromID,
			Field:     "link." + string(req.Kind),
			OldValue:  req.ToID,
			NewValue:  "",
			Timestamp: now,
		},
		{
			EntityID:  req.ToID,
			Field:     "link." + string(req.Kind),
			OldValue:  req.FromID,
			NewValue:  "",
			Timestamp: now,
		},
	}
	if err := s.history.AppendHistory(ctx, ws.DBDir, entries); err != nil {
		return fmt.Errorf("record link removal history: %w", err)
	}

	return nil
}

// ListLinks returns all links touching the given entity, with titles
// resolved for display. If entityID is empty, all links are returned.
func (s *LinkService) ListLinks(ctx context.Context, rootDir string, entityID string) ([]driving.ResolvedLink, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}

	links, err := s.linkRepo.ListLinksByEntity(ctx, ws.DBDir, entityID)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}

	resolved := make([]driving.ResolvedLink, 0, len(links))
	for _, link := range links {
		rl := driving.ResolvedLink{Link: link}
		if title, err := s.resolver.ResolveEntity(ctx, ws.DBDir, link.FromID); err == nil {
			rl.FromTitle = title
		}
		if title, err := s.resolver.ResolveEntity(ctx, ws.DBDir, link.ToID); err == nil {
			rl.ToTitle = title
		}
		resolved = append(resolved, rl)
	}

	return resolved, nil
}

// AddRef validates the entity, checks for duplicates, and persists
// the external reference.
func (s *LinkService) AddRef(ctx context.Context, req driving.AddRefRequest) (*domain.Ref, error) {
	if req.EntityID == "" {
		return nil, ErrRefEntityRequired
	}
	if req.URL == "" {
		return nil, ErrRefURLRequired
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return nil, err
	}

	// Validate entity exists.
	if _, err := s.resolver.ResolveEntity(ctx, ws.DBDir, req.EntityID); err != nil {
		return nil, fmt.Errorf("resolve entity: %w", err)
	}

	// Check for duplicate.
	existing, err := s.refRepo.GetRef(ctx, ws.DBDir, req.EntityID, req.URL)
	if err != nil {
		return nil, fmt.Errorf("check duplicate ref: %w", err)
	}
	if existing != nil {
		return existing, nil // idempotent
	}

	ref := domain.Ref{
		EntityID:  req.EntityID,
		URL:       req.URL,
		Label:     req.Label,
		CreatedAt: time.Now(),
	}

	if err := s.refRepo.SaveRef(ctx, ws.DBDir, ref); err != nil {
		return nil, fmt.Errorf("save ref: %w", err)
	}

	// Record history.
	display := req.URL
	if req.Label != "" {
		display = req.Label + " (" + req.URL + ")"
	}
	entries := []domain.HistoryEntry{
		{
			EntityID:  req.EntityID,
			Field:     "ref",
			OldValue:  "",
			NewValue:  display,
			Timestamp: time.Now(),
		},
	}
	if err := s.history.AppendHistory(ctx, ws.DBDir, entries); err != nil {
		return nil, fmt.Errorf("record ref history: %w", err)
	}

	return &ref, nil
}

// RemoveRef validates and deletes an external reference.
func (s *LinkService) RemoveRef(ctx context.Context, req driving.RemoveRefRequest) error {
	if req.EntityID == "" {
		return ErrRefEntityRequired
	}
	if req.URL == "" {
		return ErrRefURLRequired
	}

	ws, err := requireWorkspace(s.fs, req.RootDir)
	if err != nil {
		return err
	}

	if err := s.refRepo.DeleteRef(ctx, ws.DBDir, req.EntityID, req.URL); err != nil {
		return fmt.Errorf("delete ref: %w", err)
	}

	entries := []domain.HistoryEntry{
		{
			EntityID:  req.EntityID,
			Field:     "ref",
			OldValue:  req.URL,
			NewValue:  "",
			Timestamp: time.Now(),
		},
	}
	if err := s.history.AppendHistory(ctx, ws.DBDir, entries); err != nil {
		return fmt.Errorf("record ref removal history: %w", err)
	}

	return nil
}

// ListRefs returns all refs attached to the given entity. If entityID
// is empty, all refs are returned.
func (s *LinkService) ListRefs(ctx context.Context, rootDir string, entityID string) ([]domain.Ref, error) {
	ws, err := requireWorkspace(s.fs, rootDir)
	if err != nil {
		return nil, err
	}

	refs, err := s.refRepo.ListRefsByEntity(ctx, ws.DBDir, entityID)
	if err != nil {
		return nil, fmt.Errorf("list refs: %w", err)
	}

	return refs, nil
}

// detectCycle checks whether adding a blocked-by edge from fromID to
// toID would create a cycle. It does this by walking all outbound
// blocked-by edges from toID (DFS); if fromID is reachable, a cycle
// would form.
func (s *LinkService) detectCycle(ctx context.Context, dbDir string, fromID string, toID string) error {
	allBlockedBy, err := s.linkRepo.ListAllLinksByKind(ctx, dbDir, domain.LinkBlockedBy)
	if err != nil {
		return fmt.Errorf("load blocked-by links for cycle check: %w", err)
	}

	// Build adjacency list: from → [to, to, ...]
	adj := make(map[string][]string)
	for _, link := range allBlockedBy {
		adj[link.FromID] = append(adj[link.FromID], link.ToID)
	}

	// DFS from toID, looking for fromID.
	visited := make(map[string]bool)
	stack := []string{toID}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current == fromID {
			return ErrLinkCycle
		}
		if visited[current] {
			continue
		}
		visited[current] = true
		stack = append(stack, adj[current]...)
	}

	return nil
}
