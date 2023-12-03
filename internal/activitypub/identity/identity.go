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
func (s *Service) GetUserByID(ctx context.Context, id int64) (User, error) {
	query, args, err := s.sql.
		Select(usersFields...).
		From(usersTable).
		Where(squirrel.Eq{usersIDColumn: id}).
		ToSql()
	if err != nil {
		return User{}, fmt.Errorf("could not build query: %w", err)
	}

	var user User
	if err := s.pool.QueryRow(ctx, query, args...).Scan(user.scannableFields()...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}

		return User{}, fmt.Errorf("could not query row: %w", err)
	}

	return user, nil
}

// GetUserByUsername gets a user by username.
func (s *Service) GetUserByUsername(ctx context.Context, username string) (User, error) {
	query, args, err := s.sql.
		Select(usersFields...).
		From(usersTable).
		Where(squirrel.Eq{usersUsernameColumn: username}).
		ToSql()
	if err != nil {
		return User{}, fmt.Errorf("could not build query: %w", err)
	}

	var user User
	if err := s.pool.QueryRow(ctx, query, args...).Scan(user.scannableFields()...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}

		return User{}, fmt.Errorf("could not query row: %w", err)
	}

	return user, nil
}

type keyKind string

const (
	keyKindPublic  keyKind = "public"
	keyKindPrivate keyKind = "private"
)

// ErrSigningKeyNotFound is returned when a signing key is not found.
var ErrSigningKeyNotFound = fmt.Errorf("signing key not found")

// GetPublicKey gets a user's public signing key.
func (s *Service) GetPublicKey(ctx context.Context, userID int64) (SigningKey, error) {
	return s.getSigningKey(ctx, userID, keyKindPublic)
}

// GetPrivateKey gets a user's private signing key.
func (s *Service) GetPrivateKey(ctx context.Context, userID int64) (SigningKey, error) {
	return s.getSigningKey(ctx, userID, keyKindPrivate)
}

func (s *Service) getSigningKey(ctx context.Context, userID int64, kind keyKind) (SigningKey, error) {
	query, args, err := s.sql.
		Select(signingKeysFields...).
		From(signingKeysTable).
		Where(squirrel.Eq{signingKeysKindColumn: kind}).
		Where(squirrel.Eq{signingKeysUserIDColumn: userID}).
		ToSql()
	if err != nil {
		return SigningKey{}, fmt.Errorf("could not build query: %w", err)
	}

	var key SigningKey
	if err := s.pool.QueryRow(ctx, query, args...).Scan(key.scannableFields()...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SigningKey{}, ErrSigningKeyNotFound
		}

		return SigningKey{}, fmt.Errorf("could not query row: %w", err)
	}

	return key, nil
}

// ErrInvalidAPIKey is returned when an API key is invalid.
var ErrInvalidAPIKey = fmt.Errorf("invalid API key")

// ValidateAPIKey validates an API key and returns its associated user.
//
// API keys submitted by clients are of the format "$id.$value" where $id is the
// user ID and $value is the API key value (a random string).
func (s *Service) ValidateAPIKey(ctx context.Context, key string) (User, error) {
	keyparts := strings.SplitN(key, ".", 2)
	if len(keyparts) != 2 {
		return User{}, ErrInvalidAPIKey
	}

	keyid := keyparts[0]
	keyvalue := keyparts[1]

	query, args, err := s.sql.
		Select(apiKeysFields...).
		From(apiKeysTable).
		Where(squirrel.Eq{apiKeysIDColumn: keyid}).
		ToSql()
	if err != nil {
		return User{}, fmt.Errorf("could not build query: %w", err)
	}

	var apikey APIKey
	if err := s.pool.QueryRow(ctx, query, args...).Scan(apikey.scannableFields()...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidAPIKey
		}

		return User{}, fmt.Errorf("could not query row: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(apikey.Value), []byte(keyvalue)) != 1 {
		return User{}, ErrInvalidAPIKey
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

const signingKeysTable = "key_pems"
const signingKeysIDColumn = "id"
const signingKeysUserIDColumn = "user_id"
const signingKeysKindColumn = "kind"
const signingKeysPEMColumn = "pem"
const signingKeysCreatedAtColumn = "created_at"
const signingKeysUpdatedAtColumn = "updated_at"

var signingKeysFields = []string{ //nolint:gochecknoglobals
	signingKeysIDColumn,
	signingKeysUserIDColumn,
	signingKeysKindColumn,
	signingKeysPEMColumn,
	signingKeysCreatedAtColumn,
	signingKeysUpdatedAtColumn,
}

// A SigningKey is a public key in PEM format.
type SigningKey struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Kind      string    `json:"kind"`
	PEM       string    `json:"pem"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (k *SigningKey) scannableFields() []any {
	return []any{
		&k.ID,
		&k.UserID,
		&k.Kind,
		&k.PEM,
		&k.CreatedAt,
		&k.UpdatedAt,
	}
}

const usersTable = "users"
const usersIDColumn = "id"
const usersEmailColumn = "email"
const usersUsernameColumn = "username"
const usersSummaryColumn = "summary"
const usersNameColumn = "name"
const usersImageURLColumn = "image_url"
const usersCreatedAt = "created_at"
const usersUpdatedAt = "updated_at"

var usersFields = []string{ //nolint:gochecknoglobals
	usersIDColumn,
	usersEmailColumn,
	usersUsernameColumn,
	usersSummaryColumn,
	usersNameColumn,
	usersImageURLColumn,
	usersCreatedAt,
	usersUpdatedAt,
}

// A User is a user of the system.
type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Summary   string    `json:"summary"`
	Name      string    `json:"name"`
	ImageURL  string    `json:"image_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetUsername implements the activitypub.Actorish interface.
func (u User) GetUsername() string {
	return u.Username
}

// GetSummary implements the activitypub.Actorish interface.
func (u User) GetSummary() string {
	return u.Summary
}

// GetName implements the activitypub.Actorish interface.
func (u User) GetName() string {
	return u.Name
}

// GetImageURL implements the activitypub.Actorish interface.
func (u User) GetImageURL() string {
	return u.ImageURL
}

func (u *User) scannableFields() []any {
	return []any{
		&u.ID,
		&u.Email,
		&u.Username,
		&u.Summary,
		&u.Name,
		&u.ImageURL,
		&u.CreatedAt,
		&u.UpdatedAt,
	}
}

const apiKeysTable = "api_keys"
const apiKeysIDColumn = "id"
const apiKeysUserIDColumn = "user_id"
const apiKeysValueColumn = "value"
const apiKeysCreatedAtColumn = "created_at"
const apiKeysUpdatedAtColumn = "updated_at"

var apiKeysFields = []string{ //nolint:gochecknoglobals
	apiKeysIDColumn,
	apiKeysUserIDColumn,
	apiKeysValueColumn,
	apiKeysCreatedAtColumn,
	apiKeysUpdatedAtColumn,
}

// An APIKey is a key used to verify a user's API requests.
type APIKey struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a *APIKey) scannableFields() []any {
	return []any{
		&a.ID,
		&a.UserID,
		&a.Value,
		&a.CreatedAt,
		&a.UpdatedAt,
	}
}
