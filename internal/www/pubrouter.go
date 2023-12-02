package www

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v2"
	ap "github.com/jclem/jclem.me/internal/activitypub"
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
	pub *ap.Service
}

func newPubRouter(pub *ap.Service) *pubRouter {
	r := chi.NewRouter()
	p := pubRouter{Mux: r, pub: pub}
	r.Use(httplog.RequestLogger(httplog.NewLogger("pub")))
	r.Use(p.setContentType)
	r.Get("/.well-known/webfinger", p.handleWebfinger)
	r.Mount("/~{username}", p.userRouter())

	return &p
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
	auth := r.Header.Get("Authorization")
	if auth == "" {
		returnCodeError(w, http.StatusUnauthorized, "no authorization header")

		return
	}

	parts := strings.Split(auth, " ")
	if len(parts) != 2 {
		returnCodeError(w, http.StatusUnauthorized, "invalid authorization header")

		return
	}

	if parts[0] != "Bearer" {
		returnCodeError(w, http.StatusUnauthorized, "invalid authorization header")

		return
	}

	token := parts[1]
	if token != config.APIKey() {
		returnCodeError(w, http.StatusUnauthorized, "invalid authorization header")

		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		returnError(w, err, "error reading body")

		return
	}

	var activity activityInput
	if err := json.Unmarshal(b, &activity); err != nil {
		returnError(w, err, "error decoding activity")

		return
	}

	ar, err := p.pub.CreateActivity(r.Context(), ap.Inbox, activity.Context, activity.Type, activity.ID, b)
	if err != nil {
		returnError(w, err, "error creating activity")

		return
	}

	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(ar); err != nil {
		returnError(w, err, "error encoding activity")

		return
	}
}

func (p *pubRouter) getUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	user, err := ap.GetUser(username)
	if err != nil {
		returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

		return
	}

	if err := json.NewEncoder(w).Encode(user); err != nil {
		returnError(w, err, "error encoding actor")

		return
	}
}

func (p *pubRouter) getNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	returnCodeError(w, http.StatusNotFound, fmt.Sprintf("note not found: %q", id))
}

func (p *pubRouter) getOutbox(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := ap.GetUser(username)

	if err != nil {
		returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

		return
	}

	collection := ap.NewCollection(user.Outbox, []ap.Activity[ap.Note]{})
	if err := json.NewEncoder(w).Encode(collection); err != nil {
		returnError(w, err, "error encoding actor")

		return
	}
}

func (p *pubRouter) listFollowers(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := ap.GetUser(username)

	if err != nil {
		returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

		return
	}

	followers, err := p.pub.ListFollowers(r.Context())
	if err != nil {
		returnError(w, err, "error listing followers")

		return
	}

	followerIDs := make([]string, 0, len(followers))
	for _, follower := range followers {
		followerIDs = append(followerIDs, follower.ActorID)
	}

	collection := ap.NewCollection(user.Followers, followerIDs)
	if err := json.NewEncoder(w).Encode(collection); err != nil {
		returnError(w, err, "error encoding collection")

		return
	}
}

func (p *pubRouter) listFollowing(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := ap.GetUser(username)

	if err != nil {
		returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

		return
	}

	collection := ap.NewCollection(user.Following, []string{})
	if err := json.NewEncoder(w).Encode(collection); err != nil {
		returnError(w, err, "error encoding collection")

		return
	}
}

func (p *pubRouter) acceptActivity(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		returnError(w, err, "error reading body")

		return
	}

	var activity activityInput
	if err := json.Unmarshal(b, &activity); err != nil {
		returnError(w, err, "error decoding follow")

		return
	}

	ar, err := p.pub.CreateActivity(r.Context(), ap.Inbox, activity.Context, activity.Type, activity.ID, b)
	if err != nil {
		returnError(w, err, "error creating activity")

		return
	}

	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(ar); err != nil {
		returnError(w, err, "error encoding activity")

		return
	}
}

var webfingerResourceRegex = regexp.MustCompile(`^acct:([^@]+)@([^@]+)$`)

func (p *pubRouter) handleWebfinger(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if resource == "" {
		returnCodeError(w, http.StatusBadRequest, "missing resource parameter")

		return
	}

	parts := webfingerResourceRegex.FindStringSubmatch(resource)
	if len(parts) != 3 {
		returnCodeError(w, http.StatusBadRequest, "invalid resource parameter")

		return
	}

	if domain := parts[2]; domain != ap.Domain {
		returnCodeError(w, http.StatusNotFound, "user not found")

		return
	}

	username := parts[1]

	user, err := ap.GetUser(username)
	if err != nil {
		returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

		return
	}

	if err := json.NewEncoder(w).Encode(webfinger.JRD{
		Subject: resource,
		Aliases: []string{user.ID},
		Links: []webfinger.Link{
			{
				Rel:  "self",
				Type: ap.ContentType,
				Href: user.ID,
			},
		},
	}); err != nil {
		returnError(w, err, "error encoding webfinger response")

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
		if username != ap.GetMe().PreferredUsername {
			returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

			return
		}

		next.ServeHTTP(w, r)
	})
}

var bearerTokenRegex = regexp.MustCompile(`^Bearer (\S+)$`)

func (p *pubRouter) verifyBearerToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			returnCodeError(w, http.StatusUnauthorized, "no authorization header")

			return
		}

		// Find the match group in the regex.
		parts := bearerTokenRegex.FindStringSubmatch(auth)
		if len(parts) != 2 {
			returnCodeError(w, http.StatusUnauthorized, "invalid authorization header")

			return
		}

		if token := parts[1]; token != config.APIKey() {
			returnCodeError(w, http.StatusUnauthorized, "invalid authorization header")

			return
		}

		next.ServeHTTP(w, r)
	})
}
