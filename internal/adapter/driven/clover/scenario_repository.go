package clover

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	c "github.com/ostafen/clover/v2"
	d "github.com/ostafen/clover/v2/document"
	q "github.com/ostafen/clover/v2/query"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port/driven"
)

// Clover collection and field names for scenarios.
const (
	scenariosCollection = "scenarios"

	fieldSpecID = "spec_id"
	fieldTags   = "tags"
	fieldGiven  = "given"
	fieldWhen   = "when"
	fieldThen   = "then"
)

// Compile-time assertion.
var _ driven.ScenarioRepository = (*ScenarioRepository)(nil)

// ScenarioRepository is the Clover v2 implementation of
// driven.ScenarioRepository. Scalar fields are stored as native
// Clover values so they can be filtered by query; tags and the
// three step slices are JSON-encoded strings because queries
// never filter on them — only deserialized on read.
type ScenarioRepository struct{}

// NewScenarioRepository returns a Clover-backed scenario repository.
func NewScenarioRepository() *ScenarioRepository {
	return &ScenarioRepository{}
}

// NextScenarioSeq atomically increments the "SCEN" counter and
// returns the new value.
func (r *ScenarioRepository) NextScenarioSeq(_ context.Context, dbDir string) (_ int, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return 0, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, countersCollection); err != nil {
		return 0, err
	}

	crit := q.NewQuery(countersCollection).Where(q.Field(fieldCounterName).Eq(domain.ScenarioIDPrefix))

	doc, err := db.FindFirst(crit)
	if err != nil {
		return 0, fmt.Errorf("find scenario counter: %w", err)
	}

	var next int
	if doc == nil {
		next = 1
		newDoc := d.NewDocument()
		newDoc.Set(fieldCounterName, domain.ScenarioIDPrefix)
		newDoc.Set(fieldCounterVal, next)
		if _, err := db.InsertOne(countersCollection, newDoc); err != nil {
			return 0, fmt.Errorf("insert scenario counter: %w", err)
		}
	} else {
		cur := toInt(doc.Get(fieldCounterVal))
		next = cur + 1
		if err := db.UpdateById(countersCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
			doc.Set(fieldCounterVal, next)
			return doc
		}); err != nil {
			return 0, fmt.Errorf("update scenario counter: %w", err)
		}
	}

	return next, nil
}

// SaveScenario persists a fully constructed Scenario into the
// scenarios collection.
func (r *ScenarioRepository) SaveScenario(_ context.Context, dbDir string, s domain.Scenario) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	if err := ensureCollection(db, scenariosCollection); err != nil {
		return err
	}

	doc, err := scenarioToDoc(s)
	if err != nil {
		return err
	}

	if _, err := db.InsertOne(scenariosCollection, doc); err != nil {
		return fmt.Errorf("insert scenario: %w", err)
	}

	return nil
}

// GetScenario retrieves a single Scenario by its human-readable ID.
func (r *ScenarioRepository) GetScenario(_ context.Context, dbDir string, id string) (_ *domain.Scenario, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(scenariosCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", scenariosCollection, err)
	}
	if !has {
		return nil, fmt.Errorf("%w: %s", driven.ErrScenarioNotFound, id)
	}

	doc, err := db.FindFirst(q.NewQuery(scenariosCollection).Where(q.Field(fieldID).Eq(id)))
	if err != nil {
		return nil, fmt.Errorf("find scenario %s: %w", id, err)
	}
	if doc == nil {
		return nil, fmt.Errorf("%w: %s", driven.ErrScenarioNotFound, id)
	}

	return docToScenario(doc)
}

// ListScenarios returns Scenarios ordered by creation time. If
// specID is non-empty, only Scenarios belonging to that Spec are
// returned.
func (r *ScenarioRepository) ListScenarios(_ context.Context, dbDir string, specID string) (_ []domain.Scenario, retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return nil, fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	has, err := db.HasCollection(scenariosCollection)
	if err != nil {
		return nil, fmt.Errorf("check %q collection: %w", scenariosCollection, err)
	}
	if !has {
		return nil, nil
	}

	query := q.NewQuery(scenariosCollection).Sort(q.SortOption{
		Field:     fieldCreatedAt,
		Direction: 1,
	})

	if specID != "" {
		query = query.Where(q.Field(fieldSpecID).Eq(specID))
	}

	docs, err := db.FindAll(query)
	if err != nil {
		return nil, fmt.Errorf("list scenarios: %w", err)
	}

	scenarios := make([]domain.Scenario, 0, len(docs))
	for _, doc := range docs {
		scen, err := docToScenario(doc)
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, *scen)
	}

	return scenarios, nil
}

// UpdateScenario replaces an existing Scenario by its ID.
func (r *ScenarioRepository) UpdateScenario(_ context.Context, dbDir string, s domain.Scenario) (retErr error) {
	db, err := c.Open(dbDir)
	if err != nil {
		return fmt.Errorf("open clover db: %w", err)
	}
	defer closeDB(db, &retErr)

	doc, err := db.FindFirst(q.NewQuery(scenariosCollection).Where(q.Field(fieldID).Eq(s.ID)))
	if err != nil {
		return fmt.Errorf("find scenario %s: %w", s.ID, err)
	}
	if doc == nil {
		return fmt.Errorf("%w: %s", driven.ErrScenarioNotFound, s.ID)
	}

	tagsJSON, err := marshalJSON(s.Tags)
	if err != nil {
		return err
	}
	givenJSON, err := marshalJSON(s.Given)
	if err != nil {
		return err
	}
	whenJSON, err := marshalJSON(s.When)
	if err != nil {
		return err
	}
	thenJSON, err := marshalJSON(s.Then)
	if err != nil {
		return err
	}

	return db.UpdateById(scenariosCollection, doc.ObjectId(), func(doc *d.Document) *d.Document {
		doc.Set(fieldTitle, s.Title)
		doc.Set(fieldStatus, string(s.Status))
		doc.Set(fieldTags, tagsJSON)
		doc.Set(fieldGiven, givenJSON)
		doc.Set(fieldWhen, whenJSON)
		doc.Set(fieldThen, thenJSON)
		return doc
	})
}

// scenarioToDoc converts a Scenario into a new Clover document.
func scenarioToDoc(s domain.Scenario) (*d.Document, error) {
	tagsJSON, err := marshalJSON(s.Tags)
	if err != nil {
		return nil, err
	}
	givenJSON, err := marshalJSON(s.Given)
	if err != nil {
		return nil, err
	}
	whenJSON, err := marshalJSON(s.When)
	if err != nil {
		return nil, err
	}
	thenJSON, err := marshalJSON(s.Then)
	if err != nil {
		return nil, err
	}

	doc := d.NewDocument()
	doc.Set(fieldID, s.ID)
	doc.Set(fieldSpecID, s.SpecID)
	doc.Set(fieldTitle, s.Title)
	doc.Set(fieldStatus, string(s.Status))
	doc.Set(fieldCreatedAt, s.CreatedAt.Format(time.RFC3339))
	doc.Set(fieldTags, tagsJSON)
	doc.Set(fieldGiven, givenJSON)
	doc.Set(fieldWhen, whenJSON)
	doc.Set(fieldThen, thenJSON)
	return doc, nil
}

// docToScenario converts a Clover document back into a
// domain.Scenario.
func docToScenario(doc *d.Document) (*domain.Scenario, error) {
	id, _ := doc.Get(fieldID).(string)
	specID, _ := doc.Get(fieldSpecID).(string)
	title, _ := doc.Get(fieldTitle).(string)
	statusRaw, _ := doc.Get(fieldStatus).(string)
	createdRaw, _ := doc.Get(fieldCreatedAt).(string)
	tagsRaw, _ := doc.Get(fieldTags).(string)
	givenRaw, _ := doc.Get(fieldGiven).(string)
	whenRaw, _ := doc.Get(fieldWhen).(string)
	thenRaw, _ := doc.Get(fieldThen).(string)

	status, err := domain.ParseStatus(statusRaw)
	if err != nil {
		return nil, fmt.Errorf("decode scenario %s status: %w", id, err)
	}

	createdAt, err := time.Parse(time.RFC3339, createdRaw)
	if err != nil {
		return nil, fmt.Errorf("decode scenario %s created_at: %w", id, err)
	}

	var tags []string
	if err := unmarshalJSON(tagsRaw, &tags); err != nil {
		return nil, fmt.Errorf("decode scenario %s tags: %w", id, err)
	}
	var given []domain.Step
	if err := unmarshalJSON(givenRaw, &given); err != nil {
		return nil, fmt.Errorf("decode scenario %s given: %w", id, err)
	}
	var when []domain.Step
	if err := unmarshalJSON(whenRaw, &when); err != nil {
		return nil, fmt.Errorf("decode scenario %s when: %w", id, err)
	}
	var then []domain.Step
	if err := unmarshalJSON(thenRaw, &then); err != nil {
		return nil, fmt.Errorf("decode scenario %s then: %w", id, err)
	}

	return &domain.Scenario{
		ID:        id,
		SpecID:    specID,
		Title:     title,
		Tags:      tags,
		Given:     given,
		When:      when,
		Then:      then,
		Status:    status,
		CreatedAt: createdAt,
	}, nil
}

// marshalJSON marshals v to a string. A nil or empty slice becomes
// the empty JSON array "[]" rather than "null" so the stored form
// is stable across writes.
func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}
	return string(b), nil
}

// unmarshalJSON unmarshals a stored string into v. An empty string
// is treated as an empty value (no-op).
func unmarshalJSON(raw string, v any) error {
	if raw == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		return fmt.Errorf("unmarshal json %q: %w", raw, err)
	}
	return nil
}
