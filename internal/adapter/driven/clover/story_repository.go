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

// Clover collection and field names for stories.
const (
	storiesCollection = "stories"

	fieldFeatureID = "feature_id"
)

// Compile-time assertion.
var _ driven.StoryRepository = (*StoryRepository)(nil)

// StoryRepository is the Clover v2 implementation of
// driven.StoryRepository.
type StoryRepository struct{}

// NewStoryRepository returns a Clover-backed story repository.
func NewStoryRepository() *StoryRepository {
	return &StoryRepository{}
}

// NextStorySeq atomically increments the "STORY" counter and returns
// the new value.
func (r *StoryRepository) NextStorySeq(_ context.Context, dbDir string) (_ int, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return 0, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, countersCollection); err != nil {
		return 0, err
	}

	crit := q.NewQuery(countersCollection).Where(q.Field(fieldCounterName).Eq(domain.StoryIDPrefix))

	doc, err := db.FindFirst(crit)
	if err != nil {
		return 0, fmt.Errorf("find story counter: %w", err)
	}

	var next int
	if doc == nil {
		next = 1
		newDoc := d.NewDocument()
		newDoc.Set(fieldCounterName, domain.StoryIDPrefix)
		newDoc.Set(fieldCounterVal, next)
		if _, err := db.InsertOne(countersCollection, newDoc); err != nil {
			return 0, fmt.Errorf("insert story counter: %w", err)
		}
	} else {
		cur := toInt(doc.Get(fieldCounterVal))
		next = cur + 1
		if err := db.UpdateById(countersCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
			doc.Set(fieldCounterVal, next)
			return doc
		}); err != nil {
			return 0, fmt.Errorf("update story counter: %w", err)
		}
	}

	return next, nil
}

// SaveStory persists a fully constructed Story into the stories collection.
func (r *StoryRepository) SaveStory(_ context.Context, dbDir string, story domain.Story) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, storiesCollection); err != nil {
		return err
	}

	doc := d.NewDocument()
	doc.Set(fieldID, story.ID)
	doc.Set(fieldFeatureID, story.FeatureID)
	doc.Set(fieldTitle, story.Title)
	doc.Set(fieldDescription, story.Description)
	doc.Set(fieldStatus, string(story.Status))
	doc.Set(fieldPriority, string(story.Priority))
	doc.Set(fieldSize, int(story.Size))
	doc.Set(fieldCreatedAt, story.CreatedAt.Format(time.RFC3339))

	if _, err := db.InsertOne(storiesCollection, doc); err != nil {
		return fmt.Errorf("insert story: %w", err)
	}

	return nil
}

// GetStory retrieves a single Story by its human-readable ID.
func (r *StoryRepository) GetStory(_ context.Context, dbDir string, id string) (_ *domain.Story, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(storiesCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", storiesCollection, err)
	}
	if !has {
		return nil, fmt.Errorf("%w: %s", driven.ErrStoryNotFound, id)
	}

	doc, err := db.FindFirst(q.NewQuery(storiesCollection).Where(q.Field(fieldID).Eq(id)))
	if err != nil {
		return nil, fmt.Errorf("find story %s: %w", id, err)
	}
	if doc == nil {
		return nil, fmt.Errorf("%w: %s", driven.ErrStoryNotFound, id)
	}

	return docToStory(doc)
}

// ListStories returns Stories ordered by creation time. If featureID
// is non-empty, only Stories belonging to that Feature are returned.
func (r *StoryRepository) ListStories(_ context.Context, dbDir string, featureID string) (_ []domain.Story, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(storiesCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", storiesCollection, err)
	}
	if !has {
		return nil, nil
	}

	query := q.NewQuery(storiesCollection).Sort(q.SortOption{
		Field:     fieldCreatedAt,
		Direction: 1,
	})

	if featureID != "" {
		query = query.Where(q.Field(fieldFeatureID).Eq(featureID))
	}

	docs, err := db.FindAll(query)
	if err != nil {
		return nil, fmt.Errorf("list stories: %w", err)
	}

	stories := make([]domain.Story, 0, len(docs))
	for _, doc := range docs {
		story, err := docToStory(doc)
		if err != nil {
			return nil, err
		}
		stories = append(stories, *story)
	}

	return stories, nil
}

// UpdateStory replaces an existing Story by its ID.
func (r *StoryRepository) UpdateStory(_ context.Context, dbDir string, story domain.Story) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	doc, err := db.FindFirst(q.NewQuery(storiesCollection).Where(q.Field(fieldID).Eq(story.ID)))
	if err != nil {
		return fmt.Errorf("find story %s: %w", story.ID, err)
	}
	if doc == nil {
		return fmt.Errorf("%w: %s", driven.ErrStoryNotFound, story.ID)
	}

	return db.UpdateById(storiesCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
		doc.Set(fieldTitle, story.Title)
		doc.Set(fieldDescription, story.Description)
		doc.Set(fieldStatus, string(story.Status))
		doc.Set(fieldPriority, string(story.Priority))
		doc.Set(fieldSize, int(story.Size))
		return doc
	})
}

// docToStory converts a Clover document back into a domain.Story.
func docToStory(doc *d.Document) (*domain.Story, error) {
	id, _ := doc.Get(fieldID).(string)
	featureID, _ := doc.Get(fieldFeatureID).(string)
	title, _ := doc.Get(fieldTitle).(string)
	desc, _ := doc.Get(fieldDescription).(string)
	statusRaw, _ := doc.Get(fieldStatus).(string)
	priorityRaw, _ := doc.Get(fieldPriority).(string)
	sizeRaw := toInt(doc.Get(fieldSize))
	createdRaw, _ := doc.Get(fieldCreatedAt).(string)

	status, err := domain.ParseStatus(statusRaw)
	if err != nil {
		return nil, fmt.Errorf("decode story %s status: %w", id, err)
	}

	createdAt, err := time.Parse(time.RFC3339, createdRaw)
	if err != nil {
		return nil, fmt.Errorf("decode story %s created_at: %w", id, err)
	}

	var priority domain.Priority
	if priorityRaw != "" {
		priority, err = domain.ParsePriority(priorityRaw)
		if err != nil {
			return nil, fmt.Errorf("decode story %s priority: %w", id, err)
		}
	}

	return &domain.Story{
		ID:          id,
		FeatureID:   featureID,
		Title:       title,
		Description: desc,
		Status:      status,
		Priority:    priority,
		Size:        domain.Size(sizeRaw),
		CreatedAt:   createdAt,
	}, nil
}
