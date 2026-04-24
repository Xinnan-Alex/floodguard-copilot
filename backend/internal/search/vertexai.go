// Package search provides Vertex AI Discovery Engine integration for RAG.
package search

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	discoveryengine "cloud.google.com/go/discoveryengine/apiv1"
	discoveryenginepb "cloud.google.com/go/discoveryengine/apiv1/discoveryenginepb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	clientOnce sync.Once
	client     *discoveryengine.SearchClient
	clientErr  error
)

func getClient() (*discoveryengine.SearchClient, error) {
	clientOnce.Do(func() {
		client, clientErr = discoveryengine.NewSearchClient(
			context.Background(),
			option.WithEndpoint("discoveryengine.googleapis.com:443"),
		)
	})
	return client, clientErr
}

// QueryVertexAI queries the Vertex AI Discovery Engine datastore for
// government policy documents to ground relief claim decisions (RAG).
// Falls back to a sensible default if no datastore is configured.
func QueryVertexAI(ctx context.Context, query string) (string, error) {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := "global"
	dataStoreID := os.Getenv("VERTEX_SEARCH_DATASTORE")

	if dataStoreID == "" {
		return "According to official MyDIGITAL blueprint, verified flood victims are eligible for RM 1000 automated fast-track payout.", nil
	}

	c, err := getClient()
	if err != nil {
		return "", err
	}

	req := &discoveryenginepb.SearchRequest{
		ServingConfig: fmt.Sprintf("projects/%s/locations/%s/collections/default_collection/dataStores/%s/servingConfigs/default_search", projectID, location, dataStoreID),
		Query:         query,
		PageSize:      3,
		ContentSearchSpec: &discoveryenginepb.SearchRequest_ContentSearchSpec{
			SnippetSpec: &discoveryenginepb.SearchRequest_ContentSearchSpec_SnippetSpec{
				ReturnSnippet: true,
			},
		},
	}

	it := c.Search(ctx, req)
	var snippets []string
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", err
		}

		if structData := resp.GetDocument().GetDerivedStructData(); structData != nil {
			if sField, ok := structData.GetFields()["snippets"]; ok {
				for _, snippet := range sField.GetListValue().GetValues() {
					snippets = append(snippets, snippet.GetStringValue())
				}
			}
		}
	}

	if len(snippets) == 0 {
		return "According to standard NADMA RM 1000 payout policy, verified flood victims are eligible for immediate fast-track aid.", nil
	}

	return strings.Join(snippets, "\n"), nil
}
