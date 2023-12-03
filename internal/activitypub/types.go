package activitypub

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/jclem/jclem.me/internal/activitypub/identity"
)

// ActivityStreamsContext is the ActivityStreams context.
const ActivityStreamsContext = "https://www.w3.org/ns/activitystreams"

// SecurityContext is the security context (for public keys on actors).
const SecurityContext = "https://w3id.org/security/v1"

// MastodonContext is the Mastodon context.
var MastodonContext = map[string]string{ //nolint:gochecknoglobals
	"toot":         "http://joinmastodon.org/ns#",
	"discoverable": "toot:discoverable",
	"Hashtag":      "as:Hashtag",
	"sensitive":    "as:sensitive",
}

// A Context is a JSON-LD context.
//
// Although there is a normative object definition for context at
// https://www.w3.org/TR/json-ld/#context-definitions, we use a simple any
// array, as we usually do not care about the contents of the context.
//
// SEE https://www.w3.org/TR/json-ld/#the-context
type Context struct {
	rawValues []any
}

// Contains returns true if the given context is contained in the context.
//
// NOTE: This only checks for membership and doesn't look for expanded context,
// etc.
func (c Context) Contains(context any) bool {
	return slices.Contains(c.rawValues, context)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (c *Context) UnmarshalJSON(data []byte) error {
	var rawValuesArray []any
	if err := json.Unmarshal(data, &rawValuesArray); err == nil {
		c.rawValues = rawValuesArray

		return nil
	}

	var rawValues any
	if err := json.Unmarshal(data, &rawValues); err != nil {
		return fmt.Errorf("unmarshal context: %w", err)
	}

	c.rawValues = []any{rawValues}

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (c Context) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(c.rawValues)
	if err != nil {
		return nil, fmt.Errorf("marshal context: %w", err)
	}

	return b, nil
}

// NewContext creates a new context from the given raw values.
func NewContext(rawValues ...any) Context {
	return Context{rawValues: rawValues}
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
	Sensitive    bool     `json:"sensitive"`
	To           []string `json:"to"`
	Cc           []string `json:"cc"`
}

// ActorFromUser gets an actor from a system user.
func ActorFromUser(user identity.User, pubKey identity.SigningKey) (Actor, error) {
	username := user.Username

	return Actor{
		Context:           NewContext(ActivityStreamsContext, SecurityContext),
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
			Context: NewContext(ActivityStreamsContext),
			Type:    "Image",
			Name:    "Photograph of Jonathan Clem",
			URL:     "https://jclem.nyc3.cdn.digitaloceanspaces.com/profile/profile-1024.webp",
		},
		PublicKey: PublicKey{
			ID:           fmt.Sprintf("https://%s/~%s#main-key", Domain, username),
			Owner:        fmt.Sprintf("https://%s/~%s", Domain, username),
			PublicKeyPem: pubKey.PEM,
		},
	}, nil
}

// NewCollection creates a new OrderedCollection containing the given items.
func NewCollection[T any](id string, items []T) OrderedCollection[T] {
	return OrderedCollection[T]{
		Context: NewContext(
			ActivityStreamsContext,
			MastodonContext,
		),
		Type:         "OrderedCollection",
		ID:           id,
		TotalItems:   len(items),
		OrderedItems: items,
	}
}
