// Package store provides Firestore-backed data stores.
package store

import (
	"context"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"floodguard-backend/internal/models"
)

// FeedStore persists incoming feed items in Firestore.
type FeedStore struct {
	col *firestore.CollectionRef
}

// NewFeedStore creates a feed store backed by the "feeds" Firestore collection.
func NewFeedStore(client *firestore.Client) *FeedStore {
	return &FeedStore{col: client.Collection("feeds")}
}

// Add writes a feed item to Firestore with a server-side timestamp.
func (fs *FeedStore) Add(ctx context.Context, item models.FeedItem) {
	item.CreatedAt = time.Now().UnixMilli()
	if _, err := fs.col.Doc(item.ID).Set(ctx, item); err != nil {
		log.Printf("Firestore feed write error: %v", err)
	}
}

// GetSince returns feed items newer than the given ID, ordered newest-first.
// If sinceID is empty, returns the latest 50 items.
// Always returns a non-nil slice to avoid JSON null.
func (fs *FeedStore) GetSince(ctx context.Context, sinceID string) []models.FeedItem {
	q := fs.col.OrderBy("createdAt", firestore.Desc).Limit(50)

	if sinceID != "" {
		doc, err := fs.col.Doc(sinceID).Get(ctx)
		if err == nil {
			var ref models.FeedItem
			if err := doc.DataTo(&ref); err == nil && ref.CreatedAt > 0 {
				q = fs.col.Where("createdAt", ">", ref.CreatedAt).OrderBy("createdAt", firestore.Desc).Limit(50)
			}
		}
	}

	iter := q.Documents(ctx)
	defer iter.Stop()

	items := []models.FeedItem{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Firestore feed read error: %v", err)
			break
		}
		var item models.FeedItem
		if err := doc.DataTo(&item); err == nil {
			items = append(items, item)
		}
	}
	return items
}

// UpdateTriage patches a feed item's triageResult field in Firestore.
func (fs *FeedStore) UpdateTriage(ctx context.Context, feedID string, triage map[string]interface{}) {
	_, err := fs.col.Doc(feedID).Update(ctx, []firestore.Update{
		{Path: "triageResult", Value: triage},
	})
	if err != nil {
		log.Printf("Firestore triage update error for %s: %v", feedID, err)
	}
}
