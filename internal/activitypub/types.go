package activitypub

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ActivityStreamsContext is the ActivityStreams context.
const ActivityStreamsContext = "https://www.w3.org/ns/activitystreams"

// SecurityContext is the security context (for public keys on actors).
const SecurityContext = "https://w3id.org/security/v1"

// A Context is a JSON-LD context.
//
// Although there is a normative object definition for context at
// https://www.w3.org/TR/json-ld/#context-definitions, we use a simple string
// array, as typically we see context represented as a string or array of
// strings.
//
// SEE https://www.w3.org/TR/json-ld/#the-context
type Context []string

// UnmarshalJSON implements the json.Unmarshaler interface.
func (c *Context) UnmarshalJSON(data []byte) error {
	var context string
	if err := json.Unmarshal(data, &context); err == nil {
		*c = Context{context}

		return nil
	}

	var contexts []string
	if err := json.Unmarshal(data, &contexts); err != nil {
		// HACK: If we can't unmarshal as a string or array of strings, this is
		// a context object, and we just ignore those now.
		return nil //nolint:nilerr
	}

	*c = Context(contexts)

	return nil
}

// An Actor is an ActivityPub actor.
//
// We also include Mastodon-specific fields here, such as the public key.
// Likewise, we ignore some parts of ActivityPub/ActivityStreams, such as
// natural language support
// (https://www.w3.org/TR/activitystreams-core/#naturalLanguageValues).
//
// SEE https://www.w3.org/TR/activitypub/#actor-objects
type Actor struct {
	Context           Context   `json:"@context"`
	Type              string    `json:"type"`
	ID                string    `json:"id"`
	Inbox             string    `json:"inbox"`
	Outbox            string    `json:"outbox"`
	Following         string    `json:"following"`
	Followers         string    `json:"followers"`
	PreferredUsername string    `json:"preferredUsername"`
	Name              string    `json:"name"`
	Summary           string    `json:"summary"`
	URL               string    `json:"url"`
	Icon              Image     `json:"icon"`
	PublicKey         PublicKey `json:"publicKey"`
}

// A PublicKey is a public key definition as defined by the Security Vocabulary
// (https://w3c.github.io/vc-data-integrity/vocab/security/vocabulary.html#publicKey).
//
// As in other types, we do not allow for some expanded fields, such as for
// "owner".
//
// SEE https://w3c-ccg.github.io/security-vocab/contexts/security-v1.jsonld
type PublicKey struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	PublicKeyPem string `json:"publicKeyPem"`
}

// An Image is an ActivityStreams Image used to identify a user.
//
// SEE https://www.w3.org/TR/activitystreams-vocabulary/#dfn-image
type Image struct {
	Context Context `json:"@context"`
	Type    string  `json:"type"`
	Name    string  `json:"name"`
	URL     string  `json:"url"`
}

// An OrderedCollection is an ActivityStreams OrderedCollection.
//
// SEE https://www.w3.org/TR/activitystreams-vocabulary/#dfn-orderedcollection
type OrderedCollection[T any] struct {
	Context      Context `json:"@context"`
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	TotalItems   int     `json:"totalItems"`
	First        string  `json:"first,omitempty"`
	Last         string  `json:"last,omitempty"`
	OrderedItems []T     `json:"orderedItems,omitempty"`
}

// An Activity is an ActivityStreams Activity.
//
// SEE https://www.w3.org/TR/activitystreams-vocabulary/#dfn-activity
type Activity[T any] struct {
	Context   Context  `json:"@context"`
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	Actor     string   `json:"actor"`
	Object    T        `json:"object"`
	Published string   `json:"published"`
	To        []string `json:"to"`
	Cc        []string `json:"cc"`
}

// A Note is an ActivityStreams Note.
//
// SEE https://www.w3.org/TR/activitystreams-vocabulary/#dfn-note
type Note struct {
	Context      Context  `json:"@context"`
	Type         string   `json:"type"`
	ID           string   `json:"id"`
	AttributedTo string   `json:"attributedTo"`
	Content      string   `json:"content"`
	Published    string   `json:"published"`
	To           []string `json:"to"`
	Cc           []string `json:"cc"`
}

const me = "jclem"

// GetMe gets the actor Jonathan Clem :).
func GetMe() Actor {
	p, err := GetUser(me)
	if err != nil {
		panic(err)
	}

	return p
}

// GetUser gets an actor by username.
func GetUser(username string) (Actor, error) {
	if username != me {
		return Actor{}, fmt.Errorf("user %s not found", username)
	}

	pubkey := strings.ReplaceAll(os.Getenv("AP_PUBLIC_KEY_PEM"), `\n`, "\n")
	if pubkey == "" {
		return Actor{}, errors.New("AP_PUBLIC_KEY_PEM not set")
	}

	return Actor{
		Context:           Context{ActivityStreamsContext, SecurityContext},
		Type:              "Person",
		ID:                fmt.Sprintf("https://%s/~%s", Domain, username),
		Inbox:             fmt.Sprintf("https://%s/~%s/inbox", Domain, username),
		Outbox:            fmt.Sprintf("https://%s/~%s/outbox", Domain, username),
		Followers:         fmt.Sprintf("https://%s/~%s/followers", Domain, username),
		Following:         fmt.Sprintf("https://%s/~%s/following", Domain, username),
		PreferredUsername: username,
		Name:              "Jonathan Clem",
		Summary:           "A person that enjoys helping build things on the internet",
		Icon: Image{
			Context: Context{ActivityStreamsContext},
			Type:    "Image",
			Name:    "Photograph of Jonathan Clem",
			URL:     "https://jclem.nyc3.cdn.digitaloceanspaces.com/profile/profile-1024.webp",
		},
		PublicKey: PublicKey{
			ID:           fmt.Sprintf("https://%s/~%s#main-key", Domain, username),
			Owner:        fmt.Sprintf("https://%s/~%s", Domain, username),
			PublicKeyPem: pubkey,
		},
	}, nil
}

// NewCollection creates a new OrderedCollection containing the given items.
func NewCollection[T any](id string, items []T) OrderedCollection[T] {
	return OrderedCollection[T]{
		Context:      Context{ActivityStreamsContext},
		Type:         "OrderedCollection",
		ID:           id,
		TotalItems:   len(items),
		OrderedItems: items,
	}
}
