package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gojo/config"
	"gojo/infrastructure/cache"
	"gojo/internal/problem/dto"
	"gojo/internal/problem/model"
	"gojo/internal/problem/repository"
)

type ProblemService struct {
	repo       repository.ProblemRepository
	searchRepo repository.ProblemSearchRepository
}

type ragProblemSyncRequest struct {
	ProblemID uint `json:"problem_id"`
}

func NewProblemService(r repository.ProblemRepository, sr repository.ProblemSearchRepository) *ProblemService {
	return &ProblemService{
		repo:       r,
		searchRepo: sr,
	}
}

func (s *ProblemService) CreateProblem(ctx context.Context, req dto.ProblemRequest) (*model.Problem, error) {
	timeLimit := req.TimeLimit
	if timeLimit == 0 {
		timeLimit = 1000
	}

	memoryLimit := req.MemoryLimit
	if memoryLimit == 0 {
		memoryLimit = 256
	}

	problem := model.Problem{
		Title:       req.Title,
		Description: req.Description,
		TimeLimit:   timeLimit,
		MemoryLimit: memoryLimit,
	}

	if len(req.TagIDs) > 0 {
		tags, err := s.repo.GetTagsByIDs(ctx, req.TagIDs)
		if err != nil {
			return nil, err
		}
		problem.Tags = tags
	}

	if len(req.TestCases) > 0 {
		cases := make([]model.TestCase, 0, len(req.TestCases))
		for _, tcReq := range req.TestCases {
			cases = append(cases, model.TestCase{
				Input:          tcReq.Input,
				ExpectedOutput: tcReq.ExpectedOutput,
			})
		}
		problem.TestCases = cases
	}

	if err := s.repo.CreateProblem(ctx, &problem); err != nil {
		return nil, err
	}

	s.syncProblemToESDocument(ctx, &problem)
	s.clearProblemListCache(ctx, "create")
	s.syncProblemToRAG(ctx, problem.ID)
	return &problem, nil
}

func (s *ProblemService) clearProblemListCache(ctx context.Context, scene string) {
	keys, err := cache.Rdb.Keys(ctx, "cache:problems:page:*").Result()
	if err != nil {
		log.Printf("list problem cache keys failed after %s: %v", scene, err)
		return
	}
	if len(keys) == 0 {
		return
	}
	if err := cache.Rdb.Del(ctx, keys...).Err(); err != nil {
		log.Printf("clear problem list cache failed after %s: %v", scene, err)
	}
}

func (s *ProblemService) clearProblemDetailCache(ctx context.Context, problemID string, scene string) {
	if err := cache.Rdb.Del(ctx, fmt.Sprintf("cache:problem:detail:%s", problemID)).Err(); err != nil {
		log.Printf("clear problem detail cache failed after %s for problem %s: %v", scene, problemID, err)
	}
}

func (s *ProblemService) GetProblemList(ctx context.Context, page int, limit int, tagIDStr string, uid uint) (*dto.ProblemListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	res := &dto.ProblemListResponse{
		Page:  page,
		Limit: limit,
		TagID: tagIDStr,
	}

	cacheKey := fmt.Sprintf("cache:problems:page:%d:limit:%d:tag:%s", res.Page, res.Limit, res.TagID)
	cachedData, err := cache.Rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		fmt.Println("cache hit for problem list")
		if unmarshalErr := json.Unmarshal([]byte(cachedData), res); unmarshalErr != nil {
			log.Printf("unmarshal problem list cache failed: %v", unmarshalErr)
			res = &dto.ProblemListResponse{
				Page:  page,
				Limit: limit,
				TagID: tagIDStr,
			}
		}
	}

	if len(res.Items) == 0 && res.Total == 0 {
		fmt.Println("cache miss for problem list, loading from mysql")
		items, total, err := s.repo.GetProblemList(ctx, page, limit, tagIDStr)
		if err != nil {
			return nil, err
		}
		res.Items = items
		res.Total = total
		res.Message = "get problem list success"

		jsonBytes, marshalErr := json.Marshal(res)
		if marshalErr != nil {
			log.Printf("marshal problem list cache failed: %v", marshalErr)
		} else if setErr := cache.Rdb.Set(ctx, cacheKey, jsonBytes, time.Hour).Err(); setErr != nil {
			log.Printf("cache problem list failed: %v", setErr)
		}
	}

	if uid != 0 && len(res.Items) > 0 {
		pageProblemIDs := make([]uint, 0, len(res.Items))
		for _, p := range res.Items {
			pageProblemIDs = append(pageProblemIDs, p.ID)
		}

		userACList, err := s.repo.GetUserACProblemIDs(ctx, uid, pageProblemIDs)
		if err != nil {
			log.Printf("load user ac problem ids failed for user %d: %v", uid, err)
			return res, nil
		}

		acMap := make(map[uint]bool, len(userACList))
		for _, id := range userACList {
			acMap[id] = true
		}

		for i := range res.Items {
			if acMap[res.Items[i].ID] {
				res.Items[i].IsAC = true
			}
		}
	}

	return res, nil
}

func (s *ProblemService) GetProblemDetail(ctx context.Context, problemID string) (*model.Problem, error) {
	cacheKey := fmt.Sprintf("cache:problem:detail:%s", problemID)
	cachedData, err := cache.Rdb.Get(ctx, cacheKey).Result()
	problem := &model.Problem{}
	if err == nil {
		fmt.Printf("cache hit for problem detail %s\n", problemID)
		if unmarshalErr := json.Unmarshal([]byte(cachedData), problem); unmarshalErr == nil {
			return problem, nil
		} else {
			log.Printf("unmarshal problem detail cache failed: %v", unmarshalErr)
		}
	}

	problem, err = s.repo.GetProblemByID(ctx, problemID)
	if err != nil {
		return nil, err
	}

	jsonBytes, marshalErr := json.Marshal(problem)
	if marshalErr != nil {
		log.Printf("marshal problem detail cache failed: %v", marshalErr)
	} else if setErr := cache.Rdb.Set(ctx, cacheKey, jsonBytes, 24*time.Hour).Err(); setErr != nil {
		log.Printf("cache problem detail failed: %v", setErr)
	}

	return problem, nil
}

func (s *ProblemService) UpdateProblem(ctx context.Context, problemID string, req dto.ProblemRequest) error {
	updateData := make(map[string]interface{})
	if req.Title != "" {
		updateData["title"] = req.Title
	}
	if req.Description != "" {
		updateData["description"] = req.Description
	}
	if req.TimeLimit > 0 {
		updateData["time_limit"] = req.TimeLimit
	}
	if req.MemoryLimit > 0 {
		updateData["memory_limit"] = req.MemoryLimit
	}

	if len(updateData) == 0 {
		return nil
	}

	if err := s.repo.UpdateProblem(ctx, problemID, updateData); err != nil {
		return err
	}

	s.clearProblemDetailCache(ctx, problemID, "update")
	s.clearProblemListCache(ctx, "update")
	s.syncProblemToESByStringID(ctx, problemID)
	s.syncProblemToRAGByStringID(ctx, problemID)
	return nil
}

func (s *ProblemService) DeleteProblem(ctx context.Context, problemID string) error {
	if err := s.repo.DeleteProblem(ctx, problemID); err != nil {
		return err
	}

	s.clearProblemDetailCache(ctx, problemID, "delete")
	s.clearProblemListCache(ctx, "delete")
	s.deleteProblemFromES(ctx, problemID)
	s.deleteProblemFromRAG(ctx, problemID)
	return nil
}

func (s *ProblemService) UpdateProblemTags(ctx context.Context, problemID string, tagIDs []uint) error {
	if err := s.repo.UpdateProblemTags(ctx, problemID, tagIDs); err != nil {
		return err
	}

	s.clearProblemDetailCache(ctx, problemID, "update tags")
	s.clearProblemListCache(ctx, "update tags")
	s.syncProblemToESByStringID(ctx, problemID)
	s.syncProblemToRAGByStringID(ctx, problemID)
	return nil
}

func (s *ProblemService) SearchProblems(ctx context.Context, req dto.SearchRequest) (*dto.SearchResult, error) {
	total, data, err := s.searchRepo.SearchProblems(ctx, req.Keyword, req.Tags)
	if err != nil {
		return nil, err
	}

	return &dto.SearchResult{
		Total: total,
		Data:  data,
	}, nil
}

func (s *ProblemService) SyncAllProblemsToES(ctx context.Context) error {
	problems, err := s.repo.GetAllProblemsWithTags(ctx)
	if err != nil {
		return fmt.Errorf("load problems from mysql failed: %w", err)
	}

	fmt.Printf("preparing to sync %d problems to elasticsearch\n", len(problems))
	successCount := 0

	for i := range problems {
		if err := s.searchRepo.UpsertProblemToES(ctx, buildESProblemDoc(&problems[i])); err != nil {
			log.Printf("sync problem %d to es failed: %v", problems[i].ID, err)
			continue
		}
		successCount++
	}

	fmt.Printf("elasticsearch sync finished, succeeded: %d\n", successCount)
	return nil
}

func buildESProblemDoc(problem *model.Problem) model.EsProblem {
	tagNames := make([]string, 0, len(problem.Tags))
	for _, t := range problem.Tags {
		tagNames = append(tagNames, t.Name)
	}

	return model.EsProblem{
		ID:          problem.ID,
		Title:       problem.Title,
		Description: problem.Description,
		Tags:        tagNames,
	}
}

func (s *ProblemService) syncProblemToESDocument(ctx context.Context, problem *model.Problem) {
	if err := s.searchRepo.UpsertProblemToES(ctx, buildESProblemDoc(problem)); err != nil {
		log.Printf("sync problem %d to es failed: %v", problem.ID, err)
	}
}

func (s *ProblemService) syncProblemToESByStringID(ctx context.Context, problemID string) {
	problem, err := s.repo.GetProblemByID(ctx, problemID)
	if err != nil {
		log.Printf("load problem %s for es sync failed: %v", problemID, err)
		return
	}

	s.syncProblemToESDocument(ctx, problem)
}

func (s *ProblemService) deleteProblemFromES(ctx context.Context, problemID string) {
	id, err := strconv.ParseUint(strings.TrimSpace(problemID), 10, 64)
	if err != nil {
		log.Printf("skip es delete because problem id is invalid: %q err=%v", problemID, err)
		return
	}

	if err := s.searchRepo.DeleteProblemFromES(ctx, uint(id)); err != nil {
		log.Printf("delete problem %d from es failed: %v", id, err)
	}
}

func (s *ProblemService) syncProblemToRAGByStringID(ctx context.Context, problemID string) {
	id, err := strconv.ParseUint(strings.TrimSpace(problemID), 10, 64)
	if err != nil {
		log.Printf("skip rag sync because problem id is invalid: %q err=%v", problemID, err)
		return
	}

	s.syncProblemToRAG(ctx, uint(id))
}

func (s *ProblemService) syncProblemToRAG(ctx context.Context, problemID uint) {
	if err := s.callRAGSyncEndpoint(ctx, "/rag/problems/sync", problemID); err != nil {
		log.Printf("sync problem %d to qdrant failed: %v", problemID, err)
	}
}

func (s *ProblemService) deleteProblemFromRAG(ctx context.Context, problemID string) {
	id, err := strconv.ParseUint(strings.TrimSpace(problemID), 10, 64)
	if err != nil {
		log.Printf("skip rag delete because problem id is invalid: %q err=%v", problemID, err)
		return
	}

	if err := s.callRAGSyncEndpoint(ctx, "/rag/problems/delete", uint(id)); err != nil {
		log.Printf("delete problem %d from qdrant failed: %v", id, err)
	}
}

func (s *ProblemService) callRAGSyncEndpoint(ctx context.Context, path string, problemID uint) error {
	agentBaseURL := strings.TrimRight(config.GlobalConfig.StudyPlan.AgentBaseURL, "/")
	if agentBaseURL == "" {
		agentBaseURL = "http://localhost:8000"
	}

	bodyBytes, err := json.Marshal(ragProblemSyncRequest{ProblemID: problemID})
	if err != nil {
		return fmt.Errorf("marshal rag sync request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		agentBaseURL+path,
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return fmt.Errorf("create rag sync request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer problem-sync-service")

	timeoutSeconds := config.GlobalConfig.StudyPlan.AgentTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}

	resp, err := (&http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("do rag sync request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rag sync returned status=%d", resp.StatusCode)
	}

	return nil
}
