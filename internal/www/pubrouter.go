package www

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	ap "github.com/jclem/jclem.me/internal/activitypub"
	"github.com/jclem/jclem.me/internal/activitypub/identity"
	"github.com/jclem/jclem.me/internal/webfinger"
	"github.com/jclem/jclem.me/internal/www/config"
)

type activityInput struct {
	Context string `json:"@context"`
	Type    string `json:"type"`
	ID      string `json:"id"`
}

type pubRouter struct {
	*chi.Mux
	id  *identity.Service
	pub *ap.Service
}

func newPubRouter() (*pubRouter, error) {
	pool, err := pgxpool.New(context.Background(), config.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	id, err := identity.NewService(pool)
	if err != nil {
		return nil, fmt.Errorf("error creating identity service: %w", err)
	}

	pub, err := ap.NewService(context.Background(), pool, id)
	if err != nil {
		return nil, fmt.Errorf("error creating activitypub service: %w", err)
	}

	r := chi.NewRouter()
	p := &pubRouter{Mux: r, id: id, pub: pub}
	r.Use(p.setContentType)
	r.Get("/.well-known/webfinger", p.handleWebfinger)
	r.Mount("/~{username}", p.userRouter())

	return p, nil
}

func (p *pubRouter) userRouter() chi.Router { //nolint:ireturn
	rr := chi.NewRouter()
	rr.Use(p.ensureUser)
	rr.Get("/", p.getUser)
	rr.Get("/notes/{id}", p.getNote)
	rr.Get("/outbox", p.getOutbox)
	rr.Get("/followers", p.listFollowers)
	rr.Get("/following", p.listFollowing)
	rr.Post("/inbox", p.acceptActivity)

	rr.Group(func(rr chi.Router) {
		rr.Use(p.verifyBearerToken)
		rr.Post("/outbox", p.createActivity)
	})

	return rr
}

func (p *pubRouter) createActivity(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(identity.User) //nolint:forceTypeAssert

	pubKey, err := p.id.GetPublicKey(r.Context(), user.ID)
	if err != nil {
		returnError(r.Context(), w, err, "error getting public key")

		return
	}

	actor, err := ap.ActorFromUser(user, pubKey)
	if err != nil {
		returnError(r.Context(), w, err, "error getting user")

		return
	}

	var note ap.Note
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		returnError(r.Context(), w, err, "error decoding note")

		return
	}

	if note.Type != "Note" {
		returnCodeError(r.Context(), w, http.StatusUnprocessableEntity, "only Note activities are supported")

		return
	}

	if !note.Context.Contains(ap.ActivityStreamsContext) {
		returnCodeError(r.Context(), w, http.StatusUnprocessableEntity, "only ActivityStreams context is supported")

		return
	}

	note.Context = ap.NewContext(ap.ActivityStreamsContext, ap.MastodonContext)
	note.ID = fmt.Sprintf("%s/notes/%s", actor.ID, uuid.New())
	note.AttributedTo = actor.ID
	note.Type = "Note"
	note.Published = time.Now().UTC().Format(http.TimeFormat)
	note.To = []string{ap.ActivityStreamsContext + "#Public"}
	note.Cc = []string{actor.Followers}

	activity := ap.Activity[ap.Note]{
		Context:   ap.NewContext(ap.ActivityStreamsContext),
		Type:      "Create",
		ID:        fmt.Sprintf("%s/outbox/%s", actor.ID, uuid.New()),
		Actor:     actor.ID,
		Object:    note,
		Published: note.Published,
		To:        note.To,
		Cc:        note.Cc,
	}

	j, err := json.Marshal(activity)
	if err != nil {
		returnError(r.Context(), w, err, "error encoding activity")

		return
	}

	ar, err := p.pub.CreateActivity(r.Context(), user.ID, ap.Outbox, ap.ActivityStreamsContext, activity.Type, activity.ID, j)
	if err != nil {
		returnError(r.Context(), w, err, "error creating activity")

		return
	}

	a, err := ap.ActivityRecordToActivity[ap.Note](ar)
	if err != nil {
		returnError(r.Context(), w, err, "error converting activity record to activity")

		return
	}

	w.Header().Set("Location", a.ID)
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(a); err != nil {
		returnError(r.Context(), w, err, "error encoding activity")

		return
	}
}

func (p *pubRouter) acceptActivity(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(identity.User) //nolint:forceTypeAssert

	b, err := io.ReadAll(r.Body)
	if err != nil {
		returnError(r.Context(), w, err, "error reading body")

		return
	}

	var activity activityInput
	if err := json.Unmarshal(b, &activity); err != nil {
		returnError(r.Context(), w, err, "error decoding activity")

		return
	}

	ar, err := p.pub.CreateActivity(r.Context(), user.ID, ap.Inbox, activity.Context, activity.Type, activity.ID, b)
	if err != nil {
		returnError(r.Context(), w, err, "error creating activity")

		return
	}

	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(ar); err != nil {
		returnError(r.Context(), w, err, "error encoding activity")

		return
	}
}

func (p *pubRouter) getUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	user, err := p.id.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			returnCodeError(r.Context(), w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

			return
		}

		returnError(r.Context(), w, err, "error getting user")

		return
	}

	pubKey, err := p.id.GetPublicKey(r.Context(), user.ID)
	if err != nil {
		returnError(r.Context(), w, err, "error getting public key")

		return
	}

	actor, err := ap.ActorFromUser(user, pubKey)
	if err != nil {
		returnCodeError(r.Context(), w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

		return
	}

	if err := json.NewEncoder(w).Encode(actor); err != nil {
		returnError(r.Context(), w, err, "error encoding actor")

		return
	}
}

func (p *pubRouter) getNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	returnCodeError(r.Context(), w, http.StatusNotFound, fmt.Sprintf("note not found: %q", id))
}

func (p *pubRouter) getOutbox(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(identity.User) //nolint:forceTypeAssert

	pubKey, err := p.id.GetPublicKey(r.Context(), user.ID)
	if err != nil {
		returnError(r.Context(), w, err, "error getting public key")

		return
	}

	actor, err := ap.ActorFromUser(user, pubKey)
	if err != nil {
		returnError(r.Context(), w, err, "error getting AP user")

		return
	}

	items, err := p.pub.ListPublicOutbox(r.Context(), user.ID)
	if err != nil {
		returnError(r.Context(), w, err, "error listing outbox")

		return
	}

	itemObjects := make([]*ap.Activity[ap.Note], 0, len(items))

	for _, item := range items {
		itemObject, err := ap.ActivityRecordToActivity[ap.Note](item)
		if err != nil {
			returnError(r.Context(), w, err, "error converting activity record to activity")

			return
		}

		itemObjects = append(itemObjects, itemObject)
	}

	collection := ap.NewCollection(actor.Outbox, itemObjects)
	if err := json.NewEncoder(w).Encode(collection); err != nil {
		returnError(r.Context(), w, err, "error encoding actor")

		return
	}
}

func (p *pubRouter) listFollowers(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(identity.User) //nolint:forceTypeAssert

	followers, err := p.pub.ListFollowers(r.Context(), user.ID)
	if err != nil {
		returnError(r.Context(), w, err, "error listing followers")

		return
	}

	followerIDs := make([]string, 0, len(followers))
	for _, follower := range followers {
		followerIDs = append(followerIDs, follower.ActorID)
	}

	pubKey, err := p.id.GetPublicKey(r.Context(), user.ID)
	if err != nil {
		returnError(r.Context(), w, err, "error getting public key")

		return
	}

	actor, err := ap.ActorFromUser(user, pubKey)
	if err != nil {
		returnError(r.Context(), w, err, "error getting AP user")

		return
	}

	collection := ap.NewCollection(actor.Followers, followerIDs)
	if err := json.NewEncoder(w).Encode(collection); err != nil {
		returnError(r.Context(), w, err, "error encoding collection")

		return
	}
}

func (p *pubRouter) listFollowing(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	user, err := p.id.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			returnCodeError(r.Context(), w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

			return
		}

		returnError(r.Context(), w, err, "error getting user")

		return
	}

	pubKey, err := p.id.GetPublicKey(r.Context(), user.ID)
	if err != nil {
		returnError(r.Context(), w, err, "error getting public key")

		return
	}

	actor, err := ap.ActorFromUser(user, pubKey)

	if err != nil {
		returnCodeError(r.Context(), w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

		return
	}

	collection := ap.NewCollection(actor.Following, []string{})
	if err := json.NewEncoder(w).Encode(collection); err != nil {
		returnError(r.Context(), w, err, "error encoding collection")

		return
	}
}

var webfingerResourceRegex = regexp.MustCompile(`^acct:([^@]+)@([^@]+)$`)

func (p *pubRouter) handleWebfinger(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if resource == "" {
		returnCodeError(r.Context(), w, http.StatusBadRequest, "missing resource parameter")

		return
	}

	parts := webfingerResourceRegex.FindStringSubmatch(resource)
	if len(parts) != 3 {
		returnCodeError(r.Context(), w, http.StatusBadRequest, "invalid resource parameter")

		return
	}

	if domain := parts[2]; domain != ap.Domain {
		returnCodeError(r.Context(), w, http.StatusNotFound, "user not found")

		return
	}

	username := parts[1]

	user, err := p.id.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			returnCodeError(r.Context(), w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

			return
		}

		returnError(r.Context(), w, err, "error getting user")

		return
	}

	pubKey, err := p.id.GetPublicKey(r.Context(), user.ID)
	if err != nil {
		returnError(r.Context(), w, err, "error getting public key")

		return
	}

	actor, err := ap.ActorFromUser(user, pubKey)
	if err != nil {
		returnCodeError(r.Context(), w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

		return
	}

	if err := json.NewEncoder(w).Encode(webfinger.JRD{
		Subject: resource,
		Aliases: []string{actor.ID},
		Links: []webfinger.Link{
			{
				Rel:  "self",
				Type: ap.ContentType,
				Href: actor.ID,
			},
		},
	}); err != nil {
		returnError(r.Context(), w, err, "error encoding webfinger response")

		return
	}
}

func (p *pubRouter) setContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ap.ContentType)
		next.ServeHTTP(w, r)
	})
}

func (p *pubRouter) ensureUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")

		user, err := p.id.GetUserByUsername(r.Context(), username)
		if err != nil {
			returnCodeError(r.Context(), w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

var bearerTokenRegex = regexp.MustCompile(`^Bearer (\S+)$`)
var userContextKey = struct{}{} //nolint:gochecknoglobals

func (p *pubRouter) verifyBearerToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			returnCodeError(r.Context(), w, http.StatusUnauthorized, "no authorization header")

			return
		}

		parts := bearerTokenRegex.FindStringSubmatch(auth)
		if len(parts) != 2 {
			returnCodeError(r.Context(), w, http.StatusUnauthorized, "invalid authorization header")

			return
		}

		user, err := p.id.ValidateAPIKey(r.Context(), parts[1])
		if err != nil {
			returnCodeError(r.Context(), w, http.StatusUnauthorized, "invalid authorization header")

			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
