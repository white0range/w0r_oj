package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"gojo/infrastructure/cache"

	"github.com/redis/go-redis/v9"
)

type RankRecord struct {
	UserID uint
	Score  int
}

type LeaderboardRepository interface {
	GetTopN(ctx context.Context, limit int64) ([]RankRecord, error)
	GetUserRankAndScore(ctx context.Context, userID uint) (rank int64, score int, err error)
}

type leaderboardRepoRedis struct {
	leaderboardKey string
}

func NewLeaderboardRepository() LeaderboardRepository {
	return &leaderboardRepoRedis{leaderboardKey: "leaderboard:infrastructure"}
}

func (r *leaderboardRepoRedis) GetTopN(ctx context.Context, limit int64) ([]RankRecord, error) {
	zs, err := cache.Rdb.ZRevRangeWithScores(ctx, r.leaderboardKey, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}

	records := make([]RankRecord, 0, len(zs))
	for _, z := range zs {
		member, ok := z.Member.(string)
		if !ok {
			return nil, fmt.Errorf("invalid leaderboard member type: %T", z.Member)
		}

		uid, convErr := strconv.Atoi(member)
		if convErr != nil {
			return nil, fmt.Errorf("invalid leaderboard member value %q: %w", member, convErr)
		}

		records = append(records, RankRecord{
			UserID: uint(uid),
			Score:  int(z.Score),
		})
	}
	return records, nil
}

func (r *leaderboardRepoRedis) GetUserRankAndScore(ctx context.Context, userID uint) (int64, int, error) {
	uidStr := strconv.Itoa(int(userID))

	rank, err := cache.Rdb.ZRevRank(ctx, r.leaderboardKey, uidStr).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return -1, 0, nil
		}
		return -1, 0, err
	}

	score, err := cache.Rdb.ZScore(ctx, r.leaderboardKey, uidStr).Result()
	if err != nil {
		return -1, 0, err
	}

	return rank + 1, int(score), nil
}
