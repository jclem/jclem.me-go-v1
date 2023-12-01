package www

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v2"
	ap "github.com/jclem/jclem.me/internal/activitypub"
)

func pubRouter(s *Server) chi.Router { //nolint:ireturn
	r := chi.NewRouter()
	r.Use(httplog.RequestLogger(httplog.NewLogger("pub")))
	r.Use(setActivityJSONContentType)
	r.Get("/.well-known/webfinger", webfinger())
	r.Mount("/~{username}", userRouter(s))

	return r
}

func userRouter(s *Server) chi.Router { //nolint:ireturn
	r := chi.NewRouter()
	r.Use(ensureUser)
	r.Get("/", getUser)
	r.Get("/notes/{id}", getNote(s))
	r.Get("/outbox", getOutbox(s))
	r.Get("/followers", listFollowers(s))
	r.Get("/following", listFollowing(s))
	r.Post("/inbox", createActivity(s))

	return r
}

func getUser(w http.ResponseWriter, r *http.Request) {
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

func getNote(_ *Server) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")

		user, err := ap.GetUser(username)
		if err != nil {
			returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

			return
		}

		id := chi.URLParam(r, "id")
		nid := fmt.Sprintf("https://%s/~%s/notes/%s", ap.Domain, username, id)

		for _, note := range ap.GetNotes(user) {
			if note.Object.ID == nid {
				if err := json.NewEncoder(w).Encode(note); err != nil {
					returnError(w, err, "error encoding note")

					return
				}

				return
			}
		}

		returnCodeError(w, http.StatusNotFound, fmt.Sprintf("note not found: %q", id))
	})
}

func getOutbox(_ *Server) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		user, err := ap.GetUser(username)
		if err != nil {
			returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

			return
		}

		collection := ap.NewCollection(user.Outbox, ap.GetNotes(user))
		if err := json.NewEncoder(w).Encode(collection); err != nil {
			returnError(w, err, "error encoding actor")

			return
		}
	})
}

func listFollowers(s *Server) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		user, err := ap.GetUser(username)
		if err != nil {
			returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

			return
		}

		followers, err := s.pub.ListFollowers(r.Context())
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
	})
}

func listFollowing(_ *Server) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})
}

func createActivity(s *Server) http.HandlerFunc {
	type activityInput struct {
		Context string `json:"@context"`
		Type    string `json:"type"`
		ID      string `json:"id"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		ar, err := s.pub.CreateActivity(r.Context(), ap.Inbox, activity.Context, activity.Type, activity.ID, b)
		if err != nil {
			returnError(w, err, "error creating activity")

			return
		}

		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(ar); err != nil {
			returnError(w, err, "error encoding activity")

			return
		}
	})
}

func webfinger() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resource := r.URL.Query().Get("resource")
		if resource == "" {
			returnCodeError(w, http.StatusBadRequest, "missing resource parameter")

			return
		}

		parts := strings.Split(resource, ":")
		if len(parts) != 2 {
			returnCodeError(w, http.StatusBadRequest, "invalid resource parameter")

			return
		}

		parts2 := strings.Split(parts[1], "@")
		if len(parts2) != 2 {
			returnCodeError(w, http.StatusBadRequest, "invalid resource parameter")

			return
		}

		if parts2[1] != ap.Domain {
			returnCodeError(w, http.StatusNotFound, "user not found")

			return
		}

		username := parts2[0]
		user, err := ap.GetUser(username)
		if err != nil {
			returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

			return
		}

		if err := json.NewEncoder(w).Encode(ap.WebfingerResponse{
			Subject: resource,
			Aliases: []string{user.ID},
			Links: []ap.WebfingerLink{
				{
					Rel:  "self",
					Type: "application/activity+json",
					Href: user.ID,
				},
			},
		}); err != nil {
			returnError(w, err, "error encoding webfinger response")

			return
		}
	})
}

func setActivityJSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

func ensureUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		if username != ap.GetMe().PreferredUsername {
			returnCodeError(w, http.StatusNotFound, fmt.Sprintf("user not found: %q", username))

			return
		}

		next.ServeHTTP(w, r)
	})
}
