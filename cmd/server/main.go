package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gojo/config"
	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	"gojo/infrastructure/search"
	"gojo/internal/app"
	chatHandler "gojo/internal/chat/handler"
	chatRepo "gojo/internal/chat/repository"
	chatSvc "gojo/internal/chat/service"
	chatWorker "gojo/internal/chat/worker"
	leaderboardHandler "gojo/internal/leaderboard/handler"
	leaderboardRepo "gojo/internal/leaderboard/repository"
	leaderboardSvc "gojo/internal/leaderboard/service"
	problemHandler "gojo/internal/problem/handler"
	problemRepo "gojo/internal/problem/repository"
	problemSvc "gojo/internal/problem/service"
	"gojo/internal/realtime"
	subHandler "gojo/internal/submission/handler"
	subRepo "gojo/internal/submission/repository"
	subSvc "gojo/internal/submission/service"
	"gojo/internal/syncer"
	userHandler "gojo/internal/user/handler"
	userRepo "gojo/internal/user/repository"
	userSvc "gojo/internal/user/service"
)

const (
	httpShutdownTimeout       = 30 * time.Second
	backgroundShutdownTimeout = 150 * time.Second
)

type shutdownResult struct {
	component string
	err       error
}

func main() {
	fmt.Println("starting Gojo backend...")

	shutdownSignalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	config.InitConfig()
	mysql.InitDB()
	cache.InitRedis()
	search.InitElasticsearch()
	realtimeBridgeDone, err := realtime.StartDistributedBridge(shutdownSignalCtx)
	if err != nil {
		log.Fatalf("start realtime event bridge failed: %v", err)
	}

	ur := userRepo.NewUserRepository()
	usr := userRepo.NewRefreshSessionRepository()
	pr := problemRepo.NewProblemRepository()
	sr := problemRepo.NewProblemSearchRepository()
	subR := subRepo.NewSubmissionRepository()
	syncManager := syncer.NewManager(pr, sr)
	syncManager.Start(shutdownSignalCtx)

	lr := leaderboardRepo.NewLeaderboardRepository()
	cr := chatRepo.NewChatRepository()

	submissionService := subSvc.NewSubmissionService(subR, pr)
	userService := userSvc.NewUserService(ur, usr, submissionService)
	problemService := problemSvc.NewProblemService(pr, sr, syncManager)
	tagService := problemSvc.NewTagService(problemRepo.NewTagRepository(), syncManager)
	testCaseService := problemSvc.NewTestCaseService(problemRepo.NewTestCaseRepository(), syncManager)
	leaderboardService := leaderboardSvc.NewLeaderboardService(lr, userService)
	chatService := chatSvc.NewChatService(cr, userService, subR, pr)

	cw, err := chatWorker.NewChatWorker(cr)
	if err != nil {
		log.Fatalf("chat worker init failed: %v", err)
	}
	cw.StartTurnWorkerPool(config.GlobalConfig.Chat.WorkerCount)

	uHandler := userHandler.NewUserHandler(userService)
	pHandler := problemHandler.NewProblemHandler(problemService)
	sHandler := subHandler.NewSubmissionHandler(submissionService)
	lHandler := leaderboardHandler.NewLeaderboardHandler(leaderboardService)
	tHandler := problemHandler.NewTagHandler(tagService)
	tcHandler := problemHandler.NewTestCaseHandler(testCaseService)
	searchHandler := problemHandler.NewSearchHandler(problemService)
	cHandler := chatHandler.NewChatHandler(chatService)

	r := app.SetupRouter(
		uHandler,
		pHandler,
		sHandler,
		lHandler,
		tHandler,
		tcHandler,
		searchHandler,
		cHandler,
	)

	addr := fmt.Sprintf(":%d", config.GlobalConfig.Server.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: secondsDuration(config.GlobalConfig.Server.ReadHeaderTimeoutSeconds),
		ReadTimeout:       secondsDuration(config.GlobalConfig.Server.ReadTimeoutSeconds),
		WriteTimeout:      secondsDuration(config.GlobalConfig.Server.WriteTimeoutSeconds),
		IdleTimeout:       secondsDuration(config.GlobalConfig.Server.IdleTimeoutSeconds),
		MaxHeaderBytes:    config.GlobalConfig.Server.MaxHeaderBytes,
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()
	fmt.Printf("server listening on %s\n", addr)

	select {
	case <-shutdownSignalCtx.Done():
		log.Printf("shutdown signal received; stopping new requests and queue consumption")
	case err := <-serverErr:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return
		}
		log.Printf("server stopped unexpectedly: %v", err)
		stopSignals()
	}

	httpShutdownCtx, cancelHTTP := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancelHTTP()
	backgroundShutdownCtx, cancelBackground := context.WithTimeout(context.Background(), backgroundShutdownTimeout)
	defer cancelBackground()

	// Stop queue consumption before waiting. Already claimed work keeps using
	// its task context until the shutdown deadline is reached.
	cw.StopAccepting()

	results := make(chan shutdownResult, 4)
	go func() { results <- shutdownResult{"http server", shutdownHTTPServer(httpShutdownCtx, server)} }()
	go func() { results <- shutdownResult{"chat workers", cw.Shutdown(backgroundShutdownCtx)} }()
	go func() { results <- shutdownResult{"sync workers", syncManager.Wait(backgroundShutdownCtx)} }()
	go func() { results <- shutdownResult{"realtime event bridge", <-realtimeBridgeDone} }()

	for i := 0; i < cap(results); i++ {
		result := <-results
		if result.err != nil && !errors.Is(result.err, http.ErrServerClosed) {
			log.Printf("graceful shutdown %s: %v", result.component, result.err)
		}
	}

	closeInfrastructure()
	log.Printf("Gojo backend stopped")
}

func shutdownHTTPServer(ctx context.Context, server *http.Server) error {
	if err := server.Shutdown(ctx); err != nil {
		// Shutdown waits for SSE and other in-flight requests. Once its deadline
		// expires, close the remaining connections so the process can exit.
		closeErr := server.Close()
		if closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
			return errors.Join(err, closeErr)
		}
		return err
	}
	return nil
}

func closeInfrastructure() {
	if err := cache.Close(); err != nil {
		log.Printf("close Redis: %v", err)
	}
	if err := mysql.Close(); err != nil {
		log.Printf("close MySQL: %v", err)
	}
}

func secondsDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
