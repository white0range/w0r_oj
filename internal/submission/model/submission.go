package model

import "gorm.io/gorm"

// Submission 提交记录表
type Submission struct {
	gorm.Model

	// 核心外键：记录是谁提交的，提交了哪道题
	UserID    uint `gorm:"index" json:"user_id"`    // 加上 index 索引，方便以后查询“我的提交记录”
	ProblemID uint `gorm:"index" json:"problem_id"` // 加上 index 索引，方便以后查询“这道题的所有提交”

	// 提交的内容
	Language string `gorm:"type:varchar(50);not null" json:"language"` // 编程语言，比如 "go", "java", "python"
	Code     string `gorm:"type:text;not null" json:"code"`            // 玩家写的满腔热血的源代码

	// 判题结果（系统的命脉）
	// 默认值是 "Pending"（排队等待评测中）
	Status     string `gorm:"type:varchar(50);default:'Pending'" json:"status"`
	TimeCost   int    `json:"time_cost"`   // 实际运行耗时 (ms)
	MemoryCost int    `json:"memory_cost"` // 实际消耗内存 (KB)

	// New results store a fixed verdict message. Older rows may contain raw output.
	ActualOutput string `gorm:"type:text" json:"actual_output"`
}
