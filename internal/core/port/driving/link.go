package driving

import (
	"context"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// AddLinkRequest captures the parameters for creating a typed link
// between two entities.
type AddLinkRequest struct {
	RootDir string
	FromID  string
	Kind    domain.LinkKind
	ToID    string
}

// RemoveLinkRequest captures the parameters for deleting a link.
type RemoveLinkRequest struct {
	RootDir string
	FromID  string
	Kind    domain.LinkKind
	ToID    string
}

// ResolvedLink enriches a domain.Link with the titles of both
// endpoints so the CLI can display them without extra queries.
type ResolvedLink struct {
	domain.Link
	FromTitle string
	ToTitle   string
}

// LinkAdder creates a new typed link between two entities.
type LinkAdder interface {
	AddLink(ctx context.Context, req AddLinkRequest) (*domain.Link, error)
}

// LinkRemover deletes an existing link.
type LinkRemover interface {
	RemoveLink(ctx context.Context, req RemoveLinkRequest) error
}

// LinkReader lists links touching an entity, with titles resolved.
type LinkReader interface {
	ListLinks(ctx context.Context, rootDir string, entityID string) ([]ResolvedLink, error)
}

// AddRefRequest captures the parameters for attaching an external
// reference to an entity.
type AddRefRequest struct {
	RootDir  string
	EntityID string
	URL      string
	Label    string
}

// RemoveRefRequest captures the parameters for detaching an external
// reference.
type RemoveRefRequest struct {
	RootDir  string
	EntityID string
	URL      string
}

// RefAdder attaches an external reference to an entity.
type RefAdder interface {
	AddRef(ctx context.Context, req AddRefRequest) (*domain.Ref, error)
}

// RefRemover removes an external reference from an entity.
type RefRemover interface {
	RemoveRef(ctx context.Context, req RemoveRefRequest) error
}

// RefReader lists external refs attached to an entity.
type RefReader interface {
	ListRefs(ctx context.Context, rootDir string, entityID string) ([]domain.Ref, error)
}

// Link is the combined driving surface for the link subcommand group.
// Individual CLI subcommands still take the narrow port they actually
// need; the bundle exists only to keep root-command wiring flat.
type Link interface {
	LinkAdder
	LinkRemover
	LinkReader
}

// Ref is the combined driving surface for the ref subcommand group.
// Individual CLI subcommands still take the narrow port they actually
// need; the bundle exists only to keep root-command wiring flat.
type Ref interface {
	RefAdder
	RefRemover
	RefReader
}
