package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// loadWorkspace reads the project description from the workspace
// repository. Returns an empty string (not an error) when no
// description has been set yet.
func loadWorkspace(ctx context.Context, wsRepo driven.WorkspaceRepository, dbDir string) (string, error) {
	desc, err := wsRepo.ReadProjectDescription(ctx, dbDir)
	if err == nil {
		return desc, nil
	}
	if errors.Is(err, driven.ErrProjectDescriptionNotFound) {
		return "", nil
	}
	return "", fmt.Errorf("read project description: %w", err)
}

// loadTargetRefs reads the external references attached to an entity.
func loadTargetRefs(ctx context.Context, refRepo driven.RefRepository, dbDir, entityID string) ([]domain.DialogContextRef, error) {
	refs, err := refRepo.ListRefsByEntity(ctx, dbDir, entityID)
	if err != nil {
		return nil, fmt.Errorf("list refs: %w", err)
	}
	out := make([]domain.DialogContextRef, len(refs))
	for i, r := range refs {
		out[i] = domain.DialogContextRef{URL: r.URL, Label: r.Label}
	}
	return out, nil
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

func specToContextEntity(sp domain.Spec) domain.DialogContextEntity {
	return domain.DialogContextEntity{
		Kind:        "Spec",
		ID:          sp.ID,
		Title:       sp.Title,
		Status:      string(sp.Status),
		Description: sp.Description,
	}
}

func scenarioToContextEntity(sc domain.Scenario) domain.DialogContextEntity {
	given := make([]string, len(sc.Given))
	for i, s := range sc.Given {
		given[i] = s.Text
	}
	when := make([]string, len(sc.When))
	for i, s := range sc.When {
		when[i] = s.Text
	}
	then := make([]string, len(sc.Then))
	for i, s := range sc.Then {
		then[i] = s.Text
	}
	tags := make([]string, len(sc.Tags))
	copy(tags, sc.Tags)
	return domain.DialogContextEntity{
		Kind:   "Scenario",
		ID:     sc.ID,
		Title:  sc.Title,
		Status: string(sc.Status),
		Tags:   tags,
		Given:  given,
		When:   when,
		Then:   then,
	}
}

// sizeString renders a Size as a Fibonacci string, or "" for unset.
func sizeString(sz domain.Size) string {
	if sz == 0 {
		return ""
	}
	return fmt.Sprintf("%d", int(sz))
}
