package main

import (
	"context"
	"fmt"
	"log"

	"gojo/config"
	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	"gojo/infrastructure/search"
	analysisHandler "gojo/internal/analysis/handler"
	analysisRepo "gojo/internal/analysis/repository"
	analysisSvc "gojo/internal/analysis/service"
	analysisWorker "gojo/internal/analysis/worker"
	"gojo/internal/app"
	"gojo/internal/judge/docker"
	judgeRepo "gojo/internal/judge/repository"
	judgeSvc "gojo/internal/judge/service"
	judgeWorker "gojo/internal/judge/worker"
	leaderboardHandler "gojo/internal/leaderboard/handler"
	leaderboardRepo "gojo/internal/leaderboard/repository"
	leaderboardSvc "gojo/internal/leaderboard/service"
	problemHandler "gojo/internal/problem/handler"
	problemRepo "gojo/internal/problem/repository"
	problemSvc "gojo/internal/problem/service"
	studyPlanHandler "gojo/internal/study_plan/handler"
	studyPlanRepo "gojo/internal/study_plan/repository"
	studyPlanSvc "gojo/internal/study_plan/service"
	studyPlanWorker "gojo/internal/study_plan/worker"
	subHandler "gojo/internal/submission/handler"
	subRepo "gojo/internal/submission/repository"
	subSvc "gojo/internal/submission/service"
	"gojo/internal/syncer"
	userHandler "gojo/internal/user/handler"
	userRepo "gojo/internal/user/repository"
	userSvc "gojo/internal/user/service"
	"gojo/pkg/ai"
)

func main() {
	fmt.Println("starting Gojo backend...")

	config.InitConfig()
	mysql.InitDB()
	cache.InitRedis()
	search.InitElasticsearch()

	if err := docker.InitDockerClient(); err != nil {
		log.Fatalf("docker client init failed: %v", err)
	}

	ur := userRepo.NewUserRepository()
	usr := userRepo.NewRefreshSessionRepository()
	pr := problemRepo.NewProblemRepository()
	sr := problemRepo.NewProblemSearchRepository()
	subR := subRepo.NewSubmissionRepository()
	syncManager := syncer.NewManager(pr, sr)
	syncManager.Start(context.Background())

	jr := judgeRepo.NewJudgeRepository(syncManager)
	lr := leaderboardRepo.NewLeaderboardRepository()
	ar := analysisRepo.NewAnalysisRepository()
	spr := studyPlanRepo.NewStudyPlanRepository()

	judgeService := judgeSvc.NewJudgeService(jr)

	aiProvider := ai.NewAIProvider()
	submissionService := subSvc.NewSubmissionService(subR)
	userService := userSvc.NewUserService(ur, usr, submissionService)
	problemService := problemSvc.NewProblemService(pr, sr, syncManager)
	tagService := problemSvc.NewTagService(problemRepo.NewTagRepository(), syncManager)
	testCaseService := problemSvc.NewTestCaseService(problemRepo.NewTestCaseRepository(), syncManager)
	leaderboardService := leaderboardSvc.NewLeaderboardService(lr, userService)
	analysisService := analysisSvc.NewAnalysisService(ar, subR)
	studyPlanService := studyPlanSvc.NewStudyPlanService(spr, userService, subR, pr)

	jw := judgeWorker.NewJudgeWorker(judgeService)
	jw.StartWorkerPool(config.GlobalConfig.Judge.WorkerCount)

	aw := analysisWorker.NewAnalysisWorker(ar, subR, pr, aiProvider)
	aw.StartWorkerPool(3)

	spw, err := studyPlanWorker.NewStudyPlanWorker(spr)
	if err != nil {
		log.Fatalf("study plan worker init failed: %v", err)
	}
	spw.StartWorkerPool(config.GlobalConfig.StudyPlan.WorkerCount)
	spw.StartTurnWorkerPool(config.GlobalConfig.StudyPlan.WorkerCount)

	uHandler := userHandler.NewUserHandler(userService)
	pHandler := problemHandler.NewProblemHandler(problemService)
	sHandler := subHandler.NewSubmissionHandler(submissionService)
	lHandler := leaderboardHandler.NewLeaderboardHandler(leaderboardService)
	tHandler := problemHandler.NewTagHandler(tagService)
	tcHandler := problemHandler.NewTestCaseHandler(testCaseService)
	searchHandler := problemHandler.NewSearchHandler(problemService)
	aHandler := analysisHandler.NewAnalysisHandler(analysisService)
	spHandler := studyPlanHandler.NewStudyPlanHandler(studyPlanService)

	r := app.SetupRouter(
		uHandler,
		pHandler,
		sHandler,
		lHandler,
		tHandler,
		tcHandler,
		searchHandler,
		aHandler,
		spHandler,
	)

	addr := fmt.Sprintf(":%d", config.GlobalConfig.Server.Port)
	fmt.Printf("server listening on %s\n", addr)

	if err := r.Run(addr); err != nil {
		log.Fatalf("server start failed: %v", err)
	}
}
