package repository

import (
	"context"
	"log"

	"gojo/infrastructure/mysql"
	judgeModel "gojo/internal/judge/model"
	"gojo/internal/problem/cacheutil"
	"gojo/internal/problem/model"
	submodel "gojo/internal/submission/model"
	"gojo/internal/syncer"
	usermodel "gojo/internal/user/model"

	"gorm.io/gorm"
)

type JudgeRepository interface {
	GetProblemWithCases(ctx context.Context, problemID uint) (*model.Problem, error)
	IsSubmissionPending(ctx context.Context, subID uint) (bool, error)
	UpdateJudgeResult(ctx context.Context, subID, problemID, userID uint, status judgeModel.JudgeStatus, timeCost, memoryCost int) (bool, error)
}

type judgeRepoMysql struct {
	syncer syncer.Producer
}

func NewJudgeRepository(producer syncer.Producer) JudgeRepository {
	return &judgeRepoMysql{syncer: producer}
}

func (r *judgeRepoMysql) GetProblemWithCases(ctx context.Context, problemID uint) (*model.Problem, error) {
	var problem model.Problem
	err := mysql.DB.WithContext(ctx).Preload("TestCases").First(&problem, problemID).Error
	return &problem, err
}

func (r *judgeRepoMysql) IsSubmissionPending(ctx context.Context, subID uint) (bool, error) {
	var count int64
	err := mysql.DB.WithContext(ctx).
		Model(&submodel.Submission{}).
		Where("id = ? AND status = ?", subID, "Pending").
		Count(&count).Error
	return count == 1, err
}

// UpdateJudgeResult only transitions a Pending submission. This makes task
// retries safe when a worker completed judging but failed before acknowledging
// the Redis task.
func (r *judgeRepoMysql) UpdateJudgeResult(ctx context.Context, subID, problemID, userID uint, status judgeModel.JudgeStatus, timeCost, memoryCost int) (bool, error) {
	firstAC := false
	updated := false
	err := mysql.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&submodel.Submission{}).Where("id = ? AND status = ?", subID, "Pending").Updates(map[string]interface{}{
			"status":        status,
			"actual_output": judgeModel.PublicMessage(status),
			"time_cost":     timeCost,
			"memory_cost":   memoryCost,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		updated = true

		if err := tx.Model(&model.Problem{}).Where("id = ?", problemID).UpdateColumn("submit_count", gorm.Expr("submit_count + ?", 1)).Error; err != nil {
			return err
		}
		if status != judgeModel.StatusAccepted {
			return nil
		}
		if err := tx.Model(&model.Problem{}).Where("id = ?", problemID).UpdateColumn("accepted_count", gorm.Expr("accepted_count + ?", 1)).Error; err != nil {
			return err
		}

		var acCount int64
		if err := tx.Model(&submodel.Submission{}).Where("user_id = ? AND problem_id = ? AND status = 'AC'", userID, problemID).Count(&acCount).Error; err != nil {
			return err
		}
		if acCount == 1 {
			if err := tx.Model(&usermodel.User{}).Where("id = ?", userID).UpdateColumn("solved_count", gorm.Expr("solved_count + ?", 1)).Error; err != nil {
				return err
			}
			firstAC = true
		}
		return nil
	})
	if err != nil || !updated {
		return updated, err
	}

	cacheutil.InvalidateProblem(ctx, problemID, "judge result")
	if err := r.syncer.EnqueueProblemUpsert(ctx, problemID); err != nil {
		log.Printf("enqueue problem %d sync after judge result failed: %v", problemID, err)
	}
	if firstAC {
		if err := r.syncer.EnqueueUserScoreSync(ctx, userID); err != nil {
			log.Printf("enqueue leaderboard score sync for user %d failed: %v", userID, err)
		}
	}
	return true, nil
}
