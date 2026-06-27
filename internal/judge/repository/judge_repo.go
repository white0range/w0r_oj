package repository

import (
	"context"

	"gojo/infrastructure/mysql"
	"gojo/internal/problem/model"
	submodel "gojo/internal/submission/model"
	usermodel "gojo/internal/user/model"
	"gojo/pkg/addscore"

	"gorm.io/gorm"
)

type JudgeRepository interface {
	GetProblemWithCases(ctx context.Context, problemID uint) (*model.Problem, error)
	UpdateJudgeResult(ctx context.Context, subID, problemID, userID uint, status, output string, timeCost, memoryCost int) error
}

type judgeRepoMysql struct{}

func NewJudgeRepository() JudgeRepository {
	return &judgeRepoMysql{}
}

func (r *judgeRepoMysql) GetProblemWithCases(ctx context.Context, problemID uint) (*model.Problem, error) {
	var problem model.Problem
	err := mysql.DB.WithContext(ctx).Preload("TestCases").First(&problem, problemID).Error
	return &problem, err
}

func (r *judgeRepoMysql) UpdateJudgeResult(ctx context.Context, subID, problemID, userID uint, status, output string, timeCost, memoryCost int) error {
	return mysql.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&submodel.Submission{}).Where("id = ?", subID).Updates(map[string]interface{}{
			"status":        status,
			"actual_output": output,
			"time_cost":     timeCost,
			"memory_cost":   memoryCost,
		}).Error; err != nil {
			return err
		}

		tx.Model(&model.Problem{}).Where("id = ?", problemID).UpdateColumn("submit_count", gorm.Expr("submit_count + ?", 1))

		if status == "AC" {
			tx.Model(&model.Problem{}).Where("id = ?", problemID).UpdateColumn("accepted_count", gorm.Expr("accepted_count + ?", 1))

			var acCount int64
			tx.Model(&submodel.Submission{}).Where("user_id = ? AND problem_id = ? AND status = 'AC'", userID, problemID).Count(&acCount)

			if acCount == 1 {
				tx.Model(&usermodel.User{}).Where("id = ?", userID).UpdateColumn("solved_count", gorm.Expr("solved_count + ?", 1))
				_ = addscore.AddUserScore(userID, 10.0)
			}
		}

		return nil
	})
}
