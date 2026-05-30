package mysql

import (
	"fmt"
	"gojo/config"
	analysisModel "gojo/internal/analysis/model"
	problemModel "gojo/internal/problem/model"
	studyPlanModel "gojo/internal/study_plan/model"
	submissionModel "gojo/internal/submission/model"
	userModel "gojo/internal/user/model"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB 是一个全局变量。
// 注意看！它是大写的 'D'！这意味着以后整个公司的任何部门（比如 controllers），
// 只要 import 了 models，就能直接用 models.DB 来操作数据库！
var DB *gorm.DB

// InitDB 是初始化数据库连接的专门函数
func InitDB() {
	// 1. 准备连接字符串 (这里为了演示顺畅，我们先写在代码里。
	// 等项目跑通了，我们再把它抽离到 .yaml 配置文件里去)
	dsn := config.GlobalConfig.SQL.Dsn

	// 2. 尝试连接数据库
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		// log.Fatal 会打印错误信息，并且直接强行关闭整个服务器，因为连不上数据库后端也没必要跑了
		log.Fatal("救命，连不上 MySQL: ", err)
	}

	fmt.Println("MySQL 数据库连接成功！万能钥匙已就绪。")

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("get sql db failed: ", err)
	}

	// 配置连接池参数
	sqlDB.SetMaxOpenConns(config.GlobalConfig.SQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.GlobalConfig.SQL.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(config.GlobalConfig.SQL.ConnMaxLifetimeSeconds) * time.Second)

	// 3. 自动迁移（AutoMigrate）：把 Go 的结构体翻译成 MySQL 的建表语句
	// 把咱们刚刚建好的 User 图纸传进去
	// 把它修改成这样，让 GORM 一次性把所有表都建好：
	err = DB.AutoMigrate(
		&userModel.User{},
		&problemModel.Problem{}, // 新增：把题目图纸交给包工头
		&submissionModel.Submission{},
		&problemModel.TestCase{},
		&problemModel.Tag{},
		&analysisModel.AnalysisTask{},
		&analysisModel.AnalysisFeedback{},
		&studyPlanModel.StudyPlanTask{},
		&studyPlanModel.StudyPlanFeedback{},
	)
	if err != nil {
		log.Fatal("建表失败: ", err)
	}

	fmt.Println("数据库表结构已自动同步完毕！")
}
