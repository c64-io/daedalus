package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// ai_context.go groups the helpers that both expand_service.go and
// suggest_service.go use to build a DialogContext for an entity. Each
// helper is small, pure of command-specific knowledge, and reusable:
//
//   • attachCommonContext  — workspace description + links + refs.
//   • resolveDialogLink    — normalize link direction relative to self.
//   • {idea,epic,feature,story,spec}ToContextEntity — typed → dialog.
//   • sizeString           — Size → "" or "1","2","3",…
//
// These used to live inside expand_service.go. They were extracted so a
// second service (SuggestService) could reuse them without fighting Go's
// method-set receiver rules. Nothing here is command-specific.
//
// contextSource is embedded in ExpandService and SuggestService so
// callers write s.attachCommonContext(...) the same way in both places.
type contextSource struct {
	wsRepo   driven.WorkspaceRepository
	linkRepo driven.LinkRepository
	refRepo  driven.RefRepository
	resolver driven.EntityResolver
}

// attachCommonContext populates the workspace description, links, and
// refs — the slice of context every entity shares regardless of depth.
func (cs *contextSource) attachCommonContext(ctx context.Context, dbDir string, selfID string, dc *domain.DialogContext) error {
	if desc, err := cs.wsRepo.ReadProjectDescription(ctx, dbDir); err == nil {
		dc.Workspace = desc
	} else if !errors.Is(err, driven.ErrProjectDescriptionNotFound) {
		return fmt.Errorf("read project description: %w", err)
	}

	links, err := cs.linkRepo.ListLinksByEntity(ctx, dbDir, selfID)
	if err != nil {
		return fmt.Errorf("list links: %w", err)
	}
	for _, l := range links {
		dcl, err := cs.resolveDialogLink(ctx, dbDir, selfID, l)
		if err != nil {
			return err
		}
		dc.Links = append(dc.Links, dcl)
	}

	refs, err := cs.refRepo.ListRefsByEntity(ctx, dbDir, selfID)
	if err != nil {
		return fmt.Errorf("list refs: %w", err)
	}
	for _, r := range refs {
		dc.Refs = append(dc.Refs, domain.DialogContextRef{URL: r.URL, Label: r.Label})
	}
	return nil
}

// resolveDialogLink turns a stored Link into a DialogContextLink with
// the "other" endpoint resolved to its title and the relation label
// direction-normalized relative to selfID.
func (cs *contextSource) resolveDialogLink(ctx context.Context, dbDir string, selfID string, l domain.Link) (domain.DialogContextLink, error) {
	var other string
	var relation string
	switch {
	case l.FromID == selfID:
		other = l.ToID
		relation = string(l.Kind)
	case l.ToID == selfID:
		other = l.FromID
		relation = l.Kind.InverseLabel()
	default:
		other = l.ToID
		relation = string(l.Kind)
	}
	title, err := cs.resolver.ResolveEntity(ctx, dbDir, other)
	if err != nil && !errors.Is(err, driven.ErrEntityNotFound) {
		return domain.DialogContextLink{}, fmt.Errorf("resolve link endpoint %s: %w", other, err)
	}
	return domain.DialogContextLink{
		Relation:   relation,
		OtherID:    other,
		OtherTitle: title,
	}, nil
}

// ---------------------------------------------------------------------
// Typed → DialogContextEntity converters.
// ---------------------------------------------------------------------

func ideaToContextEntity(i domain.Idea) domain.DialogContextEntity {
	return domain.DialogContextEntity{
		Kind:        "Idea",
		ID:          i.ID,
		Title:       i.Title,
		Status:      string(i.Status),
		Description: i.Description,
	}
}

func epicToContextEntity(e domain.Epic) domain.DialogContextEntity {
	return domain.DialogContextEntity{
		Kind:        "Epic",
		ID:          e.ID,
		Title:       e.Title,
		Status:      string(e.Status),
		Description: e.Description,
		Priority:    string(e.Priority),
		Size:        sizeString(e.Size),
	}
}

func featureToContextEntity(f domain.Feature) domain.DialogContextEntity {
	return domain.DialogContextEntity{
		Kind:        "Feature",
		ID:          f.ID,
		Title:       f.Title,
		Status:      string(f.Status),
		Description: f.Description,
		Priority:    string(f.Priority),
		Size:        sizeString(f.Size),
	}
}

func storyToContextEntity(s domain.Story) domain.DialogContextEntity {
	return domain.DialogContextEntity{
		Kind:        "Story",
		ID:          s.ID,
		Title:       s.Title,
		Status:      string(s.Status),
		Description: s.Description,
		Priority:    string(s.Priority),
		Size:        sizeString(s.Size),
	}
}

// specToContextEntity — Specs do not carry priority or size (they are
// prose, not work items), so those fields are left empty.
func specToContextEntity(sp domain.Spec) domain.DialogContextEntity {
	return domain.DialogContextEntity{
		Kind:        "Spec",
		ID:          sp.ID,
		Title:       sp.Title,
		Status:      string(sp.Status),
		Description: sp.Description,
	}
}

// sizeString renders a Size as a Fibonacci string, or "" for unset.
func sizeString(sz domain.Size) string {
	if sz == 0 {
		return ""
	}
	return fmt.Sprintf("%d", int(sz))
}
