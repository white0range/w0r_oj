package app

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gojo/config"
	middlewares2 "gojo/internal/app/middlewares"
	chatHandler "gojo/internal/chat/handler"
	chatRepository "gojo/internal/chat/repository"
	leaderboardHandler "gojo/internal/leaderboard/handler"
	problemHandler "gojo/internal/problem/handler"
	"gojo/internal/realtime"
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
	chatHandler *chatHandler.ChatHandler,
	agentHandler *chatHandler.AgentToolHandler,
) *gin.Engine {
	r := gin.Default()
	if err := r.SetTrustedProxies(config.GlobalConfig.Server.TrustedProxyCIDRs); err != nil {
		panic("invalid server.trusted_proxy_cidrs: " + err.Error())
	}
	r.Use(middlewares2.RequestBodyLimit(config.GlobalConfig.Server.MaxRequestBodyBytes))
	r.Use(middlewares2.PublicIPRateLimit())
	realtimeHandler := realtime.NewHandler()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "welcome to gojo oj",
		})
	})

	r.POST("/api/register", middlewares2.RegisterRateLimit(), uHandler.Register)
	r.POST("/api/login", middlewares2.LoginIPRateLimit(), uHandler.Login)
	r.POST("/api/refresh", uHandler.Refresh)
	r.POST("/api/logout", uHandler.Logout)

	r.GET("/api/problems", pHandler.GetProblemList)
	r.GET("/api/problems/:id", pHandler.GetProblemDetail)
	r.GET("/api/tags", tHandler.GetTagList)
	r.GET("/api/leaderboard", middlewares2.OptionalAuth(), lHandler.GetGlobalLeaderboard)
	r.POST("/api/problems/search", middlewares2.SearchRateLimit(), searchHandler.SearchProblems)

	agentGroup := r.Group("/api/agent")
	agentGroup.Use(middlewares2.AgentAuth(), middlewares2.AgentTurnContext(chatRepository.NewChatRepository()))
	{
		agentGroup.GET("/context", agentHandler.GetExecutionContext)
		agentGroup.POST("/trials", agentHandler.RunTrial)
		agentGroup.GET("/me/ac-history", agentHandler.GetUserACHistory)
		agentGroup.GET("/me/failed-submissions", agentHandler.GetUserFailedSubmissions)
		agentGroup.GET("/me/failed-submissions/:submission_id", agentHandler.GetFailedSubmissionDetail)
		agentGroup.GET("/me/tag-stats", agentHandler.GetUserTagStats)
		agentGroup.GET("/problems/candidates", agentHandler.GetCandidateProblems)
		agentGroup.GET("/problems/:id", agentHandler.GetProblemDetail)
	}

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
		}

		protected.GET("/profile", uHandler.GetProfile)

		protected.POST("/problems/:id/run", middlewares2.RunCodeRateLimit(), pHandler.RunCode)
		protected.POST("/submit", middlewares2.SubmitRateLimit(), sHandler.SubmitCode)
		protected.GET("/submissions/:id", sHandler.GetSubmissionResult)
		protected.GET("/my-submissions", sHandler.GetMySubmissions)

		protected.POST("/events/ticket", middlewares2.SSETicketRateLimit(), realtimeHandler.CreateEventsTicket)

		protected.GET("/chat/sessions", chatHandler.ListSessions)
		protected.POST("/chat/sessions", chatHandler.CreateSession)
		protected.GET("/chat/sessions/:session_id", chatHandler.GetSession)
		protected.DELETE("/chat/sessions/:session_id", chatHandler.DeleteSession)
		protected.GET("/chat/sessions/:session_id/messages", chatHandler.ListMessages)
		protected.POST("/chat/sessions/:session_id/messages", middlewares2.ChatMessageRateLimit(), chatHandler.SendMessage)
		protected.GET("/chat/turns/:turn_id", chatHandler.GetTurn)
		protected.POST("/chat/turns/:turn_id/stream-ticket", middlewares2.SSETicketRateLimit(), chatHandler.CreateStreamTicket)
		protected.POST("/chat/turns/:turn_id/feedback", chatHandler.SubmitPlanFeedback)
		protected.GET("/chat/turns/:turn_id/feedback", chatHandler.GetPlanFeedback)
	}

	r.GET("/api/events", middlewares2.RealtimeTicketAuth(realtime.ScopeEvents, ""), realtimeHandler.StreamEvents)
	r.GET("/api/chat/turns/:turn_id/stream", middlewares2.RealtimeTicketAuth(realtime.ScopeChatTurn, "turn_id"), chatHandler.StreamTurn)

	return r
}
