package search

import (
	"context"
	"log"

	"github.com/elastic/go-elasticsearch/v8"

	"gojo/config"
)

// EsClient is the shared Elasticsearch client.
var EsClient *elasticsearch.Client

func InitElasticsearch() {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: config.GlobalConfig.Elasticsearch.Addresses,
	})
	if err != nil {
		log.Fatalf("create Elasticsearch client: %v", err)
	}

	res, err := client.Info()
	if err != nil {
		log.Fatalf("connect to Elasticsearch: %v", err)
	}
	res.Body.Close()

	EsClient = client
	if err := EnsureProblemIndex(context.Background()); err != nil {
		log.Fatalf("initialize IK problem index: %v", err)
	}
	log.Printf("Elasticsearch connected and IK problem index is ready")
}
