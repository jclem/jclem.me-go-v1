package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// A Service handles requests to read or modify ActivityPub data.
type Service struct {
	pool  *pgxpool.Pool
	sql   squirrel.StatementBuilderType
	river *river.Client[pgx.Tx]
}

// A Mailbox refers to a specific activity inbox or outbox.
type Mailbox = string

const (
	// Inbox is the inbox for a user.
	Inbox Mailbox = "inbox"

	// Outbox is the outbox for a user.
	Outbox Mailbox = "outbox"
)

// CreateInboxActivity creates a new ActivityPub activity record.
func (s *Service) CreateActivity(ctx context.Context, mailbox Mailbox, context, typ, id string, data []byte) (a ActivityRecord, err error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ActivityRecord{}, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if terr := endTransaction(ctx, tx, err); terr != nil {
			err = terr
		}
	}()

	now := time.Now().UTC()

	query, args, err := s.sql.
		Insert(activitiesTable).
		Columns(activitiesFieldsWritable...).
		Values(mailbox, context, typ, id, data, now, now).
		Suffix("RETURNING " + strings.Join(activitiesFields, ", ")).
		ToSql()
	if err != nil {
		return ActivityRecord{}, fmt.Errorf("failed to build query: %w", err)
	}

	if err := tx.QueryRow(ctx, query, args...).Scan(a.scannableFields()...); err != nil {
		return ActivityRecord{}, fmt.Errorf("failed to insert activity: %w", err)
	}

	switch a.Type {
	case "Follow":
		if _, err := s.river.InsertTx(ctx, tx, HandleFollowArgs{ActivityID: a.ID}, nil); err != nil {
			return ActivityRecord{}, fmt.Errorf("failed to insert follow job: %w", err)
		}
	case "Create":
		if _, err := s.river.InsertTx(ctx, tx, HandleCreateArgs{ActivityID: a.ID}, nil); err != nil {
			return ActivityRecord{}, fmt.Errorf("failed to insert create job: %w", err)
		}
	}

	return a, nil
}

// GetActivityByID gets an activity by its object ID.
func (s *Service) GetActivityByID(ctx context.Context, id string) (ActivityRecord, error) {
	query, args, err := s.sql.
		Select(activitiesFields...).
		From(activitiesTable).
		Where(squirrel.Eq{activitiesIDColumn: id}).
		ToSql()
	if err != nil {
		return ActivityRecord{}, fmt.Errorf("failed to build query: %w", err)
	}

	var a ActivityRecord
	if err := s.pool.QueryRow(ctx, query, args...).Scan(a.scannableFields()...); err != nil {
		return ActivityRecord{}, fmt.Errorf("failed to get activity by ID: %w", err)
	}

	return a, nil
}

// CreateFollower creates a new follower record.
func (s *Service) CreateFollower(ctx context.Context, actorID, activityID string) (FollowerRecord, error) {
	now := time.Now().UTC()

	var f FollowerRecord

	query, args, err := s.sql.
		Insert(followersTable).
		Columns(followersFieldsWritable...).
		Values(actorID, activityID, now, now).
		Suffix("RETURNING " + strings.Join(followersFields, ", ")).
		ToSql()
	if err != nil {
		return FollowerRecord{}, fmt.Errorf("failed to build query: %w", err)
	}

	if err := s.pool.QueryRow(ctx, query, args...).Scan(f.scannableFields()...); err != nil {
		return FollowerRecord{}, fmt.Errorf("failed to insert follower: %w", err)
	}

	return f, nil
}

// ListPublicOutbox lists all public outbox activity.
func (s *Service) ListPublicOutbox(ctx context.Context) ([]ActivityRecord, error) {
	query, args, err := s.sql.
		Select(activitiesFields...).
		From(activitiesTable).
		Where(squirrel.Eq{activitiesMailboxColumn: Outbox}).
		Where(squirrel.Eq{activitiesTypeColumn: "Create"}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query activities: %w", err)
	}

	var activities []ActivityRecord

	for rows.Next() {
		var a ActivityRecord
		if err := rows.Scan(a.scannableFields()...); err != nil {
			return nil, fmt.Errorf("failed to scan activity: %w", err)
		}

		activities = append(activities, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate activities: %w", err)
	}

	var publicActivities []ActivityRecord

	for _, a := range activities {
		type publicActivity struct {
			To []string `json:"to"`
		}

		var pa publicActivity
		if err := json.Unmarshal(a.Data, &pa); err != nil {
			return nil, fmt.Errorf("failed to unmarshal activity: %w", err)
		}

		if len(pa.To) == 0 {
			continue
		}

		if slices.Contains(pa.To, ActivityStreamsContext+"#Public") {
			publicActivities = append(publicActivities, a)
		}
	}

	return publicActivities, nil
}

// ListFollowers lists all followers.
func (s *Service) ListFollowers(ctx context.Context) ([]FollowerRecord, error) {
	query, args, err := s.sql.
		Select(followersFields...).
		From(followersTable).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query followers: %w", err)
	}

	var followers []FollowerRecord

	for rows.Next() {
		var f FollowerRecord
		if err := rows.Scan(f.scannableFields()...); err != nil {
			return nil, fmt.Errorf("failed to scan follower: %w", err)
		}

		followers = append(followers, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate followers: %w", err)
	}

	return followers, nil
}

// CreateNote creates a new note record.
func (s *Service) CreateNote(ctx context.Context, activityID string, note Note) (NoteRecord, error) {
	now := time.Now().UTC()

	var n NoteRecord

	query, args, err := s.sql.
		Insert(notesTable).
		Columns(notesFieldsWritable...).
		Values(activityID, note.ID, note.Content, note.Published, note.To, note.Cc, now, now).
		Suffix("RETURNING " + strings.Join(notesFields, ", ")).
		ToSql()
	if err != nil {
		return NoteRecord{}, fmt.Errorf("failed to build query: %w", err)
	}

	if err := s.pool.QueryRow(ctx, query, args...).Scan(n.scannableFields()...); err != nil {
		return NoteRecord{}, fmt.Errorf("failed to insert note: %w", err)
	}

	return n, nil
}

// NewService creates a new Service.
func NewService(ctx context.Context, connString string) (*Service, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	s := Service{
		pool: pool,
		sql:  squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}

	workers := river.NewWorkers()
	if err := river.AddWorkerSafely(workers, newHandleFollowWorker(&s)); err != nil {
		return nil, fmt.Errorf("failed to add worker: %w", err)
	}

	if err := river.AddWorkerSafely(workers, newHandleCreateWorker(&s)); err != nil {
		return nil, fmt.Errorf("failed to add worker: %w", err)
	}

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create river client: %w", err)
	}

	if err := riverClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start river client: %w", err)
	}

	s.river = riverClient

	return &s, nil
}

const activitiesTable = "activities"
const activitiesRecordIDColumn = "id"
const activitiesMailboxColumn = "mailbox"
const activitiesContextColumn = "activity_context"
const activitiesTypeColumn = "activity_type"
const activitiesIDColumn = "activity_id"
const activitiesDataColumn = "data"
const activitiesCreatedAtColumn = "created_at"
const activitiesUpdatedAtColumn = "updated_at"

var activitiesFields = []string{ //nolint:gochecknoglobals
	activitiesRecordIDColumn,
	activitiesMailboxColumn,
	activitiesContextColumn,
	activitiesTypeColumn,
	activitiesIDColumn,
	activitiesDataColumn,
	activitiesCreatedAtColumn,
	activitiesUpdatedAtColumn}

var activitiesFieldsWritable = []string{ //nolint:gochecknoglobals
	activitiesMailboxColumn,
	activitiesContextColumn,
	activitiesTypeColumn,
	activitiesIDColumn,
	activitiesDataColumn,
	activitiesCreatedAtColumn,
	activitiesUpdatedAtColumn}

// An ActivityRecord is a database record containing an ActivityPub activity.
// SEE: https://www.w3.org/TR/activitystreams-vocabulary/#dfn-activity
type ActivityRecord struct {
	RecordID  int64     `json:"record_id"`
	Mailbox   Mailbox   `json:"mailbox"`
	Context   string    `json:"@context"`
	Type      string    `json:"type"`
	ID        string    `json:"id"`
	Data      []byte    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a *ActivityRecord) scannableFields() []any {
	return []any{
		&a.RecordID,
		&a.Mailbox,
		&a.Context,
		&a.Type,
		&a.ID,
		&a.Data,
		&a.CreatedAt,
		&a.UpdatedAt,
	}
}

func ActivityRecordToActivity[T any](r ActivityRecord) (*Activity[T], error) {
	var a Activity[T]
	if err := json.Unmarshal(r.Data, &a); err != nil {
		return nil, fmt.Errorf("failed to unmarshal activity: %w", err)
	}

	return &a, nil
}

const followersTable = "followers"
const followersRecordIDColumn = "id"
const followersActorIDColumn = "actor_id"
const followersActivityIDColumn = "activity_id"
const followersCreatedAtColumn = "created_at"
const followersUpdatedAtColumn = "updated_at"

var followersFields = []string{ //nolint:gochecknoglobals
	followersRecordIDColumn,
	followersActorIDColumn,
	followersActivityIDColumn,
	followersCreatedAtColumn,
	followersUpdatedAtColumn}

var followersFieldsWritable = []string{ //nolint:gochecknoglobals
	followersActorIDColumn,
	followersActivityIDColumn,
	followersCreatedAtColumn,
	followersUpdatedAtColumn}

// An FollowerRecord is a database record containing a follower of a user.
type FollowerRecord struct {
	RecordID   int64     `json:"record_id"`
	ActorID    string    `json:"actor_id"`
	ActivityID string    `json:"activity_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (a *FollowerRecord) scannableFields() []any {
	return []any{
		&a.RecordID,
		&a.ActorID,
		&a.ActivityID,
		&a.CreatedAt,
		&a.UpdatedAt,
	}
}

const notesTable = "notes"
const notesRecordIDColumn = "id"
const notesActivityIDColumn = "activity_id"
const notesObjectIDColumn = "object_id"
const notesContentColumn = "content"
const notesPublishedColumn = "published"
const notesToColumn = "to_iri"
const notesCcColumn = "cc_iri"
const notesCreatedAtColumn = "created_at"
const notesUpdatedAtColumn = "updated_at"

var notesFields = []string{ //nolint:gochecknoglobals
	notesRecordIDColumn,
	notesActivityIDColumn,
	notesObjectIDColumn,
	notesContentColumn,
	notesPublishedColumn,
	notesToColumn,
	notesCcColumn,
	notesCreatedAtColumn,
	notesUpdatedAtColumn}

var notesFieldsWritable = []string{ //nolint:gochecknoglobals
	notesActivityIDColumn,
	notesObjectIDColumn,
	notesContentColumn,
	notesPublishedColumn,
	notesToColumn,
	notesCcColumn,
	notesCreatedAtColumn,
	notesUpdatedAtColumn}

// An NoteRecord is a database record containing a note.
type NoteRecord struct {
	RecordID   int64
	ActivityID string
	ObjectID   string
	Content    string
	Published  time.Time
	To         []string
	Cc         []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (n *NoteRecord) ToNote() *Note {
	return &Note{
		Context:      NewContext([]string{ActivityStreamsContext}),
		Type:         "Note",
		ID:           n.ObjectID,
		AttributedTo: GetMe().ID,
		Content:      n.Content,
		Published:    n.Published.Format(time.RFC3339),
		To:           n.To,
		Cc:           n.Cc,
	}
}

func (n *NoteRecord) scannableFields() []any {
	return []any{
		&n.RecordID,
		&n.ActivityID,
		&n.ObjectID,
		&n.Content,
		&n.Published,
		&n.To,
		&n.Cc,
		&n.CreatedAt,
		&n.UpdatedAt,
	}
}

func endTransaction(ctx context.Context, tx pgx.Tx, err error) error {
	if err != nil {
		if rerr := tx.Rollback(ctx); rerr != nil {
			// On a failed rollback, we don't want to return the rollback error,
			// but the original error will instead be used as the cause by the
			// caller.
			slog.Error("failed to rollback transaction", "error", rerr)
		}
	} else {
		if cerr := tx.Commit(ctx); cerr != nil {
			slog.Error("failed to commit transaction", "error", cerr)

			return fmt.Errorf("failed to commit transaction: %w", cerr)
		}
	}

	return nil
}
