// Package orderedmap provides a simple ordered map implementation, which is a
// list of name/value pairs.
package orderedmap

// An OrderedMap is a list of name/value pairs.
type OrderedMap []Item

// An Item is a name/value pair.
type Item struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
