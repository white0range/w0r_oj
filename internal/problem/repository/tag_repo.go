package repository

import (
	"context"

	"gojo/infrastructure/mysql"
	"gojo/internal/app/apperror"
	"gojo/internal/problem/model"

	"gorm.io/gorm"
)

type TagRepository interface {
	GetTagList(ctx context.Context, tags *[]model.Tag) error
	CreateTag(ctx context.Context, tag *model.Tag) error
	DeleteTag(ctx context.Context, tagID string) ([]uint, error)
}

type TagRepositoryMysql struct{}

func NewTagRepository() TagRepository {
	return &TagRepositoryMysql{}
}

func (r *TagRepositoryMysql) GetTagList(ctx context.Context, tags *[]model.Tag) error {
	return mysql.DB.WithContext(ctx).Find(tags).Error
}

func (r *TagRepositoryMysql) CreateTag(ctx context.Context, tag *model.Tag) error {
	return mysql.DB.WithContext(ctx).Create(&tag).Error
}

func (r *TagRepositoryMysql) DeleteTag(ctx context.Context, tagID string) (problemIDs []uint, err error) {
	err = mysql.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tag model.Tag
		if err := tx.First(&tag, tagID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return apperror.ErrTagNotFound
			}
			return err
		}

		if err := tx.Table("problem_tags").Where("tag_id = ?", tag.ID).Pluck("problem_id", &problemIDs).Error; err != nil {
			return err
		}
		if err := tx.Model(&tag).Association("Problems").Clear(); err != nil {
			return err
		}
		return tx.Delete(&tag).Error
	})
	return problemIDs, err
}
