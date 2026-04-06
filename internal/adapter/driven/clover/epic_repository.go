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

// Clover collection and field names for epics.
const (
	epicsCollection = "epics"

	fieldIdeaID   = "idea_id"
	fieldPriority = "priority"
	fieldSize     = "size"
)

// Compile-time assertion.
var _ port.EpicRepository = (*EpicRepository)(nil)

// EpicRepository is the Clover v2 implementation of
// port.EpicRepository.
type EpicRepository struct{}

// NewEpicRepository returns a Clover-backed epic repository.
func NewEpicRepository() *EpicRepository {
	return &EpicRepository{}
}

// NextEpicSeq atomically increments the "EPIC" counter and returns
// the new value.
func (r *EpicRepository) NextEpicSeq(_ context.Context, dbDir string) (_ int, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return 0, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, countersCollection); err != nil {
		return 0, err
	}

	crit := q.NewQuery(countersCollection).Where(q.Field(fieldCounterName).Eq(domain.EpicIDPrefix))

	doc, err := db.FindFirst(crit)
	if err != nil {
		return 0, fmt.Errorf("find epic counter: %w", err)
	}

	var next int
	if doc == nil {
		next = 1
		newDoc := d.NewDocument()
		newDoc.Set(fieldCounterName, domain.EpicIDPrefix)
		newDoc.Set(fieldCounterVal, next)
		if _, err := db.InsertOne(countersCollection, newDoc); err != nil {
			return 0, fmt.Errorf("insert epic counter: %w", err)
		}
	} else {
		cur := toInt(doc.Get(fieldCounterVal))
		next = cur + 1
		if err := db.UpdateById(countersCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
			doc.Set(fieldCounterVal, next)
			return doc
		}); err != nil {
			return 0, fmt.Errorf("update epic counter: %w", err)
		}
	}

	return next, nil
}

// SaveEpic persists a fully constructed Epic into the epics collection.
func (r *EpicRepository) SaveEpic(_ context.Context, dbDir string, epic domain.Epic) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, epicsCollection); err != nil {
		return err
	}

	doc := d.NewDocument()
	doc.Set(fieldID, epic.ID)
	doc.Set(fieldIdeaID, epic.IdeaID)
	doc.Set(fieldTitle, epic.Title)
	doc.Set(fieldDescription, epic.Description)
	doc.Set(fieldStatus, string(epic.Status))
	doc.Set(fieldPriority, string(epic.Priority))
	doc.Set(fieldSize, int(epic.Size))
	doc.Set(fieldCreatedAt, epic.CreatedAt.Format(time.RFC3339))

	if _, err := db.InsertOne(epicsCollection, doc); err != nil {
		return fmt.Errorf("insert epic: %w", err)
	}

	return nil
}

// GetEpic retrieves a single Epic by its human-readable ID.
func (r *EpicRepository) GetEpic(_ context.Context, dbDir string, id string) (_ *domain.Epic, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(epicsCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", epicsCollection, err)
	}
	if !has {
		return nil, fmt.Errorf("%w: %s", port.ErrEpicNotFound, id)
	}

	doc, err := db.FindFirst(q.NewQuery(epicsCollection).Where(q.Field(fieldID).Eq(id)))
	if err != nil {
		return nil, fmt.Errorf("find epic %s: %w", id, err)
	}
	if doc == nil {
		return nil, fmt.Errorf("%w: %s", port.ErrEpicNotFound, id)
	}

	return docToEpic(doc)
}

// ListEpics returns Epics ordered by creation time. If ideaID is
// non-empty, only Epics belonging to that Idea are returned.
func (r *EpicRepository) ListEpics(_ context.Context, dbDir string, ideaID string) (_ []domain.Epic, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(epicsCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", epicsCollection, err)
	}
	if !has {
		return nil, nil
	}

	query := q.NewQuery(epicsCollection).Sort(q.SortOption{
		Field:     fieldCreatedAt,
		Direction: 1,
	})

	if ideaID != "" {
		query = query.Where(q.Field(fieldIdeaID).Eq(ideaID))
	}

	docs, err := db.FindAll(query)
	if err != nil {
		return nil, fmt.Errorf("list epics: %w", err)
	}

	epics := make([]domain.Epic, 0, len(docs))
	for _, doc := range docs {
		epic, err := docToEpic(doc)
		if err != nil {
			return nil, err
		}
		epics = append(epics, *epic)
	}

	return epics, nil
}

// UpdateEpic replaces an existing Epic by its ID.
func (r *EpicRepository) UpdateEpic(_ context.Context, dbDir string, epic domain.Epic) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	doc, err := db.FindFirst(q.NewQuery(epicsCollection).Where(q.Field(fieldID).Eq(epic.ID)))
	if err != nil {
		return fmt.Errorf("find epic %s: %w", epic.ID, err)
	}
	if doc == nil {
		return fmt.Errorf("%w: %s", port.ErrEpicNotFound, epic.ID)
	}

	return db.UpdateById(epicsCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
		doc.Set(fieldTitle, epic.Title)
		doc.Set(fieldDescription, epic.Description)
		doc.Set(fieldStatus, string(epic.Status))
		doc.Set(fieldPriority, string(epic.Priority))
		doc.Set(fieldSize, int(epic.Size))
		return doc
	})
}

// docToEpic converts a Clover document back into a domain.Epic.
func docToEpic(doc *d.Document) (*domain.Epic, error) {
	id, _ := doc.Get(fieldID).(string)
	ideaID, _ := doc.Get(fieldIdeaID).(string)
	title, _ := doc.Get(fieldTitle).(string)
	desc, _ := doc.Get(fieldDescription).(string)
	statusRaw, _ := doc.Get(fieldStatus).(string)
	priorityRaw, _ := doc.Get(fieldPriority).(string)
	sizeRaw := toInt(doc.Get(fieldSize))
	createdRaw, _ := doc.Get(fieldCreatedAt).(string)

	status, err := domain.ParseStatus(statusRaw)
	if err != nil {
		return nil, fmt.Errorf("decode epic %s status: %w", id, err)
	}

	createdAt, err := time.Parse(time.RFC3339, createdRaw)
	if err != nil {
		return nil, fmt.Errorf("decode epic %s created_at: %w", id, err)
	}

	var priority domain.Priority
	if priorityRaw != "" {
		priority, err = domain.ParsePriority(priorityRaw)
		if err != nil {
			return nil, fmt.Errorf("decode epic %s priority: %w", id, err)
		}
	}

	return &domain.Epic{
		ID:          id,
		IdeaID:      ideaID,
		Title:       title,
		Description: desc,
		Status:      status,
		Priority:    priority,
		Size:        domain.Size(sizeRaw),
		CreatedAt:   createdAt,
	}, nil
}
