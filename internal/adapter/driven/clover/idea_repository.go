package clover

import (
	"context"
	"fmt"
	"time"

	c "github.com/ostafen/clover/v2"
	d "github.com/ostafen/clover/v2/document"
	q "github.com/ostafen/clover/v2/query"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
)

// Clover collection and field names for ideas and counters.
const (
	ideasCollection    = "ideas"
	countersCollection = "counters"

	fieldID          = "id"
	fieldTitle       = "title"
	fieldDescription = "description"
	fieldStatus      = "status"
	fieldCreatedAt   = "created_at"
	fieldCounterName = "name"
	fieldCounterVal  = "value"
)

// Compile-time assertion.
var _ port.IdeaRepository = (*IdeaRepository)(nil)

// IdeaRepository is the Clover v2 implementation of
// port.IdeaRepository. Each method opens/closes the DB independently
// — acceptable for a solo-founder CLI; a session pool is a v2
// concern.
type IdeaRepository struct{}

// NewIdeaRepository returns a Clover-backed idea repository.
func NewIdeaRepository() *IdeaRepository {
	return &IdeaRepository{}
}

// NextIdeaSeq atomically increments the "idea" counter in the
// counters collection and returns the new value. Creates the
// collection and seed document on first call.
func (r *IdeaRepository) NextIdeaSeq(_ context.Context, dbDir string) (_ int, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return 0, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, countersCollection); err != nil {
		return 0, err
	}

	crit := q.NewQuery(countersCollection).Where(q.Field(fieldCounterName).Eq(domain.IdeaIDPrefix))

	doc, err := db.FindFirst(crit)
	if err != nil {
		return 0, fmt.Errorf("find idea counter: %w", err)
	}

	var next int
	if doc == nil {
		next = 1
		newDoc := d.NewDocument()
		newDoc.Set(fieldCounterName, domain.IdeaIDPrefix)
		newDoc.Set(fieldCounterVal, next)
		if _, err := db.InsertOne(countersCollection, newDoc); err != nil {
			return 0, fmt.Errorf("insert idea counter: %w", err)
		}
	} else {
		cur := toInt(doc.Get(fieldCounterVal))
		next = cur + 1
		if err := db.UpdateById(countersCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
			doc.Set(fieldCounterVal, next)
			return doc
		}); err != nil {
			return 0, fmt.Errorf("update idea counter: %w", err)
		}
	}

	return next, nil
}

// SaveIdea persists a fully constructed Idea into the ideas
// collection. Creates the collection on first call.
func (r *IdeaRepository) SaveIdea(_ context.Context, dbDir string, idea domain.Idea) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, ideasCollection); err != nil {
		return err
	}

	doc := d.NewDocument()
	doc.Set(fieldID, idea.ID)
	doc.Set(fieldTitle, idea.Title)
	doc.Set(fieldDescription, idea.Description)
	doc.Set(fieldStatus, string(idea.Status))
	doc.Set(fieldCreatedAt, idea.CreatedAt.Format(time.RFC3339))

	if _, err := db.InsertOne(ideasCollection, doc); err != nil {
		return fmt.Errorf("insert idea: %w", err)
	}

	return nil
}

// GetIdea retrieves a single Idea by its human-readable ID. Returns
// a wrapped port.ErrIdeaNotFound if the idea does not exist.
func (r *IdeaRepository) GetIdea(_ context.Context, dbDir string, id string) (_ *domain.Idea, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(ideasCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", ideasCollection, err)
	}
	if !has {
		return nil, fmt.Errorf("%w: %s", port.ErrIdeaNotFound, id)
	}

	doc, err := db.FindFirst(q.NewQuery(ideasCollection).Where(q.Field(fieldID).Eq(id)))
	if err != nil {
		return nil, fmt.Errorf("find idea %s: %w", id, err)
	}
	if doc == nil {
		return nil, fmt.Errorf("%w: %s", port.ErrIdeaNotFound, id)
	}

	return docToIdea(doc)
}

// ListIdeas returns all Ideas ordered by creation time.
func (r *IdeaRepository) ListIdeas(_ context.Context, dbDir string) (_ []domain.Idea, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(ideasCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", ideasCollection, err)
	}
	if !has {
		return nil, nil // no ideas yet
	}

	docs, err := db.FindAll(q.NewQuery(ideasCollection).Sort(q.SortOption{
		Field:     fieldCreatedAt,
		Direction: 1,
	}))
	if err != nil {
		return nil, fmt.Errorf("list ideas: %w", err)
	}

	ideas := make([]domain.Idea, 0, len(docs))
	for _, doc := range docs {
		idea, err := docToIdea(doc)
		if err != nil {
			return nil, err
		}
		ideas = append(ideas, *idea)
	}

	return ideas, nil
}

// UpdateIdea replaces an existing Idea by its ID.
func (r *IdeaRepository) UpdateIdea(_ context.Context, dbDir string, idea domain.Idea) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	doc, err := db.FindFirst(q.NewQuery(ideasCollection).Where(q.Field(fieldID).Eq(idea.ID)))
	if err != nil {
		return fmt.Errorf("find idea %s: %w", idea.ID, err)
	}
	if doc == nil {
		return fmt.Errorf("%w: %s", port.ErrIdeaNotFound, idea.ID)
	}

	return db.UpdateById(ideasCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
		doc.Set(fieldTitle, idea.Title)
		doc.Set(fieldDescription, idea.Description)
		doc.Set(fieldStatus, string(idea.Status))
		return doc
	})
}

// --- helpers ---

// docToIdea converts a Clover document back into a domain.Idea.
func docToIdea(doc *d.Document) (*domain.Idea, error) {
	id, _ := doc.Get(fieldID).(string)
	title, _ := doc.Get(fieldTitle).(string)
	desc, _ := doc.Get(fieldDescription).(string)
	statusRaw, _ := doc.Get(fieldStatus).(string)
	createdRaw, _ := doc.Get(fieldCreatedAt).(string)

	status, err := domain.ParseStatus(statusRaw)
	if err != nil {
		return nil, fmt.Errorf("decode idea %s status: %w", id, err)
	}

	createdAt, err := time.Parse(time.RFC3339, createdRaw)
	if err != nil {
		return nil, fmt.Errorf("decode idea %s created_at: %w", id, err)
	}

	return &domain.Idea{
		ID:          id,
		Title:       title,
		Description: desc,
		Status:      status,
		CreatedAt:   createdAt,
	}, nil
}

// ensureCollection creates a collection if it does not already exist.
func ensureCollection(db *c.DB, name string) error {
	has, err := db.HasCollection(name)
	if err != nil {
		return fmt.Errorf("check %q collection: %w", name, err)
	}
	if !has {
		if err := db.CreateCollection(name); err != nil {
			return fmt.Errorf("create %q collection: %w", name, err)
		}
	}
	return nil
}

// toInt extracts an integer from a Clover document value. Clover's
// internal encoding returns int64 for integers that survived a
// round-trip through the on-disk store, but may return other numeric
// types depending on the code path. This helper normalizes to int.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// closeDB is a helper for deferred DB closes that preserves the first
// error: if retErr is nil and the close fails, retErr is set.
func closeDB(db *c.DB, retErr *error) {
	if closeErr := db.Close(); closeErr != nil && *retErr == nil {
		*retErr = fmt.Errorf("close clover db: %w", closeErr)
	}
}

