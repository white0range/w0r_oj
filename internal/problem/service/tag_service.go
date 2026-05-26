package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gojo/infrastructure/cache"
	"gojo/internal/problem/model"
	"gojo/internal/problem/repository"
)

type TagService struct {
	repo repository.TagRepository
}

func NewTagService(r repository.TagRepository) *TagService {
	return &TagService{repo: r}
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
	if err := s.repo.DeleteTag(ctx, tagID); err != nil {
		return err
	}

	if err := cache.Rdb.Del(ctx, TagCacheKey).Err(); err != nil {
		log.Printf("clear tag cache after delete failed: %v", err)
	}

	keys1, err1 := cache.Rdb.Keys(ctx, "cache:problems:page:*").Result()
	keys2, err2 := cache.Rdb.Keys(ctx, "cache:problem:detail:*").Result()
	if err1 != nil {
		log.Printf("list problem page cache keys failed after deleting tag: %v", err1)
	}
	if err2 != nil {
		log.Printf("list problem detail cache keys failed after deleting tag: %v", err2)
	}

	allKeys := append(keys1, keys2...)
	if len(allKeys) > 0 {
		if err := cache.Rdb.Del(ctx, allKeys...).Err(); err != nil {
			log.Printf("clear problem caches after deleting tag failed: %v", err)
		}
	}

	return nil
}
