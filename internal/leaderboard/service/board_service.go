package service

import (
	"context"
	"log"

	"gojo/internal/leaderboard/dto"
	"gojo/internal/leaderboard/repository"
	usermodel "gojo/internal/user/model"
)

type UserProvider interface {
	GetUsersMapByIDs(ctx context.Context, userIDs []uint) (map[uint]string, error)
}

type ScoreBootstrapProvider interface {
	GetTopUsersBySolvedCount(ctx context.Context, limit int) ([]usermodel.User, error)
}

type LeaderboardService struct {
	repo         repository.LeaderboardRepository
	userProvider UserProvider
}

func NewLeaderboardService(r repository.LeaderboardRepository, up UserProvider) *LeaderboardService {
	return &LeaderboardService{repo: r, userProvider: up}
}

func (s *LeaderboardService) GetGlobalLeaderboard(ctx context.Context, currentUserID uint) (*dto.LeaderboardData, error) {
	data := &dto.LeaderboardData{
		Top50:  make([]dto.LeaderboardItem, 0),
		MyRank: -1,
	}

	records, err := s.repo.GetTopN(ctx, 50)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 && s.userProvider != nil {
		if bootstrap, ok := s.userProvider.(ScoreBootstrapProvider); ok {
			users, seedErr := bootstrap.GetTopUsersBySolvedCount(ctx, 50)
			if seedErr != nil {
				log.Printf("bootstrap leaderboard from mysql failed: %v", seedErr)
			} else if len(users) > 0 {
				seedRecords := make([]repository.RankRecord, 0, len(users))
				for _, user := range users {
					seedRecords = append(seedRecords, repository.RankRecord{
						UserID: user.ID,
						Score:  user.SolvedCount * 10,
					})
				}

				if err := s.repo.SeedScores(ctx, seedRecords); err != nil {
					log.Printf("seed leaderboard redis failed: %v", err)
				}

				records = seedRecords
			}
		}
	}

	if len(records) == 0 {
		return data, nil
	}

	topUserIDs := make([]uint, 0, len(records))
	for _, rec := range records {
		topUserIDs = append(topUserIDs, rec.UserID)
	}

	userMap := make(map[uint]string)
	if s.userProvider != nil {
		userMap, err = s.userProvider.GetUsersMapByIDs(ctx, topUserIDs)
		if err != nil {
			log.Printf("load leaderboard usernames failed: %v", err)
			userMap = make(map[uint]string)
		}
	}

	for i, rec := range records {
		username := userMap[rec.UserID]
		if username == "" {
			username = "mystery user"
		}

		data.Top50 = append(data.Top50, dto.LeaderboardItem{
			Rank:     int64(i + 1),
			Score:    rec.Score,
			UserID:   rec.UserID,
			Username: username,
		})
	}

	if currentUserID != 0 {
		rank, score, err := s.repo.GetUserRankAndScore(ctx, currentUserID)
		if err != nil {
			return nil, err
		}
		if rank != -1 {
			data.MyRank = rank
			data.MyScore = score
		}
	}

	return data, nil
}
