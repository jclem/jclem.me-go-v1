package activitypub

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const Context = "https://www.w3.org/ns/activitystreams"

type Person struct {
	Context           string    `json:"@context"`
	Type              string    `json:"type"`
	ID                string    `json:"id"`
	Inbox             string    `json:"inbox"`
	Outbox            string    `json:"outbox"`
	Followers         string    `json:"followers"`
	Following         string    `json:"following"`
	Liked             string    `json:"liked"`
	PreferredUsername string    `json:"preferredUsername"`
	Name              string    `json:"name"`
	Summary           string    `json:"summary"`
	URL               string    `json:"url"`
	Icon              Image     `json:"icon"`
	PublicKey         PublicKey `json:"publicKey"`
}

type PublicKey struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	PublicKeyPem string `json:"publicKeyPem"`
}

type Image struct {
	Context string `json:"@context"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	URL     string `json:"url"`
}

type OrderedCollection[T any] struct {
	Context      string `json:"@context"`
	ID           string `json:"id"`
	Type         string `json:"type"`
	TotalItems   int    `json:"totalItems"`
	First        string `json:"first"`
	Last         string `json:"last"`
	OrderedItems []T    `json:"orderedItems,omitempty"`
}

type OrderedCollectionPage[T any] struct {
	Context      string `json:"@context"`
	ID           string `json:"id"`
	Type         string `json:"type"`
	Next         string `json:"next"`
	Prev         string `json:"prev"`
	PartOf       string `json:"partOf"`
	OrderedItems []T    `json:"orderedItems,omitempty"`
}

type Activity[T any] struct {
	Context   string   `json:"@context"`
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	Actor     string   `json:"actor"`
	Object    T        `json:"object"`
	Published string   `json:"published"`
	To        []string `json:"to"`
	Cc        []string `json:"cc"`
}

type Note struct {
	Context   string   `json:"@context"`
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	Content   string   `json:"content"`
	Published string   `json:"published"`
	To        []string `json:"to"`
	Cc        []string `json:"cc"`
}

type Follow struct {
	Context string `json:"@context"`
	Type    string `json:"type"`
	ID      string `json:"id"`
	Actor   string `json:"actor"`
}

func GetNotes(user Person) []Activity[Note] {
	return []Activity[Note]{
		{
			Context:   Context,
			Type:      "Create",
			ID:        user.NoteID(3),
			Actor:     user.ID,
			Published: "2023-11-28T16:06:00Z",
			To:        []string{"https://www.w3.org/ns/activitystreams#Public"},
			Cc:        []string{user.Followers},
			Object: Note{
				Context:   Context,
				Type:      "Note",
				ID:        user.NoteID(2),
				Content:   "Test 3",
				Published: "2023-11-28T16:06:00Z",
				To:        []string{"https://www.w3.org/ns/activitystreams#Public"},
				Cc:        []string{user.Followers},
			},
		},
		{
			Context:   Context,
			Type:      "Create",
			ID:        user.NoteID(2),
			Actor:     user.ID,
			Published: "2023-11-28T16:04:00Z",
			To:        []string{"https://www.w3.org/ns/activitystreams#Public"},
			Cc:        []string{user.Followers},
			Object: Note{
				Context:   Context,
				Type:      "Note",
				ID:        user.NoteID(2),
				Content:   "Test 2",
				Published: "2023-11-28T16:04:00Z",
				To:        []string{"https://www.w3.org/ns/activitystreams#Public"},
				Cc:        []string{user.Followers},
			},
		},
		{
			Context:   Context,
			Type:      "Create",
			ID:        user.NoteID(1),
			Actor:     user.ID,
			Published: "2023-11-28T15:47:00Z",
			Object: Note{
				Context:   Context,
				Type:      "Note",
				ID:        user.NoteID(1),
				Content:   "Test 1",
				Published: "2023-11-28T15:47:00Z",
			},
		},
	}
}

const me = "jclem"

func GetMe() Person {
	p, err := GetUser(me)
	if err != nil {
		panic(err)
	}

	return p
}

func GetUser(username string) (Person, error) {
	if username != me {
		return Person{}, fmt.Errorf("user %s not found", username)
	}

	pubkey := strings.ReplaceAll(os.Getenv("AP_PUBLIC_KEY_PEM"), `\n`, "\n")
	if pubkey == "" {
		return Person{}, errors.New("AP_PUBLIC_KEY_PEM not set")
	}

	return Person{
		Context:           Context,
		Type:              "Person",
		ID:                fmt.Sprintf("https://pub.jclem.me/~%s", username),
		Inbox:             fmt.Sprintf("https://pub.jclem.me/~%s/inbox", username),
		Outbox:            fmt.Sprintf("https://pub.jclem.me/~%s/outbox", username),
		Followers:         fmt.Sprintf("https://pub.jclem.me/~%s/followers", username),
		Following:         fmt.Sprintf("https://pub.jclem.me/~%s/following", username),
		Liked:             fmt.Sprintf("https://pub.jclem.me/~%s/liked", username),
		PreferredUsername: username,
		Name:              "Jonathan Clem",
		Summary:           "A person that enjoys helping build things on the internet",
		Icon: Image{
			Context: Context,
			Type:    "Image",
			Name:    "Photograph of Jonathan Clem",
			URL:     "https://jclem.nyc3.cdn.digitaloceanspaces.com/profile/profile-1024.webp",
		},
		PublicKey: PublicKey{
			ID:           fmt.Sprintf("https://pub.jclem.me/~%s#main-key", username),
			Owner:        fmt.Sprintf("https://pub.jclem.me/~%s", username),
			PublicKeyPem: pubkey,
		},
	}, nil
}

func (a Person) NoteID(id int) string {
	return fmt.Sprintf("https://pub.jclem.me/~%s/notes/%d", a.PreferredUsername, id)
}

func (a Person) OutboxPage(page int) string {
	return fmt.Sprintf("https://pub.jclem.me/~%s/outbox/%d", a.PreferredUsername, page)
}

func NewCollection[T any](id string, items []T) OrderedCollection[T] {
	return OrderedCollection[T]{
		Context:      Context,
		Type:         "OrderedCollection",
		ID:           id,
		TotalItems:   len(items),
		OrderedItems: items,
	}
}

type WebfingerLink struct {
	Rel  string `json:"rel"`
	Type string `json:"type"`
	Href string `json:"href"`
}

type WebfingerResponse struct {
	Subject string          `json:"subject"`
	Aliases []string        `json:"aliases"`
	Links   []WebfingerLink `json:"links"`
}
