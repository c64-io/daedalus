package clover

import (
	"context"
	"fmt"
	"time"

	c "github.com/ostafen/clover/v2"
	d "github.com/ostafen/clover/v2/document"
	q "github.com/ostafen/clover/v2/query"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// Clover collection and field names for specs.
const (
	specsCollection = "specs"

	fieldStoryID = "story_id"
)

// Compile-time assertion.
var _ driven.SpecRepository = (*SpecRepository)(nil)

// SpecRepository is the Clover v2 implementation of
// driven.SpecRepository.
type SpecRepository struct{}

// NewSpecRepository returns a Clover-backed spec repository.
func NewSpecRepository() *SpecRepository {
	return &SpecRepository{}
}

// NextSpecSeq atomically increments the "SPEC" counter and returns
// the new value.
func (r *SpecRepository) NextSpecSeq(_ context.Context, dbDir string) (_ int, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return 0, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, countersCollection); err != nil {
		return 0, err
	}

	crit := q.NewQuery(countersCollection).Where(q.Field(fieldCounterName).Eq(domain.SpecIDPrefix))

	doc, err := db.FindFirst(crit)
	if err != nil {
		return 0, fmt.Errorf("find spec counter: %w", err)
	}

	var next int
	if doc == nil {
		next = 1
		newDoc := d.NewDocument()
		newDoc.Set(fieldCounterName, domain.SpecIDPrefix)
		newDoc.Set(fieldCounterVal, next)
		if _, err := db.InsertOne(countersCollection, newDoc); err != nil {
			return 0, fmt.Errorf("insert spec counter: %w", err)
		}
	} else {
		cur := toInt(doc.Get(fieldCounterVal))
		next = cur + 1
		if err := db.UpdateById(countersCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
			doc.Set(fieldCounterVal, next)
			return doc
		}); err != nil {
			return 0, fmt.Errorf("update spec counter: %w", err)
		}
	}

	return next, nil
}

// SaveSpec persists a fully constructed Spec into the specs collection.
func (r *SpecRepository) SaveSpec(_ context.Context, dbDir string, spec domain.Spec) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, specsCollection); err != nil {
		return err
	}

	doc := d.NewDocument()
	doc.Set(fieldID, spec.ID)
	doc.Set(fieldStoryID, spec.StoryID)
	doc.Set(fieldTitle, spec.Title)
	doc.Set(fieldDescription, spec.Description)
	doc.Set(fieldStatus, string(spec.Status))
	doc.Set(fieldCreatedAt, spec.CreatedAt.Format(time.RFC3339))

	if _, err := db.InsertOne(specsCollection, doc); err != nil {
		return fmt.Errorf("insert spec: %w", err)
	}

	return nil
}

// GetSpec retrieves a single Spec by its human-readable ID.
func (r *SpecRepository) GetSpec(_ context.Context, dbDir string, id string) (_ *domain.Spec, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(specsCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", specsCollection, err)
	}
	if !has {
		return nil, fmt.Errorf("%w: %s", driven.ErrSpecNotFound, id)
	}

	doc, err := db.FindFirst(q.NewQuery(specsCollection).Where(q.Field(fieldID).Eq(id)))
	if err != nil {
		return nil, fmt.Errorf("find spec %s: %w", id, err)
	}
	if doc == nil {
		return nil, fmt.Errorf("%w: %s", driven.ErrSpecNotFound, id)
	}

	return docToSpec(doc)
}

// ListSpecs returns Specs ordered by creation time. If storyID
// is non-empty, only Specs belonging to that Story are returned.
func (r *SpecRepository) ListSpecs(_ context.Context, dbDir string, storyID string) (_ []domain.Spec, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(specsCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", specsCollection, err)
	}
	if !has {
		return nil, nil
	}

	query := q.NewQuery(specsCollection).Sort(q.SortOption{
		Field:     fieldCreatedAt,
		Direction: 1,
	})

	if storyID != "" {
		query = query.Where(q.Field(fieldStoryID).Eq(storyID))
	}

	docs, err := db.FindAll(query)
	if err != nil {
		return nil, fmt.Errorf("list specs: %w", err)
	}

	specs := make([]domain.Spec, 0, len(docs))
	for _, doc := range docs {
		spec, err := docToSpec(doc)
		if err != nil {
			return nil, err
		}
		specs = append(specs, *spec)
	}

	return specs, nil
}

// UpdateSpec replaces an existing Spec by its ID.
func (r *SpecRepository) UpdateSpec(_ context.Context, dbDir string, spec domain.Spec) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	doc, err := db.FindFirst(q.NewQuery(specsCollection).Where(q.Field(fieldID).Eq(spec.ID)))
	if err != nil {
		return fmt.Errorf("find spec %s: %w", spec.ID, err)
	}
	if doc == nil {
		return fmt.Errorf("%w: %s", driven.ErrSpecNotFound, spec.ID)
	}

	return db.UpdateById(specsCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
		doc.Set(fieldTitle, spec.Title)
		doc.Set(fieldDescription, spec.Description)
		doc.Set(fieldStatus, string(spec.Status))
		return doc
	})
}

// docToSpec converts a Clover document back into a domain.Spec.
func docToSpec(doc *d.Document) (*domain.Spec, error) {
	id, _ := doc.Get(fieldID).(string)
	storyID, _ := doc.Get(fieldStoryID).(string)
	title, _ := doc.Get(fieldTitle).(string)
	desc, _ := doc.Get(fieldDescription).(string)
	statusRaw, _ := doc.Get(fieldStatus).(string)
	createdRaw, _ := doc.Get(fieldCreatedAt).(string)

	status, err := domain.ParseStatus(statusRaw)
	if err != nil {
		return nil, fmt.Errorf("decode spec %s status: %w", id, err)
	}

	createdAt, err := time.Parse(time.RFC3339, createdRaw)
	if err != nil {
		return nil, fmt.Errorf("decode spec %s created_at: %w", id, err)
	}

	return &domain.Spec{
		ID:          id,
		StoryID:     storyID,
		Title:       title,
		Description: desc,
		Status:      status,
		CreatedAt:   createdAt,
	}, nil
}
