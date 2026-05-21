package main

import (
	"fmt"
	"gojo/internal/app"
	"gojo/pkg/ai"
	"log"

	// 🔌 1. 引入基础设施 (按你的旧包名)
	"gojo/config"
	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	"gojo/infrastructure/search"
	"gojo/internal/judge/docker"

	// 🧱 2. 引入各个模块的 Repository (仓管)
	judgeRepo "gojo/internal/judge/repository"
	leaderboardRepo "gojo/internal/leaderboard/repository"
	problemRepo "gojo/internal/problem/repository"
	subRepo "gojo/internal/submission/repository"
	userRepo "gojo/internal/user/repository"

	// 🧠 3. 引入各个模块的 Service (大脑)
	judgeSvc "gojo/internal/judge/service"
	leaderboardSvc "gojo/internal/leaderboard/service"
	problemSvc "gojo/internal/problem/service"
	subSvc "gojo/internal/submission/service"
	userSvc "gojo/internal/user/service"

	// 🚪 4. 引入各个模块的 Handler (门卫)
	leaderboardHandler "gojo/internal/leaderboard/handler"
	problemHandler "gojo/internal/problem/handler"
	subHandler "gojo/internal/submission/handler"
	userHandler "gojo/internal/user/handler"

	judgeWorker "gojo/internal/judge/worker"
)

func main() {
	fmt.Println("🚀 正在启动 Gojo OJ 平台核心服务器...")

	// ==========================================
	// 🔌 第一阶段：通电！初始化所有基础设施
	// ==========================================
	config.InitConfig() // 必须放最前面！加载配置

	mysql.InitDB()    // 连接 MySQL
	cache.InitRedis() // 连接 Redis

	// 初始化 ES
	search.InitElasticsearch()

	// 初始化 Docker 引擎 (极其关键的防御)
	if err := docker.InitDockerClient(); err != nil {
		log.Fatalf("❌ 致命错误：Docker 引擎未准备就绪, 启动失败: %v", err)
	}

	// ==========================================
	// 🧱 第二阶段：招募底层仓管 (Repositories)
	// ==========================================
	ur := userRepo.NewUserRepository()
	pr := problemRepo.NewProblemRepository()
	sr := problemRepo.NewProblemSearchRepository() // ES 仓管
	subR := subRepo.NewSubmissionRepository()
	jr := judgeRepo.NewJudgeRepository()
	lr := leaderboardRepo.NewLeaderboardRepository()

	// ==========================================
	// 🧠 第三阶段：组装业务大脑 (Services) 及其外交官！
	// ==========================================

	// 1. 造 Judge (无依赖)
	judgeService := judgeSvc.NewJudgeService(jr)

	// 2. 造 Submission (依赖仓管和 AI。如果你还没写好 AIProvider 结构体，可以先传 nil)
	// var aiProvider subSvc.AIProvider = nil
	aiProvider := ai.NewAIProvider()
	submissionService := subSvc.NewSubmissionService(subR, aiProvider)

	// 3. 造 User (依赖 Submission 作为外交官提供 AC 列表)
	userService := userSvc.NewUserService(ur, submissionService)

	// 4. 造 Problem (依赖 MySQL 和 ES 两个仓管)
	problemService := problemSvc.NewProblemService(pr, sr)

	// 5. 造 Tag & TestCase (依赖各自的仓管)
	tagService := problemSvc.NewTagService(problemRepo.NewTagRepository())
	testCaseService := problemSvc.NewTestCaseService(problemRepo.NewTestCaseRepository())

	// 6. 造 Leaderboard (依赖 User 作为外交官提供名字)
	leaderboardService := leaderboardSvc.NewLeaderboardService(lr, userService)

	// ==========================================
	// 🛠️ 临时干预：全量同步数据到 ES
	// （替代你旧版直接调用 services.SyncAllProblemsToES 的写法）
	// ==========================================
	// err := problemService.SyncAllProblemsToES(context.Background())
	// if err != nil {
	// 	fmt.Printf("⚠️ ES 数据全量同步失败: %v\n", err)
	// }

	// ==========================================
	// 👷 第四阶段：安排后台包工头 (Worker)
	// ==========================================
	jw := judgeWorker.NewJudgeWorker(judgeService)
	jw.StartWorkerPool(3) // 启动 3 个并发工人，死盯队列

	// ==========================================
	// 🚪 第五阶段：安排前台迎宾门卫 (Handlers)
	// ==========================================
	uHandler := userHandler.NewUserHandler(userService)
	pHandler := problemHandler.NewProblemHandler(problemService)
	sHandler := subHandler.NewSubmissionHandler(submissionService)
	lHandler := leaderboardHandler.NewLeaderboardHandler(leaderboardService)
	tHandler := problemHandler.NewTagHandler(tagService)
	tcHandler := problemHandler.NewTestCaseHandler(testCaseService)
	searchHandler := problemHandler.NewSearchHandler(problemService)

	// ==========================================
	// 🌟 第六阶段：组装迎宾大厅 (Router) 并开门营业！
	// ==========================================
	r := app.SetupRouter(uHandler, pHandler, sHandler, lHandler, tHandler, tcHandler, searchHandler)

	fmt.Println("✅ 装配流水线完毕！服务器监听在 :8080 端口")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("❌ 服务器启动失败: %v", err)
	}
}
