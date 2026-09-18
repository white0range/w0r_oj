package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gojo/config"
	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	"gojo/internal/judge/docker"
	judgeRepo "gojo/internal/judge/repository"
	judgeSandbox "gojo/internal/judge/sandbox"
	judgeSvc "gojo/internal/judge/service"
	judgeWorker "gojo/internal/judge/worker"
	subRepo "gojo/internal/submission/repository"
	"gojo/internal/syncer"
)

const (
	orphanCleanupTimeout  = 30 * time.Second
	workerShutdownTimeout = 150 * time.Second
)

func main() {
	fmt.Println("starting Gojo judge worker...")

	shutdownSignalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	config.InitJudgeWorkerConfig()
	mysql.InitDBWithoutMigration()
	cache.InitRedis()
	if err := docker.InitDockerClient(); err != nil {
		log.Fatalf("docker client init failed: %v", err)
	}

	cleanupCtx, cancelCleanup := context.WithTimeout(shutdownSignalCtx, orphanCleanupTimeout)
	cleanupSummary, err := judgeSandbox.CleanupOrphanedResources(cleanupCtx)
	cancelCleanup()
	if err != nil {
		log.Fatalf("cleanup orphaned judge resources failed: %v", err)
	}
	if cleanupSummary.ContainersRemoved > 0 || cleanupSummary.WorkDirsRemoved > 0 {
		log.Printf(
			"cleaned orphaned judge resources: containers=%d workdirs=%d",
			cleanupSummary.ContainersRemoved,
			cleanupSummary.WorkDirsRemoved,
		)
	}

	submissions := subRepo.NewSubmissionRepository()
	judgeRepository := judgeRepo.NewJudgeRepository(syncer.NewProducer())
	judgeService := judgeSvc.NewJudgeService(judgeRepository)
	worker := judgeWorker.NewJudgeWorker(judgeService, submissions)
	worker.StartWorkerPool(config.GlobalConfig.Judge.WorkerCount)
	log.Printf("judge worker started: workers=%d", config.GlobalConfig.Judge.WorkerCount)

	<-shutdownSignalCtx.Done()
	log.Printf("shutdown signal received; stopping judge task consumption")
	worker.StopAccepting()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), workerShutdownTimeout)
	defer cancelShutdown()
	if err := worker.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown judge workers: %v", err)
	}

	closeInfrastructure()
	log.Printf("Gojo judge worker stopped")
}

func closeInfrastructure() {
	if err := docker.Close(); err != nil {
		log.Printf("close Docker client: %v", err)
	}
	if err := cache.Close(); err != nil {
		log.Printf("close Redis: %v", err)
	}
	if err := mysql.Close(); err != nil {
		log.Printf("close MySQL: %v", err)
	}
}
