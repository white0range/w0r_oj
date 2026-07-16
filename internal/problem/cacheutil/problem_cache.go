package cacheutil

import (
	"context"
	"fmt"
	"log"

	"gojo/infrastructure/cache"
)

const (
	problemListCachePattern = "cache:problems:page:*"
	problemDetailCache      = "cache:problem:detail:%d"
)

// InvalidateProblem removes every cache entry that can expose stale problem data.
func InvalidateProblem(ctx context.Context, problemID uint, scene string) {
	if err := cache.Rdb.Del(ctx, fmt.Sprintf(problemDetailCache, problemID)).Err(); err != nil {
		log.Printf("clear problem detail cache failed after %s for problem %d: %v", scene, problemID, err)
	}
	InvalidateProblemList(ctx, scene)
}

func InvalidateProblemList(ctx context.Context, scene string) {
	if err := deleteByPattern(ctx, problemListCachePattern); err != nil {
		log.Printf("clear problem list cache failed after %s: %v", scene, err)
	}
}

func InvalidateAllProblemCaches(ctx context.Context, scene string) {
	if err := deleteByPattern(ctx, problemListCachePattern); err != nil {
		log.Printf("clear problem list cache failed after %s: %v", scene, err)
	}
	if err := deleteByPattern(ctx, "cache:problem:detail:*"); err != nil {
		log.Printf("clear problem detail caches failed after %s: %v", scene, err)
	}
}

func deleteByPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, next, err := cache.Rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := cache.Rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}
