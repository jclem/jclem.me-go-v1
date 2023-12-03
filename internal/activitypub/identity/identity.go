// Package identity provides functions for working with ActivityPub identities.
package identity

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A Service handles identity requests.
type Service struct {
	pool *pgxpool.Pool
	sql  squirrel.StatementBuilderType
}

// ErrUserNotFound is returned when a user is not found.
var ErrUserNotFound = fmt.Errorf("user not found")

// GetUserByID gets a user by ID.
func (s *Service) GetUserByID(ctx context.Context, id int64) (UserRecord, error) {
	query, args, err := s.sql.
		Select(usersFields...).
		From(usersTable).
		Where(squirrel.Eq{usersRecordIDColumn: id}).
		ToSql()
	if err != nil {
		return UserRecord{}, fmt.Errorf("could not build query: %w", err)
	}

	var user UserRecord
	if err := s.pool.QueryRow(ctx, query, args...).Scan(user.scannableFields()...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserRecord{}, ErrUserNotFound
		}

		return UserRecord{}, fmt.Errorf("could not query row: %w", err)
	}

	return user, nil
}

// GetUserByUsername gets a user by username.
func (s *Service) GetUserByUsername(ctx context.Context, username string) (UserRecord, error) {
	query, args, err := s.sql.
		Select(usersFields...).
		From(usersTable).
		Where(squirrel.Eq{usersUsernameColumn: username}).
		ToSql()
	if err != nil {
		return UserRecord{}, fmt.Errorf("could not build query: %w", err)
	}

	var user UserRecord
	if err := s.pool.QueryRow(ctx, query, args...).Scan(user.scannableFields()...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserRecord{}, ErrUserNotFound
		}

		return UserRecord{}, fmt.Errorf("could not query row: %w", err)
	}

	return user, nil
}

// ErrInvalidAPIKey is returned when an API key is invalid.
var ErrInvalidAPIKey = fmt.Errorf("invalid API key")

// ValidateAPIKey validates an API key and returns its associated user.
//
// API keys submitted by clients are of the format "$id.$value" where $id is the
// user ID and $value is the API key value (a random string).
func (s *Service) ValidateAPIKey(ctx context.Context, key string) (UserRecord, error) {
	keyparts := strings.SplitN(key, ".", 2)
	if len(keyparts) != 2 {
		return UserRecord{}, ErrInvalidAPIKey
	}

	keyid := keyparts[0]
	keyvalue := keyparts[1]

	query, args, err := s.sql.
		Select(apiKeysFields...).
		From(apiKeysTable).
		Where(squirrel.Eq{apiKeysRecordIDColumn: keyid}).
		ToSql()
	if err != nil {
		return UserRecord{}, fmt.Errorf("could not build query: %w", err)
	}

	var apikey APIKeyRecord
	if err := s.pool.QueryRow(ctx, query, args...).Scan(apikey.scannableFields()...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserRecord{}, ErrInvalidAPIKey
		}

		return UserRecord{}, fmt.Errorf("could not query row: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(apikey.Value), []byte(keyvalue)) != 1 {
		return UserRecord{}, ErrInvalidAPIKey
	}

	return s.GetUserByID(ctx, apikey.UserID)
}

// NewService returns a new identity service.
func NewService(pool *pgxpool.Pool) (*Service, error) {
	return &Service{
		pool: pool,
		sql:  squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}, nil
}

const usersTable = "users"
const usersRecordIDColumn = "id"
const usersEmailColumn = "email"
const usersUsernameColumn = "username"
const usersCreatedAt = "created_at"
const usersUpdatedAt = "updated_at"

var usersFields = []string{ //nolint:gochecknoglobals
	usersRecordIDColumn,
	usersEmailColumn,
	usersUsernameColumn,
	usersCreatedAt,
	usersUpdatedAt,
}

// A UserRecord is a database record containing a user.
type UserRecord struct {
	RecordID  int64     `json:"record_id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (u *UserRecord) scannableFields() []any {
	return []any{
		&u.RecordID,
		&u.Email,
		&u.Username,
		&u.CreatedAt,
		&u.UpdatedAt,
	}
}

const apiKeysTable = "api_keys"
const apiKeysRecordIDColumn = "id"
const apiKeysUserIDColumn = "user_id"
const apiKeysValueColumn = "value"
const apiKeysCreatedAtColumn = "created_at"
const apiKeysUpdatedAtColumn = "updated_at"

var apiKeysFields = []string{ //nolint:gochecknoglobals
	apiKeysRecordIDColumn,
	apiKeysUserIDColumn,
	apiKeysValueColumn,
	apiKeysCreatedAtColumn,
	apiKeysUpdatedAtColumn,
}

// An APIKeyRecord is a database record containing an API key.
type APIKeyRecord struct {
	RecordID  int64     `json:"record_id"`
	UserID    int64     `json:"user_id"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a *APIKeyRecord) scannableFields() []any {
	return []any{
		&a.RecordID,
		&a.UserID,
		&a.Value,
		&a.CreatedAt,
		&a.UpdatedAt,
	}
}
