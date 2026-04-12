package driven

import (
	"context"
	"errors"

	"github.com/c64-io/daedalus/internal/core/domain"
)

// ErrLinkNotFound is returned when a link lookup finds no match.
var ErrLinkNotFound = errors.New("link not found")

// ErrRefNotFound is returned when an external ref lookup finds no match.
var ErrRefNotFound = errors.New("ref not found")

// ErrEntityNotFound is returned by EntityResolver when the target
// entity does not exist in any collection.
var ErrEntityNotFound = errors.New("entity not found")

// LinkRepository is the driven port for link storage.
type LinkRepository interface {
	// SaveLink persists a new link.
	SaveLink(ctx context.Context, dbDir string, link domain.Link) error

	// DeleteLink removes a link by its composite key (from, kind, to).
	// Returns ErrLinkNotFound if no such link exists.
	DeleteLink(ctx context.Context, dbDir string, fromID string, kind domain.LinkKind, toID string) error

	// GetLink retrieves a specific link by its composite key.
	// Returns nil if no match.
	GetLink(ctx context.Context, dbDir string, fromID string, kind domain.LinkKind, toID string) (*domain.Link, error)

	// ListLinksByEntity returns all links where entityID is either
	// the from or to endpoint.
	ListLinksByEntity(ctx context.Context, dbDir string, entityID string) ([]domain.Link, error)

	// ListAllLinksByKind returns all links of a given kind. Used for
	// cycle detection on blocked-by.
	ListAllLinksByKind(ctx context.Context, dbDir string, kind domain.LinkKind) ([]domain.Link, error)
}

// RefRepository is the driven port for external reference storage.
type RefRepository interface {
	// SaveRef persists a new external reference.
	SaveRef(ctx context.Context, dbDir string, ref domain.Ref) error

	// DeleteRef removes a ref by entity ID and URL.
	// Returns ErrRefNotFound if no such ref exists.
	DeleteRef(ctx context.Context, dbDir string, entityID string, url string) error

	// GetRef retrieves a specific ref by entity ID and URL.
	// Returns nil if no match.
	GetRef(ctx context.Context, dbDir string, entityID string, url string) (*domain.Ref, error)

	// ListRefsByEntity returns all refs attached to the given entity.
	ListRefsByEntity(ctx context.Context, dbDir string, entityID string) ([]domain.Ref, error)
}

// EntityResolver resolves an entity ID to its title. This is used by
// the link service to validate that endpoints exist and to enrich
// link listings with human-readable titles.
type EntityResolver interface {
	// ResolveEntity looks up an entity by ID (any type) and returns
	// its title. Returns ErrEntityNotFound if the ID does not exist.
	ResolveEntity(ctx context.Context, dbDir string, id string) (title string, err error)
}
