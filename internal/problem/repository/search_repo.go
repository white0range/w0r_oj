package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"gojo/infrastructure/search"
	"gojo/internal/problem/model"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type ProblemSearchRepository interface {
	SearchProblems(ctx context.Context, keyword string, tags []string) (int64, []map[string]interface{}, error)
	UpsertProblemToES(ctx context.Context, doc model.EsProblem) error
	DeleteProblemFromES(ctx context.Context, problemID uint) error
}

type problemSearchRepoES struct{}

func NewProblemSearchRepository() ProblemSearchRepository {
	return &problemSearchRepoES{}
}

func (r *problemSearchRepoES) SearchProblems(ctx context.Context, keyword string, tags []string) (int64, []map[string]interface{}, error) {
	must := []map[string]interface{}{}
	filter := []map[string]interface{}{}

	if keyword != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  keyword,
				"fields": []string{"title^3", "description"},
			},
		})
	}
	if len(tags) > 0 {
		filter = append(filter, map[string]interface{}{
			"terms": map[string]interface{}{"tags.keyword": tags},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{"bool": map[string]interface{}{"must": must, "filter": filter}},
		"size":  50,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return 0, nil, err
	}

	res, err := search.EsClient.Search(
		search.EsClient.Search.WithContext(ctx),
		search.EsClient.Search.WithIndex("problems"),
		search.EsClient.Search.WithBody(&buf),
		search.EsClient.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return 0, nil, fmt.Errorf("ES returned error: %s", res.String())
	}

	var esResult map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&esResult); err != nil {
		return 0, nil, err
	}

	hits := esResult["hits"].(map[string]interface{})["hits"].([]interface{})
	var resultData []map[string]interface{}
	for _, hit := range hits {
		source := hit.(map[string]interface{})["_source"].(map[string]interface{})
		source["_score"] = hit.(map[string]interface{})["_score"]
		resultData = append(resultData, source)
	}
	total := int64(esResult["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64))

	return total, resultData, nil
}

func (r *problemSearchRepoES) UpsertProblemToES(ctx context.Context, doc model.EsProblem) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index:      "problems",
		DocumentID: strconv.Itoa(int(doc.ID)),
		Body:       bytes.NewReader(body),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, search.EsClient)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ES rejected upsert: %s", res.String())
	}
	return nil
}

func (r *problemSearchRepoES) DeleteProblemFromES(ctx context.Context, problemID uint) error {
	req := esapi.DeleteRequest{
		Index:      "problems",
		DocumentID: strconv.Itoa(int(problemID)),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, search.EsClient)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil
	}
	if res.IsError() {
		return fmt.Errorf("ES delete failed: %s", res.String())
	}
	return nil
}
