// Package dispatches represents short posts with one or two sentences of
// content or attached media.
package dispatches

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jclem/jclem.me/internal/www/config"
)

type Service struct {
	db *pgxpool.Pool
}

func (s *Service) Stop() error {
	s.db.Close()
	return nil
}

func New() (*Service, error) {
	pool, err := pgxpool.New(context.Background(), config.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("could not connect to database: %w", err)
	}

	return &Service{db: pool}, nil
}

type imageData struct {
	URL     string `json:"url"`
	Alt     string `json:"alt"`
	Content string `json:"content"`
}

type Dispatch struct {
	ID         int64             `json:"id"`
	Type       string            `json:"type"`
	Data       map[string]string `json:"data"`
	InsertedAt time.Time         `json:"inserted_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

func (s *Service) ListDispatches(ctx context.Context) ([]Dispatch, error) {
	rows, err := s.db.Query(ctx, `SELECT id, type, data, inserted_at, updated_at FROM dispatches ORDER BY inserted_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("could not query dispatches: %w", err)
	}

	var dispatches []Dispatch

	for rows.Next() {
		var dispatch Dispatch
		if err := rows.Scan(&dispatch.ID, &dispatch.Type, &dispatch.Data, &dispatch.InsertedAt, &dispatch.UpdatedAt); err != nil {
			return nil, fmt.Errorf("could not scan Dispatch: %w", err)
		}

		dispatches = append(dispatches, dispatch)
	}

	return dispatches, nil
}

func (s *Service) CreateDispatch(ctx context.Context, content string, alt string, name string, r io.ReadSeeker) (*Dispatch, error) {
	url, err := s.putObject(name, r)
	if err != nil {
		return nil, err
	}

	data := imageData{
		URL:     url,
		Alt:     alt,
		Content: content,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("could not marshal data: %w", err)
	}

	row := s.db.QueryRow(ctx, `INSERT INTO dispatches (type, data) VALUES ($1, $2) RETURNING id, type, data, inserted_at, updated_at`, "image", b)

	var dispatch Dispatch
	if err := row.Scan(&dispatch.ID, &dispatch.Type, &dispatch.Data, &dispatch.InsertedAt, &dispatch.UpdatedAt); err != nil {
		return nil, fmt.Errorf("could not scan Dispatch: %w", err)
	}

	return &dispatch, nil
}

func (s *Service) putObject(name string, r io.ReadSeeker) (string, error) {
	cfg := config.Spaces()
	region := "nyc3"

	s3cfg := aws.NewConfig().
		WithCredentials(
			credentials.NewStaticCredentials(cfg.KeyID, cfg.Secret, ""),
		).
		WithEndpoint(cfg.Endpoint).
		WithS3ForcePathStyle(false).
		WithRegion(region)

	s3ssn, err := session.NewSession(s3cfg)
	if err != nil {
		return "", fmt.Errorf("could not create session: %w", err)
	}

	s3client := s3.New(s3ssn)

	obj := s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String("dispatches/" + name),
		Body:        r,
		ACL:         aws.String("public-read"),
		ContentType: aws.String(getContentType(filepath.Ext(name))),
	}

	if _, err := s3client.PutObject(&obj); err != nil {
		return "", fmt.Errorf("could not put object: %w", err)
	}

	url := "https://" + cfg.Bucket + "." + region + ".cdn.digitaloceanspaces.com/dispatches/" + name

	return url, nil
}

func getContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	default:
		return ""
	}
}
