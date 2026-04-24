// Package store provides Firestore-backed data stores.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"

	"cloud.google.com/go/firestore"
)

// Cache persists Gemini responses in Firestore to avoid redundant API calls.
// Since all flows use temperature=0, identical inputs produce identical outputs.
type Cache struct {
	col *firestore.CollectionRef
}

// NewCache creates a cache backed by the "cache" Firestore collection.
func NewCache(client *firestore.Client) *Cache {
	return &Cache{col: client.Collection("cache")}
}

// Key generates a deterministic cache key from a flow name and input text.
func (c *Cache) Key(flow, input string) string {
	h := sha256.Sum256([]byte(flow + ":" + input))
	return hex.EncodeToString(h[:])
}

// Get returns a cached response from Firestore if available.
func (c *Cache) Get(ctx context.Context, key string) (map[string]interface{}, bool) {
	doc, err := c.col.Doc(key).Get(ctx)
	if err != nil {
		return nil, false
	}
	data := doc.Data()
	if data == nil {
		return nil, false
	}
	return data, true
}

// Set stores a response in Firestore.
func (c *Cache) Set(ctx context.Context, key string, value map[string]interface{}) {
	if _, err := c.col.Doc(key).Set(ctx, value); err != nil {
		log.Printf("Firestore cache write error: %v", err)
	}
}
