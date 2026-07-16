package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gojo/infrastructure/cache"
	"gojo/internal/problem/cacheutil"
	"gojo/internal/problem/model"
	"gojo/internal/problem/repository"
	"gojo/internal/syncer"
)

type TagService struct {
	repo   repository.TagRepository
	syncer syncer.Producer
}

func NewTagService(r repository.TagRepository, producer syncer.Producer) *TagService {
	return &TagService{repo: r, syncer: producer}
}

const TagCacheKey = "cache:tags:all"

func (s *TagService) GetTagList(ctx context.Context) ([]model.Tag, error) {
	var tags []model.Tag

	cachedData, err := cache.Rdb.Get(ctx, TagCacheKey).Result()
	if err == nil {
		fmt.Println("cache hit for tags")
		if unmarshalErr := json.Unmarshal([]byte(cachedData), &tags); unmarshalErr != nil {
			return nil, unmarshalErr
		}
		return tags, nil
	}

	if err := s.repo.GetTagList(ctx, &tags); err != nil {
		return nil, err
	}

	jsonBytes, marshalErr := json.Marshal(tags)
	if marshalErr != nil {
		log.Printf("marshal tag cache failed: %v", marshalErr)
		return tags, nil
	}
	if setErr := cache.Rdb.Set(ctx, TagCacheKey, jsonBytes, 7*24*time.Hour).Err(); setErr != nil {
		log.Printf("cache tag list failed: %v", setErr)
	}

	return tags, nil
}

func (s *TagService) CreateTag(ctx context.Context, name string) (*model.Tag, error) {
	tag := model.Tag{Name: name}
	if err := s.repo.CreateTag(ctx, &tag); err != nil {
		return nil, err
	}

	if err := cache.Rdb.Del(ctx, TagCacheKey).Err(); err != nil {
		log.Printf("clear tag cache after create failed: %v", err)
	}

	return &tag, nil
}

func (s *TagService) DeleteTag(ctx context.Context, tagID string) error {
	problemIDs, err := s.repo.DeleteTag(ctx, tagID)
	if err != nil {
		return err
	}

	if err := cache.Rdb.Del(ctx, TagCacheKey).Err(); err != nil {
		log.Printf("clear tag cache after delete failed: %v", err)
	}
	cacheutil.InvalidateAllProblemCaches(ctx, "delete tag")

	for _, problemID := range problemIDs {
		if err := s.syncer.EnqueueProblemUpsert(ctx, problemID); err != nil {
			log.Printf("enqueue problem %d sync after deleting tag failed: %v", problemID, err)
		}
	}
	return nil
}
