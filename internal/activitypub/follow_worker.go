package activitypub

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-fed/httpsig"
	"github.com/riverqueue/river"
)

type HandleFollowArgs struct {
	// ActivityID is the *object* ID of the activity.
	ActivityID string `json:"activity_id"`
}

func (a HandleFollowArgs) Kind() string {
	return "handle-follow"
}

type HandleFollowWorker struct {
	river.WorkerDefaults[HandleFollowArgs]
	pub *Service
}

func (w *HandleFollowWorker) Work(ctx context.Context, job *river.Job[HandleFollowArgs]) error {
	activity, err := w.pub.GetActivityByID(ctx, job.Args.ActivityID)
	if err != nil {
		return fmt.Errorf("failed to get activity: %w", err)
	}

	var data map[string]any
	if err := json.Unmarshal(activity.Data, &data); err != nil {
		return fmt.Errorf("failed to unmarshal activity data: %w", err)
	}

	var actorID string

	actor := data["actor"]

	switch t := actor.(type) {
	case string:
		actorID = t
	case map[string]any:
		id, ok := t["id"].(string)
		if !ok {
			return fmt.Errorf("failed to get actor ID")
		}

		actorID = id
	default:
		return fmt.Errorf("unexpected actor type: %T", t)
	}

	if err := w.createFollower(ctx, activity, actorID); err != nil {
		return err
	}

	return w.acceptFollower(ctx, activity, actorID)
}

func (w *HandleFollowWorker) createFollower(ctx context.Context, activity ActivityRecord, actorID string) error {
	if activity.Type != "Follow" {
		return fmt.Errorf("activity is not a follow")
	}

	follower, err := w.pub.CreateFollower(ctx, actorID, activity.ID)
	if err != nil {
		return fmt.Errorf("failed to create follower: %w", err)
	}

	slog.InfoContext(ctx, "created follower", "id", follower.ActorID)

	return nil
}

type acceptActivity struct {
	Context string `json:"@context"`
	Type    string `json:"type"`
	Actor   string `json:"actor"`
	Object  string `json:"object"`
}

func (w *HandleFollowWorker) acceptFollower(ctx context.Context, activity ActivityRecord, actorID string) error {
	me := GetMe()

	// Post an accept to the actor.
	accept := acceptActivity{
		Context: ActivityStreamsContext,
		Type:    "Accept",
		Actor:   me.ID,
		Object:  activity.ID,
	}

	j, err := json.Marshal(accept)
	if err != nil {
		return fmt.Errorf("failed to marshal accept: %w", err)
	}

	actor, err := GetActor(ctx, actorID)
	if err != nil {
		return fmt.Errorf("error getting actor: %w", err)
	}

	inboxURL := actor.Inbox
	if inboxURL == "" {
		return errors.New("actor has no inbox")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, inboxURL, bytes.NewReader(j))
	if err != nil {
		return fmt.Errorf("error creating accept request: %w", err)
	}

	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Accept", "application/activity+json")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	if err := signJSONLDRequest(me, req, j); err != nil {
		return fmt.Errorf("error signing accept request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error posting accept: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.ErrorContext(ctx, "error closing accept response body", "error", err)
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("error posting accept: %s", resp.Status)
	}

	return nil
}

func newHandleFollowWorker(pub *Service) *HandleFollowWorker {
	return &HandleFollowWorker{
		pub: pub,
	}
}

func signJSONLDRequest(u Actor, r *http.Request, b []byte) error {
	prefs := []httpsig.Algorithm{httpsig.RSA_SHA256}
	digestAlgo := httpsig.DigestSha256
	headers := []string{httpsig.RequestTarget, "date", "digest"}

	signer, _, err := httpsig.NewSigner(prefs, digestAlgo, headers, httpsig.Signature, 0)
	if err != nil {
		return fmt.Errorf("error creating signer: %w", err)
	}

	privateKeyPEM := strings.ReplaceAll(os.Getenv("AP_PRIVATE_KEY_PEM"), `\n`, "\n")
	if privateKeyPEM == "" {
		return errors.New("AP_PRIVATE_KEY_PEM is required")
	}

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return errors.New("error decoding private key")
	}

	pkey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing private key: %w", err)
	}

	rsaKey, ok := pkey.(*rsa.PrivateKey)
	if !ok {
		return errors.New("private key is not an RSA key")
	}

	if err := signer.SignRequest(rsaKey, u.PublicKey.ID, r, b); err != nil {
		return fmt.Errorf("error signing request: %w", err)
	}

	return nil
}
