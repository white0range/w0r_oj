package app

import (
	"net/http"

	"github.com/gin-gonic/gin"

	analysisHandler "gojo/internal/analysis/handler"
	middlewares2 "gojo/internal/app/middlewares"
	chatHandler "gojo/internal/chat/handler"
	leaderboardHandler "gojo/internal/leaderboard/handler"
	problemHandler "gojo/internal/problem/handler"
	subHandler "gojo/internal/submission/handler"
	userHandler "gojo/internal/user/handler"
)

func SetupRouter(
	uHandler *userHandler.UserHandler,
	pHandler *problemHandler.ProblemHandler,
	sHandler *subHandler.SubmissionHandler,
	lHandler *leaderboardHandler.LeaderboardHandler,
	tHandler *problemHandler.TagHandler,
	tcHandler *problemHandler.TestCaseHandler,
	searchHandler *problemHandler.SearchHandler,
	analysisHandler *analysisHandler.AnalysisHandler,
	chatHandler *chatHandler.ChatHandler,
) *gin.Engine {
	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "welcome to gojo oj",
		})
	})

	r.POST("/api/register", uHandler.Register)
	r.POST("/api/login", uHandler.Login)
	r.POST("/api/refresh", uHandler.Refresh)
	r.POST("/api/logout", uHandler.Logout)

	r.GET("/api/problems", pHandler.GetProblemList)
	r.GET("/api/problems/:id", pHandler.GetProblemDetail)
	r.GET("/api/tags", tHandler.GetTagList)
	r.GET("/api/leaderboard", middlewares2.OptionalAuth(), lHandler.GetGlobalLeaderboard)
	r.POST("/api/problems/search", searchHandler.SearchProblems)

	protected := r.Group("/api")
	protected.Use(middlewares2.AuthMiddleware())
	{
		adminGroup := protected.Group("/admin")
		adminGroup.Use(middlewares2.AdminCheck())
		{
			adminGroup.GET("/users", uHandler.AdminListUsers)
			adminGroup.POST("/users/:id/ban", uHandler.AdminBanUser)
			adminGroup.POST("/users/:id/unban", uHandler.AdminUnbanUser)
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

			adminGroup.GET("/agent/users/:id/ac-history", chatHandler.GetUserACHistory)
			adminGroup.GET("/agent/users/:id/failed-submissions", chatHandler.GetUserFailedSubmissions)
			adminGroup.GET("/agent/users/:id/tag-stats", chatHandler.GetUserTagStats)
			adminGroup.GET("/agent/problems/candidates", chatHandler.GetCandidateProblems)
			adminGroup.GET("/agent/problems/:id", chatHandler.GetProblemDetail)
		}

		protected.GET("/profile", uHandler.GetProfile)

		protected.POST("/submit", middlewares2.SubmitRateLimit(), sHandler.SubmitCode)
		protected.GET("/submissions/:id", sHandler.GetSubmissionResult)
		protected.GET("/my-submissions", sHandler.GetMySubmissions)

		protected.GET("/ws", uHandler.ConnectWS)

		protected.POST("/analysis/tasks", analysisHandler.CreateAnalysisTask)
		protected.GET("/analysis/tasks/:id", analysisHandler.GetAnalysisTask)
		protected.POST("/analysis/tasks/:id/feedback", analysisHandler.SubmitFeedback)
		protected.GET("/analysis/tasks/:id/feedback", analysisHandler.GetFeedback)

		protected.GET("/chat/sessions", chatHandler.ListSessions)
		protected.POST("/chat/sessions", chatHandler.CreateSession)
		protected.GET("/chat/sessions/:session_id", chatHandler.GetSession)
		protected.DELETE("/chat/sessions/:session_id", chatHandler.DeleteSession)
		protected.GET("/chat/sessions/:session_id/messages", chatHandler.ListMessages)
		protected.POST("/chat/sessions/:session_id/messages", chatHandler.SendMessage)
		protected.GET("/chat/turns/:turn_id", chatHandler.GetTurn)
		protected.GET("/chat/turns/:turn_id/stream", chatHandler.StreamTurn)
		protected.POST("/chat/turns/:turn_id/feedback", chatHandler.SubmitPlanFeedback)
		protected.GET("/chat/turns/:turn_id/feedback", chatHandler.GetPlanFeedback)
	}

	return r
}
