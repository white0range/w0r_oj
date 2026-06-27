package app

import (
	analysisHandler "gojo/internal/analysis/handler"
	middlewares2 "gojo/internal/app/middlewares"
	"net/http"

	leaderboardHandler "gojo/internal/leaderboard/handler"
	problemHandler "gojo/internal/problem/handler"
	studyPlanHandler "gojo/internal/study_plan/handler"
	subHandler "gojo/internal/submission/handler"
	userHandler "gojo/internal/user/handler"

	"github.com/gin-gonic/gin"
)

// SetupRouter 负责配置所有的 API 路由地址
func SetupRouter(
	uHandler *userHandler.UserHandler,
	pHandler *problemHandler.ProblemHandler,
	sHandler *subHandler.SubmissionHandler,
	lHandler *leaderboardHandler.LeaderboardHandler,
	tHandler *problemHandler.TagHandler,
	tcHandler *problemHandler.TestCaseHandler,
	searchHandler *problemHandler.SearchHandler,
	analysisHandler *analysisHandler.AnalysisHandler,
	spHandler *studyPlanHandler.StudyPlanHandler,
) *gin.Engine {
	r := gin.Default()

	// ====================
	// 公共区域：不需要手环谁都能进
	// ====================
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "欢迎来到我的 OJ 平台，核心引擎运转正常！",
		})
	})

	r.POST("/api/register", uHandler.Register)
	r.POST("/api/login", uHandler.Login)

	r.GET("/api/problems", pHandler.GetProblemList)
	r.GET("/api/problems/:id", pHandler.GetProblemDetail)

	r.GET("/api/tags", tHandler.GetTagList)
	r.GET("/api/leaderboard", middlewares2.OptionalAuth(), lHandler.GetGlobalLeaderboard)

	r.POST("/api/problems/search", searchHandler.SearchProblems)

	// ====================
	// 核心区域：必须通过安检 (使用中间件)
	// ====================
	protected := r.Group("/api")
	protected.Use(middlewares2.AuthMiddleware())
	{
		// ==========================================
		// 👑 皇家禁地：管理员专属操作台
		// ==========================================
		adminGroup := protected.Group("/admin")
		adminGroup.Use(middlewares2.AdminCheck())
		{
			adminGroup.POST("/problems", pHandler.CreateProblem)
			adminGroup.PUT("/problems/:id", pHandler.UpdateProblem)
			adminGroup.DELETE("/problems/:id", pHandler.DeleteProblem)

			adminGroup.GET("/problems/:id/cases", tcHandler.GetTestCases)
			adminGroup.POST("/problems/:id/cases", tcHandler.AddTestCase)
			adminGroup.DELETE("/problems/cases/:case_id", tcHandler.DeleteTestCase)

			adminGroup.POST("/tags", tHandler.CreateTag)
			adminGroup.DELETE("/tags/:id", tHandler.DeleteTag)
			adminGroup.PUT("/problems/:id/tags", pHandler.UpdateProblemTags)
			adminGroup.GET("/analysis/stats", analysisHandler.GetAdminStats)
			adminGroup.GET("/study-plan/stats", spHandler.GetAdminStats)

			adminGroup.GET("/agent/users/:id/ac-history", spHandler.GetUserACHistory)
			adminGroup.GET("/agent/users/:id/failed-submissions", spHandler.GetUserFailedSubmissions)
			adminGroup.GET("/agent/users/:id/tag-stats", spHandler.GetUserTagStats)
			adminGroup.GET("/agent/problems/candidates", spHandler.GetCandidateProblems)
			adminGroup.GET("/agent/problems/:id", spHandler.GetProblemDetail)
		}

		protected.GET("/profile", uHandler.GetProfile)

		protected.POST("/submit", middlewares2.SubmitRateLimit(), sHandler.SubmitCode)
		protected.GET("/submissions/:id", sHandler.GetSubmissionResult)
		protected.GET("/my-submissions", sHandler.GetMySubmissions)

		protected.GET("/ws", uHandler.ConnectWS)
		// AI 诊断任务
		protected.POST("/analysis/tasks", analysisHandler.CreateAnalysisTask)
		protected.GET("/analysis/tasks/:id", analysisHandler.GetAnalysisTask)
		protected.POST("/analysis/tasks/:id/feedback", analysisHandler.SubmitFeedback)
		protected.GET("/analysis/tasks/:id/feedback", analysisHandler.GetFeedback)
		protected.POST("/study-plan/tasks", spHandler.CreateTask)
		protected.GET("/study-plan/tasks/:id/stream", spHandler.StreamTask)
		protected.GET("/study-plan/tasks/:id", spHandler.GetTask)
		protected.POST("/study-plan/tasks/:id/feedback", spHandler.SubmitFeedback)
		protected.GET("/study-plan/tasks/:id/feedback", spHandler.GetFeedback)
	}

	return r
}
