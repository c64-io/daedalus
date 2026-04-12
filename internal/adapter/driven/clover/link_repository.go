package clover

import (
	"context"
	"fmt"
	"strings"
	"time"

	c "github.com/ostafen/clover/v2"
	d "github.com/ostafen/clover/v2/document"
	q "github.com/ostafen/clover/v2/query"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// Clover collection and field names for links and refs.
const (
	linksCollection = "links"
	refsCollection  = "refs"

	fieldFromID = "from_id"
	fieldToID   = "to_id"
	fieldKind   = "kind"
	fieldURL    = "url"
	fieldLabel  = "label"
)

// Compile-time assertions.
var (
	_ driven.LinkRepository   = (*LinkRepository)(nil)
	_ driven.RefRepository    = (*RefRepository)(nil)
	_ driven.EntityResolver   = (*EntityResolverAdapter)(nil)
)

// ---- LinkRepository ----

// LinkRepository is the Clover v2 implementation of driven.LinkRepository.
type LinkRepository struct{}

// NewLinkRepository returns a Clover-backed link repository.
func NewLinkRepository() *LinkRepository {
	return &LinkRepository{}
}

// SaveLink persists a new link.
func (r *LinkRepository) SaveLink(_ context.Context, dbDir string, link domain.Link) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, linksCollection); err != nil {
		return err
	}

	doc := d.NewDocument()
	doc.Set(fieldFromID, link.FromID)
	doc.Set(fieldToID, link.ToID)
	doc.Set(fieldKind, string(link.Kind))
	doc.Set(fieldCreatedAt, link.CreatedAt.Format(time.RFC3339))

	if _, err := db.InsertOne(linksCollection, doc); err != nil {
		return fmt.Errorf("insert link: %w", err)
	}
	return nil
}

// DeleteLink removes a link by its composite key.
func (r *LinkRepository) DeleteLink(_ context.Context, dbDir string, fromID string, kind domain.LinkKind, toID string) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(linksCollection)
	if err != nil {
		return fmt.Errorf("check %q collection: %w", linksCollection, err)
	}
	if !has {
		return fmt.Errorf("%w: %s %s %s", driven.ErrLinkNotFound, fromID, kind, toID)
	}

	criteria := q.NewQuery(linksCollection).Where(
		q.Field(fieldFromID).Eq(fromID).
			And(q.Field(fieldKind).Eq(string(kind))).
			And(q.Field(fieldToID).Eq(toID)),
	)

	doc, err := db.FindFirst(criteria)
	if err != nil {
		return fmt.Errorf("find link: %w", err)
	}
	if doc == nil {
		return fmt.Errorf("%w: %s %s %s", driven.ErrLinkNotFound, fromID, kind, toID)
	}

	return db.DeleteById(linksCollection, doc.ObjectId())
}

// GetLink retrieves a specific link by its composite key.
func (r *LinkRepository) GetLink(_ context.Context, dbDir string, fromID string, kind domain.LinkKind, toID string) (_ *domain.Link, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(linksCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", linksCollection, err)
	}
	if !has {
		return nil, nil
	}

	criteria := q.NewQuery(linksCollection).Where(
		q.Field(fieldFromID).Eq(fromID).
			And(q.Field(fieldKind).Eq(string(kind))).
			And(q.Field(fieldToID).Eq(toID)),
	)

	doc, err := db.FindFirst(criteria)
	if err != nil {
		return nil, fmt.Errorf("find link: %w", err)
	}
	if doc == nil {
		return nil, nil
	}

	return docToLink(doc)
}

// ListLinksByEntity returns all links where entityID is either endpoint.
func (r *LinkRepository) ListLinksByEntity(_ context.Context, dbDir string, entityID string) (_ []domain.Link, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(linksCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", linksCollection, err)
	}
	if !has {
		return nil, nil
	}

	query := q.NewQuery(linksCollection)
	if entityID != "" {
		query = query.Where(
			q.Field(fieldFromID).Eq(entityID).
				Or(q.Field(fieldToID).Eq(entityID)),
		)
	}

	docs, err := db.FindAll(query)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}

	links := make([]domain.Link, 0, len(docs))
	for _, doc := range docs {
		link, err := docToLink(doc)
		if err != nil {
			return nil, err
		}
		links = append(links, *link)
	}
	return links, nil
}

// ListAllLinksByKind returns all links of a given kind.
func (r *LinkRepository) ListAllLinksByKind(_ context.Context, dbDir string, kind domain.LinkKind) (_ []domain.Link, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(linksCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", linksCollection, err)
	}
	if !has {
		return nil, nil
	}

	docs, err := db.FindAll(
		q.NewQuery(linksCollection).Where(q.Field(fieldKind).Eq(string(kind))),
	)
	if err != nil {
		return nil, fmt.Errorf("list links by kind: %w", err)
	}

	links := make([]domain.Link, 0, len(docs))
	for _, doc := range docs {
		link, err := docToLink(doc)
		if err != nil {
			return nil, err
		}
		links = append(links, *link)
	}
	return links, nil
}

func docToLink(doc *d.Document) (*domain.Link, error) {
	fromID, _ := doc.Get(fieldFromID).(string)
	toID, _ := doc.Get(fieldToID).(string)
	kindRaw, _ := doc.Get(fieldKind).(string)
	createdRaw, _ := doc.Get(fieldCreatedAt).(string)

	kind, err := domain.ParseLinkKind(kindRaw)
	if err != nil {
		return nil, fmt.Errorf("decode link kind: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, createdRaw)
	if err != nil {
		return nil, fmt.Errorf("decode link created_at: %w", err)
	}

	return &domain.Link{
		FromID:    fromID,
		ToID:      toID,
		Kind:      kind,
		CreatedAt: createdAt,
	}, nil
}

// ---- RefRepository ----

// RefRepository is the Clover v2 implementation of driven.RefRepository.
type RefRepository struct{}

// NewRefRepository returns a Clover-backed ref repository.
func NewRefRepository() *RefRepository {
	return &RefRepository{}
}

// SaveRef persists a new external reference.
func (r *RefRepository) SaveRef(_ context.Context, dbDir string, ref domain.Ref) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, refsCollection); err != nil {
		return err
	}

	doc := d.NewDocument()
	doc.Set(fieldEntityID, ref.EntityID)
	doc.Set(fieldURL, ref.URL)
	doc.Set(fieldLabel, ref.Label)
	doc.Set(fieldCreatedAt, ref.CreatedAt.Format(time.RFC3339))

	if _, err := db.InsertOne(refsCollection, doc); err != nil {
		return fmt.Errorf("insert ref: %w", err)
	}
	return nil
}

// DeleteRef removes a ref by entity ID and URL.
func (r *RefRepository) DeleteRef(_ context.Context, dbDir string, entityID string, url string) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(refsCollection)
	if err != nil {
		return fmt.Errorf("check %q collection: %w", refsCollection, err)
	}
	if !has {
		return fmt.Errorf("%w: %s %s", driven.ErrRefNotFound, entityID, url)
	}

	criteria := q.NewQuery(refsCollection).Where(
		q.Field(fieldEntityID).Eq(entityID).
			And(q.Field(fieldURL).Eq(url)),
	)

	doc, err := db.FindFirst(criteria)
	if err != nil {
		return fmt.Errorf("find ref: %w", err)
	}
	if doc == nil {
		return fmt.Errorf("%w: %s %s", driven.ErrRefNotFound, entityID, url)
	}

	return db.DeleteById(refsCollection, doc.ObjectId())
}

// GetRef retrieves a specific ref by entity ID and URL.
func (r *RefRepository) GetRef(_ context.Context, dbDir string, entityID string, url string) (_ *domain.Ref, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(refsCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", refsCollection, err)
	}
	if !has {
		return nil, nil
	}

	criteria := q.NewQuery(refsCollection).Where(
		q.Field(fieldEntityID).Eq(entityID).
			And(q.Field(fieldURL).Eq(url)),
	)

	doc, err := db.FindFirst(criteria)
	if err != nil {
		return nil, fmt.Errorf("find ref: %w", err)
	}
	if doc == nil {
		return nil, nil
	}

	return docToRef(doc)
}

// ListRefsByEntity returns all refs attached to the given entity.
func (r *RefRepository) ListRefsByEntity(_ context.Context, dbDir string, entityID string) (_ []domain.Ref, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(refsCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", refsCollection, err)
	}
	if !has {
		return nil, nil
	}

	query := q.NewQuery(refsCollection)
	if entityID != "" {
		query = query.Where(q.Field(fieldEntityID).Eq(entityID))
	}

	docs, err := db.FindAll(query)
	if err != nil {
		return nil, fmt.Errorf("list refs: %w", err)
	}

	refs := make([]domain.Ref, 0, len(docs))
	for _, doc := range docs {
		ref, err := docToRef(doc)
		if err != nil {
			return nil, err
		}
		refs = append(refs, *ref)
	}
	return refs, nil
}

func docToRef(doc *d.Document) (*domain.Ref, error) {
	entityID, _ := doc.Get(fieldEntityID).(string)
	url, _ := doc.Get(fieldURL).(string)
	label, _ := doc.Get(fieldLabel).(string)
	createdRaw, _ := doc.Get(fieldCreatedAt).(string)

	createdAt, err := time.Parse(time.RFC3339, createdRaw)
	if err != nil {
		return nil, fmt.Errorf("decode ref created_at: %w", err)
	}

	return &domain.Ref{
		EntityID:  entityID,
		URL:       url,
		Label:     label,
		CreatedAt: createdAt,
	}, nil
}

// ---- EntityResolverAdapter ----

// EntityResolverAdapter resolves entity IDs to titles by dispatching
// to the appropriate Clover collection based on the ID prefix.
type EntityResolverAdapter struct{}

// NewEntityResolver returns a Clover-backed entity resolver.
func NewEntityResolver() *EntityResolverAdapter {
	return &EntityResolverAdapter{}
}

// prefixToCollection maps entity ID prefixes to their Clover collection.
var prefixToCollection = map[string]string{
	domain.IdeaIDPrefix:     ideasCollection,
	domain.EpicIDPrefix:     epicsCollection,
	domain.FeatureIDPrefix:  featuresCollection,
	domain.StoryIDPrefix:    storiesCollection,
	domain.SpecIDPrefix:     specsCollection,
	domain.ScenarioIDPrefix: scenariosCollection,
}

// ResolveEntity looks up an entity by ID and returns its title.
func (r *EntityResolverAdapter) ResolveEntity(_ context.Context, dbDir string, id string) (_ string, retErr error) {
	// Extract prefix.
	idx := strings.Index(id, "-")
	if idx < 1 {
		return "", fmt.Errorf("%w: invalid ID format %q", driven.ErrEntityNotFound, id)
	}
	prefix := id[:idx]

	collection, ok := prefixToCollection[prefix]
	if !ok {
		return "", fmt.Errorf("%w: unknown prefix %q in %q", driven.ErrEntityNotFound, prefix, id)
	}

	db, err := c.Open(dbDir)
	if err != nil {
		return "", fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(collection)
	if err != nil {
		return "", fmt.Errorf("check %q collection: %w", collection, err)
	}
	if !has {
		return "", fmt.Errorf("%w: %s", driven.ErrEntityNotFound, id)
	}

	doc, err := db.FindFirst(q.NewQuery(collection).Where(q.Field(fieldID).Eq(id)))
	if err != nil {
		return "", fmt.Errorf("find entity %s: %w", id, err)
	}
	if doc == nil {
		return "", fmt.Errorf("%w: %s", driven.ErrEntityNotFound, id)
	}

	title, _ := doc.Get(fieldTitle).(string)
	return title, nil
}
