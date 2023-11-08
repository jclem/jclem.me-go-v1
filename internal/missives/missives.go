// Package missives represents short posts with one or two sentences of content
// or attached media.
package missives

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
	"github.com/jackc/pgx/v5"
	"github.com/jclem/jclem.me/internal/www/config"
)

type Service struct {
	conn *pgx.Conn
}

func (s *Service) Stop() error {
	if err := s.conn.Close(context.Background()); err != nil {
		return fmt.Errorf("could not close database connection: %w", err)
	}

	return nil
}

func New() (*Service, error) {
	conn, err := pgx.Connect(context.Background(), config.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("could not connect to database: %w", err)
	}

	return &Service{conn: conn}, nil
}

type imageData struct {
	URL     string `json:"url"`
	Alt     string `json:"alt"`
	Content string `json:"content"`
}

type Missive struct {
	ID         int64             `json:"id"`
	Type       string            `json:"type"`
	Data       map[string]string `json:"data"`
	InsertedAt time.Time         `json:"inserted_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

func (s *Service) ListMissives(ctx context.Context) ([]Missive, error) {
	rows, err := s.conn.Query(ctx, `SELECT id, type, data, inserted_at, updated_at FROM missives ORDER BY inserted_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("could not query missives: %w", err)
	}

	var missives []Missive

	for rows.Next() {
		var missive Missive
		if err := rows.Scan(&missive.ID, &missive.Type, &missive.Data, &missive.InsertedAt, &missive.UpdatedAt); err != nil {
			return nil, fmt.Errorf("could not scan missive: %w", err)
		}

		missives = append(missives, missive)
	}

	return missives, nil
}

func (s *Service) CreateMissive(ctx context.Context, content string, alt string, name string, r io.ReadSeeker) (*Missive, error) {
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

	row := s.conn.QueryRow(ctx, `INSERT INTO missives (type, data) VALUES ($1, $2) RETURNING id, type, data, inserted_at, updated_at`, "image", b)

	var missive Missive
	if err := row.Scan(&missive.ID, &missive.Type, &missive.Data, &missive.InsertedAt, &missive.UpdatedAt); err != nil {
		return nil, fmt.Errorf("could not scan missive: %w", err)
	}

	return &missive, nil
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
		Key:         aws.String("missives/" + name),
		Body:        r,
		ACL:         aws.String("public-read"),
		ContentType: aws.String(getContentType(filepath.Ext(name))),
	}

	if _, err := s3client.PutObject(&obj); err != nil {
		return "", fmt.Errorf("could not put object: %w", err)
	}

	url := "https://" + cfg.Bucket + "." + region + ".cdn.digitaloceanspaces.com/missives/" + name

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
