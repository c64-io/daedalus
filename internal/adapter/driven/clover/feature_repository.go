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

// Clover collection and field names for features.
const (
	featuresCollection = "features"

	fieldEpicID = "epic_id"
)

// Compile-time assertion.
var _ driven.FeatureRepository = (*FeatureRepository)(nil)

// FeatureRepository is the Clover v2 implementation of
// driven.FeatureRepository.
type FeatureRepository struct{}

// NewFeatureRepository returns a Clover-backed feature repository.
func NewFeatureRepository() *FeatureRepository {
	return &FeatureRepository{}
}

// NextFeatureSeq atomically increments the "FEAT" counter and returns
// the new value.
func (r *FeatureRepository) NextFeatureSeq(_ context.Context, dbDir string) (_ int, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return 0, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, countersCollection); err != nil {
		return 0, err
	}

	crit := q.NewQuery(countersCollection).Where(q.Field(fieldCounterName).Eq(domain.FeatureIDPrefix))

	doc, err := db.FindFirst(crit)
	if err != nil {
		return 0, fmt.Errorf("find feature counter: %w", err)
	}

	var next int
	if doc == nil {
		next = 1
		newDoc := d.NewDocument()
		newDoc.Set(fieldCounterName, domain.FeatureIDPrefix)
		newDoc.Set(fieldCounterVal, next)
		if _, err := db.InsertOne(countersCollection, newDoc); err != nil {
			return 0, fmt.Errorf("insert feature counter: %w", err)
		}
	} else {
		cur := toInt(doc.Get(fieldCounterVal))
		next = cur + 1
		if err := db.UpdateById(countersCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
			doc.Set(fieldCounterVal, next)
			return doc
		}); err != nil {
			return 0, fmt.Errorf("update feature counter: %w", err)
		}
	}

	return next, nil
}

// SaveFeature persists a fully constructed Feature into the features collection.
func (r *FeatureRepository) SaveFeature(_ context.Context, dbDir string, feature domain.Feature) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, featuresCollection); err != nil {
		return err
	}

	doc := d.NewDocument()
	doc.Set(fieldID, feature.ID)
	doc.Set(fieldEpicID, feature.EpicID)
	doc.Set(fieldTitle, feature.Title)
	doc.Set(fieldDescription, feature.Description)
	doc.Set(fieldStatus, string(feature.Status))
	doc.Set(fieldPriority, string(feature.Priority))
	doc.Set(fieldSize, int(feature.Size))
	doc.Set(fieldCreatedAt, feature.CreatedAt.Format(time.RFC3339))

	if _, err := db.InsertOne(featuresCollection, doc); err != nil {
		return fmt.Errorf("insert feature: %w", err)
	}

	return nil
}

// GetFeature retrieves a single Feature by its human-readable ID.
func (r *FeatureRepository) GetFeature(_ context.Context, dbDir string, id string) (_ *domain.Feature, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(featuresCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", featuresCollection, err)
	}
	if !has {
		return nil, fmt.Errorf("%w: %s", driven.ErrFeatureNotFound, id)
	}

	doc, err := db.FindFirst(q.NewQuery(featuresCollection).Where(q.Field(fieldID).Eq(id)))
	if err != nil {
		return nil, fmt.Errorf("find feature %s: %w", id, err)
	}
	if doc == nil {
		return nil, fmt.Errorf("%w: %s", driven.ErrFeatureNotFound, id)
	}

	return docToFeature(doc)
}

// ListFeatures returns Features ordered by creation time. If epicID
// is non-empty, only Features belonging to that Epic are returned.
func (r *FeatureRepository) ListFeatures(_ context.Context, dbDir string, epicID string) (_ []domain.Feature, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(featuresCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", featuresCollection, err)
	}
	if !has {
		return nil, nil
	}

	query := q.NewQuery(featuresCollection).Sort(q.SortOption{
		Field:     fieldCreatedAt,
		Direction: 1,
	})

	if epicID != "" {
		query = query.Where(q.Field(fieldEpicID).Eq(epicID))
	}

	docs, err := db.FindAll(query)
	if err != nil {
		return nil, fmt.Errorf("list features: %w", err)
	}

	features := make([]domain.Feature, 0, len(docs))
	for _, doc := range docs {
		feature, err := docToFeature(doc)
		if err != nil {
			return nil, err
		}
		features = append(features, *feature)
	}

	return features, nil
}

// UpdateFeature replaces an existing Feature by its ID.
func (r *FeatureRepository) UpdateFeature(_ context.Context, dbDir string, feature domain.Feature) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	doc, err := db.FindFirst(q.NewQuery(featuresCollection).Where(q.Field(fieldID).Eq(feature.ID)))
	if err != nil {
		return fmt.Errorf("find feature %s: %w", feature.ID, err)
	}
	if doc == nil {
		return fmt.Errorf("%w: %s", driven.ErrFeatureNotFound, feature.ID)
	}

	return db.UpdateById(featuresCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
		doc.Set(fieldTitle, feature.Title)
		doc.Set(fieldDescription, feature.Description)
		doc.Set(fieldStatus, string(feature.Status))
		doc.Set(fieldPriority, string(feature.Priority))
		doc.Set(fieldSize, int(feature.Size))
		return doc
	})
}

// docToFeature converts a Clover document back into a domain.Feature.
func docToFeature(doc *d.Document) (*domain.Feature, error) {
	id, _ := doc.Get(fieldID).(string)
	epicID, _ := doc.Get(fieldEpicID).(string)
	title, _ := doc.Get(fieldTitle).(string)
	desc, _ := doc.Get(fieldDescription).(string)
	statusRaw, _ := doc.Get(fieldStatus).(string)
	priorityRaw, _ := doc.Get(fieldPriority).(string)
	sizeRaw := toInt(doc.Get(fieldSize))
	createdRaw, _ := doc.Get(fieldCreatedAt).(string)

	status, err := domain.ParseStatus(statusRaw)
	if err != nil {
		return nil, fmt.Errorf("decode feature %s status: %w", id, err)
	}

	createdAt, err := time.Parse(time.RFC3339, createdRaw)
	if err != nil {
		return nil, fmt.Errorf("decode feature %s created_at: %w", id, err)
	}

	var priority domain.Priority
	if priorityRaw != "" {
		priority, err = domain.ParsePriority(priorityRaw)
		if err != nil {
			return nil, fmt.Errorf("decode feature %s priority: %w", id, err)
		}
	}

	return &domain.Feature{
		ID:          id,
		EpicID:      epicID,
		Title:       title,
		Description: desc,
		Status:      status,
		Priority:    priority,
		Size:        domain.Size(sizeRaw),
		CreatedAt:   createdAt,
	}, nil
}
