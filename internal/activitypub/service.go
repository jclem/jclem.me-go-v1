package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jclem/jclem.me/internal/activitypub/identity"
	"github.com/jclem/jclem.me/internal/database"
	"github.com/jclem/jclem.me/internal/www/config"
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
func (s *Service) CreateActivity(ctx context.Context, userRecordID database.ULID, mailbox Mailbox, context, typ, id string, data []byte) (ar ActivityRecord, err error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ActivityRecord{}, fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if terr := endTransaction(ctx, tx, err); terr != nil {
			err = terr
		}
	}()

	ar, err = s.insertActivityRecord(ctx, tx, userRecordID, mailbox, context, typ, id, data)
	if err != nil {
		return ActivityRecord{}, fmt.Errorf("failed to create activity record: %w", err)
	}

	if mailbox == Inbox {
		if err := s.handleInbox(ctx, tx, userRecordID, ar); err != nil {
			return ActivityRecord{}, fmt.Errorf("failed to handle inbox: %w", err)
		}
	} else {
		if err := s.handleOutbox(ctx, tx, userRecordID, ar); err != nil {
			return ActivityRecord{}, fmt.Errorf("failed to handle outbox: %w", err)
		}
	}

	return ar, nil
}

var acceptableActivities = []string{followActivityType, undoActivityType} //nolint:gochecknoglobals

func (s *Service) handleInbox(ctx context.Context, tx pgx.Tx, userRecordID database.ULID, ar ActivityRecord) error {
	if !slices.Contains(acceptableActivities, ar.Type) {
		slog.InfoContext(ctx, "ignoring non-follow activity", "activity_id", ar, "activity_type", ar.Type)
		return nil
	}

	if _, err := s.river.InsertTx(ctx, tx, HandleInboxArgs{UserRecordID: userRecordID, ActivityID: ar.ID}, nil); err != nil {
		return fmt.Errorf("failed to insert follow job: %w", err)
	}

	return nil
}

func (s *Service) handleOutbox(ctx context.Context, tx pgx.Tx, userRecordID database.ULID, ar ActivityRecord) error {
	if ar.Type != createActivityType {
		return fmt.Errorf("invalid activity type: %s", ar.Type)
	}

	var ao Activity[Note]
	if err := json.Unmarshal(ar.Data, &ao); err != nil {
		return fmt.Errorf("failed to unmarshal activity data: %w", err)
	}

	if ao.Object.Type != "Note" {
		return fmt.Errorf("invalid object type: %s", ao.Object.Type)
	}

	_, err := s.insertNote(ctx, tx, userRecordID, ao.ID, ao.Object)
	if err != nil {
		return fmt.Errorf("failed to create note: %w", err)
	}

	followers, err := s.ListFollowers(ctx, userRecordID)
	if err != nil {
		return fmt.Errorf("failed to list followers: %w", err)
	}

	for _, follower := range followers {
		if _, err := s.river.InsertTx(ctx, tx, HandleOutboxArgs{ActivityID: ao.ID, FollowerID: follower.ActorID, UserRecordID: userRecordID}, nil); err != nil {
			return fmt.Errorf("failed to insert outbox job: %w", err)
		}
	}

	return nil
}

func (s *Service) insertActivityRecord(ctx context.Context, tx pgx.Tx, userRecordID database.ULID, mailbox Mailbox, context, typ, id string, data []byte) (ActivityRecord, error) {
	now := time.Now().UTC()

	// Extract generated ULID from the activity object's object ID, which is a URL.
	// The ULID is the last segment of the URL.
	parts := strings.Split(id, "/")
	activityRecordID := parts[len(parts)-1]

	query, args, err := s.sql.
		Insert(activitiesTable).
		Columns(activitiesFieldsWritable...).
		Values(activityRecordID, userRecordID, mailbox, context, typ, id, data, now, now).
		Suffix("RETURNING " + strings.Join(activitiesFields, ", ")).
		ToSql()
	if err != nil {
		return ActivityRecord{}, fmt.Errorf("failed to build query: %w", err)
	}

	var a ActivityRecord
	if err := tx.QueryRow(ctx, query, args...).Scan(a.scannableFields()...); err != nil {
		return ActivityRecord{}, fmt.Errorf("failed to insert activity: %w", err)
	}

	return a, nil
}

func (s *Service) insertNote(ctx context.Context, tx pgx.Tx, userRecordID database.ULID, activityID string, note Note) (NoteRecord, error) {
	now := time.Now().UTC()

	var n NoteRecord

	// Extract generated ULID from the note object's object ID, which is a URL.
	// The ULID is the last segment of the URL.
	parts := strings.Split(note.ID, "/")
	noteRecordID := parts[len(parts)-1]

	query, args, err := s.sql.
		Insert(notesTable).
		Columns(notesFieldsWritable...).
		Values(noteRecordID, userRecordID, activityID, note.ID, note.Content, note.Published, note.To, note.Cc, now, now).
		Suffix("RETURNING " + strings.Join(notesFields, ", ")).
		ToSql()
	if err != nil {
		return NoteRecord{}, fmt.Errorf("failed to build query: %w", err)
	}

	if err := tx.QueryRow(ctx, query, args...).Scan(n.scannableFields()...); err != nil {
		return NoteRecord{}, fmt.Errorf("failed to insert note: %w", err)
	}

	return n, nil
}

// ErrNoteNotFound is returned when a note is not found.
var ErrNoteNotFound = errors.New("note not found")

// GetNoteByID gets a note by its record ID.
func (s *Service) GetNoteByID(ctx context.Context, id database.ULID) (NoteRecord, error) {
	query, args, err := s.sql.
		Select(notesFields...).
		From(notesTable).
		Where(squirrel.Eq{notesRecordIDColumn: id}).
		ToSql()
	if err != nil {
		return NoteRecord{}, fmt.Errorf("failed to build query: %w", err)
	}

	var n NoteRecord
	if err := s.pool.QueryRow(ctx, query, args...).Scan(n.scannableFields()...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NoteRecord{}, ErrNoteNotFound
		}

		return NoteRecord{}, fmt.Errorf("failed to get note by ID: %w", err)
	}

	return n, nil
}

// ErrActivityNotFound is returned when an activity is not found.
var ErrActivityNotFound = errors.New("activity not found")

// GetActivityByID gets an activity by its object ID.
func (s *Service) GetActivityByID(ctx context.Context, userRecordID database.ULID, id string) (ActivityRecord, error) {
	query, args, err := s.sql.
		Select(activitiesFields...).
		From(activitiesTable).
		Where(squirrel.Eq{activitiesUserIDColumn: userRecordID}).
		Where(squirrel.Eq{activitiesIDColumn: id}).
		ToSql()
	if err != nil {
		return ActivityRecord{}, fmt.Errorf("failed to build query: %w", err)
	}

	var a ActivityRecord
	if err := s.pool.QueryRow(ctx, query, args...).Scan(a.scannableFields()...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ActivityRecord{}, ErrActivityNotFound
		}

		return ActivityRecord{}, fmt.Errorf("failed to get activity by ID: %w", err)
	}

	return a, nil
}

// CreateFollower creates a new follower record.
func (s *Service) CreateFollower(ctx context.Context, userRecordID database.ULID, actorID, activityID string) (FollowerRecord, error) {
	now := time.Now().UTC()

	var f FollowerRecord

	query, args, err := s.sql.
		Insert(followersTable).
		Columns(followersFieldsWritable...).
		Values(userRecordID, actorID, activityID, now, now).
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

// DeleteFollower deletes a follower record.
func (s *Service) DeleteFollower(ctx context.Context, userRecordID database.ULID, actorID string) error {
	query, args, err := s.sql.
		Delete(followersTable).
		Where(squirrel.Eq{followersUserIDColumn: userRecordID}).
		Where(squirrel.Eq{followersActorIDColumn: actorID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	if _, err := s.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to delete follower: %w", err)
	}

	return nil
}

// ListPublicOutbox lists all public outbox activity.
func (s *Service) ListPublicOutbox(ctx context.Context, userRecordID database.ULID) ([]ActivityRecord, error) {
	query, args, err := s.sql.
		Select(activitiesFields...).
		From(activitiesTable).
		Where(squirrel.Eq{activitiesUserIDColumn: userRecordID}).
		Where(squirrel.Eq{activitiesMailboxColumn: Outbox}).
		Where(squirrel.Eq{activitiesTypeColumn: createActivityType}).
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
func (s *Service) ListFollowers(ctx context.Context, userRecordID database.ULID) ([]FollowerRecord, error) {
	query, args, err := s.sql.
		Select(followersFields...).
		From(followersTable).
		Where(squirrel.Eq{followersUserIDColumn: userRecordID}).
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

// NewService creates a new Service.
func NewService(ctx context.Context, pool *pgxpool.Pool, id *identity.Service) (*Service, error) {
	s := Service{
		pool: pool,
		sql:  squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, newHandleFollowWorker(&s, id))
	river.AddWorker(workers, newHandleOutboxWorker(&s, id))

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create river client: %w", err)
	}

	if config.RunWorkers() {
		if err := riverClient.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start river client: %w", err)
		}
	}

	s.river = riverClient

	return &s, nil
}

const activitiesTable = "activities"
const activitiesRecordIDColumn = "id"
const activitiesUserIDColumn = "user_id"
const activitiesMailboxColumn = "mailbox"
const activitiesContextColumn = "activity_context"
const activitiesTypeColumn = "activity_type"
const activitiesIDColumn = "activity_id"
const activitiesDataColumn = "data"
const activitiesCreatedAtColumn = "created_at"
const activitiesUpdatedAtColumn = "updated_at"

var activitiesFields = []string{ //nolint:gochecknoglobals
	activitiesRecordIDColumn,
	activitiesUserIDColumn,
	activitiesMailboxColumn,
	activitiesContextColumn,
	activitiesTypeColumn,
	activitiesIDColumn,
	activitiesDataColumn,
	activitiesCreatedAtColumn,
	activitiesUpdatedAtColumn}

var activitiesFieldsWritable = activitiesFields //nolint:gochecknoglobals

// An ActivityRecord is a database record containing an ActivityPub activity.
// SEE: https://www.w3.org/TR/activitystreams-vocabulary/#dfn-activity
type ActivityRecord struct {
	RecordID  database.ULID `json:"record_id"`
	UserID    database.ULID `json:"user_id"`
	Mailbox   Mailbox       `json:"mailbox"`
	Context   string        `json:"@context"`
	Type      string        `json:"type"`
	ID        string        `json:"id"`
	Data      []byte        `json:"data"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

func (a *ActivityRecord) scannableFields() []any {
	return []any{
		&a.RecordID,
		&a.UserID,
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
const followersUserIDColumn = "user_id"
const followersActorIDColumn = "actor_id"
const followersActivityIDColumn = "activity_id"
const followersCreatedAtColumn = "created_at"
const followersUpdatedAtColumn = "updated_at"

var followersFields = []string{ //nolint:gochecknoglobals
	followersRecordIDColumn,
	followersUserIDColumn,
	followersActorIDColumn,
	followersActivityIDColumn,
	followersCreatedAtColumn,
	followersUpdatedAtColumn}

var followersFieldsWritable = []string{ //nolint:gochecknoglobals
	followersUserIDColumn,
	followersActorIDColumn,
	followersActivityIDColumn,
	followersCreatedAtColumn,
	followersUpdatedAtColumn}

// An FollowerRecord is a database record containing a follower of a user.
type FollowerRecord struct {
	RecordID   database.ULID `json:"record_id"`
	UserID     database.ULID `json:"user_id"`
	ActorID    string        `json:"actor_id"`
	ActivityID string        `json:"activity_id"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

func (a *FollowerRecord) scannableFields() []any {
	return []any{
		&a.RecordID,
		&a.UserID,
		&a.ActorID,
		&a.ActivityID,
		&a.CreatedAt,
		&a.UpdatedAt,
	}
}

const notesTable = "notes"
const notesRecordIDColumn = "id"
const notesUserIDColumn = "user_id"
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
	notesUserIDColumn,
	notesActivityIDColumn,
	notesObjectIDColumn,
	notesContentColumn,
	notesPublishedColumn,
	notesToColumn,
	notesCcColumn,
	notesCreatedAtColumn,
	notesUpdatedAtColumn}

var notesFieldsWritable = notesFields //nolint:gochecknoglobals

// An NoteRecord is a database record containing a note.
type NoteRecord struct {
	RecordID   database.ULID `json:"id"`
	UserID     database.ULID `json:"user_id"`
	ActivityID string        `json:"activity_id"`
	ObjectID   string        `json:"object_id"`
	Content    string        `json:"content"`
	Published  time.Time     `json:"published"`
	To         []string      `json:"to"`
	Cc         []string      `json:"cc"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

func (n *NoteRecord) ToNote(user Actor) *Note {
	return &Note{
		Context:      NewContext([]string{ActivityStreamsContext}),
		Type:         "Note",
		ID:           n.ObjectID,
		AttributedTo: user.ID,
		Content:      n.Content,
		Published:    n.Published.Format(time.RFC3339),
		To:           n.To,
		Cc:           n.Cc,
	}
}

func (n *NoteRecord) scannableFields() []any {
	return []any{
		&n.RecordID,
		&n.UserID,
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
