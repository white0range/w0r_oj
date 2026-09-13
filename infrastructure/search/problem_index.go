package search

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const (
	// ProblemIndexName is versioned because Elasticsearch field analyzers cannot
	// be changed in place. Keep the old index intact for rollback.
	ProblemIndexName  = "problems_ik_v1"
	ProblemIndexAlias = "problems_current"
)

const problemIndexMapping = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0
  },
  "mappings": {
    "dynamic": "strict",
    "properties": {
      "id": { "type": "long" },
      "title": {
        "type": "text",
        "analyzer": "ik_max_word",
        "search_analyzer": "ik_smart",
        "fields": {
          "keyword": { "type": "keyword", "ignore_above": 256 }
        }
      },
      "description": {
        "type": "text",
        "analyzer": "ik_max_word",
        "search_analyzer": "ik_smart"
      },
      "tags": { "type": "keyword" }
    }
  }
}`

// EnsureProblemIndex creates the versioned IK index and its read/write alias.
// It deliberately never deletes the legacy dynamic-mapping `problems` index.
func EnsureProblemIndex(ctx context.Context) error {
	if EsClient == nil {
		return fmt.Errorf("elasticsearch client is not initialized")
	}

	exists, err := (esapi.IndicesExistsRequest{Index: []string{ProblemIndexName}}).Do(ctx, EsClient)
	if err != nil {
		return fmt.Errorf("check IK problem index: %w", err)
	}
	switch exists.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		create, err := (esapi.IndicesCreateRequest{
			Index: ProblemIndexName,
			Body:  bytes.NewBufferString(problemIndexMapping),
		}).Do(ctx, EsClient)
		if err != nil {
			return fmt.Errorf("create IK problem index: %w", err)
		}
		defer create.Body.Close()
		if create.IsError() {
			return fmt.Errorf("create IK problem index: %s", create.Status())
		}
	default:
		defer exists.Body.Close()
		return fmt.Errorf("check IK problem index: %s", exists.Status())
	}
	_ = exists.Body.Close()

	aliasBody := bytes.NewBufferString(`{
  "actions": [
    {
      "add": {
        "index": "problems_ik_v1",
        "alias": "problems_current",
        "is_write_index": true
      }
    }
  ]
}`)
	alias, err := (esapi.IndicesUpdateAliasesRequest{Body: aliasBody}).Do(ctx, EsClient)
	if err != nil {
		return fmt.Errorf("assign problem index alias: %w", err)
	}
	defer alias.Body.Close()
	if alias.IsError() {
		return fmt.Errorf("assign problem index alias: %s", alias.Status())
	}
	return nil
}
