package mysql

import (
	"fmt"
	"log"
	"time"

	"gojo/config"
	analysisModel "gojo/internal/analysis/model"
	problemModel "gojo/internal/problem/model"
	studyPlanModel "gojo/internal/study_plan/model"
	submissionModel "gojo/internal/submission/model"
	userModel "gojo/internal/user/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	dsn := config.GlobalConfig.SQL.Dsn

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("connect mysql failed: ", err)
	}

	fmt.Println("MySQL connected successfully")

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("get sql db failed: ", err)
	}

	sqlDB.SetMaxOpenConns(config.GlobalConfig.SQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.GlobalConfig.SQL.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(config.GlobalConfig.SQL.ConnMaxLifetimeSeconds) * time.Second)

	err = DB.AutoMigrate(
		&userModel.User{},
		&problemModel.Problem{},
		&submissionModel.Submission{},
		&problemModel.TestCase{},
		&problemModel.Tag{},
		&analysisModel.AnalysisTask{},
		&analysisModel.AnalysisFeedback{},
		&studyPlanModel.StudyPlanTask{},
		&studyPlanModel.StudyPlanFeedback{},
		&studyPlanModel.ChatSession{},
		&studyPlanModel.ChatMessage{},
		&studyPlanModel.ChatTurn{},
	)
	if err != nil {
		log.Fatal("auto migrate failed: ", err)
	}

	fmt.Println("database schema migrated successfully")
}
