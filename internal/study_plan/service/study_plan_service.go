package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sort"
	"strconv"
	"strings"

	"gojo/config"
	"gojo/internal/app/apperror"
	"gojo/internal/study_plan/dto"
	"gojo/internal/study_plan/model"
	"gojo/internal/study_plan/repository"
	problemModel "gojo/internal/problem/model"
	submissionModel "gojo/internal/submission/model"
	userDTO "gojo/internal/user/dto"
)

// UserProfileProvider 抽象出训练计划服务真正关心的用户画像能力。
// 这样后面无论是复用 userService，还是替换成别的实现，都不用改 StudyPlanService。
type UserProfileProvider interface {
	GetUserProfile(ctx context.Context, userID uint) (*userDTO.UserProfileResponse, error)
}

// SubmissionDataProvider 提供训练计划需要的提交历史数据。
type SubmissionDataProvider interface {
	GetAllSubmissionsByUserID(ctx context.Context, userID uint) ([]submissionModel.Submission, error)
	GetRecentFailedSubmissionsByUserID(ctx context.Context, userID uint, limit int) ([]submissionModel.Submission, error)
}

// ProblemCatalogProvider 提供训练计划需要的题目目录信息。
type ProblemCatalogProvider interface {
	GetAllProblemsWithTags(ctx context.Context) ([]problemModel.Problem, error)
	GetProblemByID(ctx context.Context, id string) (*problemModel.Problem, error)
}

type StudyPlanService struct {
	repo        repository.StudyPlanRepository
	userProfile UserProfileProvider
	submissions SubmissionDataProvider
	problems    ProblemCatalogProvider
}

func NewStudyPlanService(
	r repository.StudyPlanRepository,
	up UserProfileProvider,
	sub SubmissionDataProvider,
	pro ProblemCatalogProvider,
) *StudyPlanService {
	return &StudyPlanService{
		repo:        r,
		userProfile: up,
		submissions: sub,
		problems:    pro,
	}
}

// CreateTask 创建训练计划任务，并立刻把任务推进后台队列。
// 第一版先用 mock worker 结果把异步链路跑通，后面再替换成真正的 Python agent。
func (s *StudyPlanService) CreateTask(ctx context.Context, userID uint, goal string) (*model.StudyPlanTask, error) {
	modelName := config.GlobalConfig.AI.Model
	if strings.TrimSpace(modelName) == "" {
		modelName = "study-plan-agent"
	}

	task := &model.StudyPlanTask{
		UserID: userID,
		Goal:   goal,
		Status: model.TaskStatusPending,
		Model:  modelName,
	}

	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, err
	}

	queueTask := dto.StudyPlanQueueTask{
		TaskID: task.ID,
		UserID: task.UserID,
		Goal:   task.Goal,
	}

	taskBytes, err := json.Marshal(queueTask)
	if err != nil {
		return nil, err
	}

	if err := s.repo.PushToQueue(ctx, taskBytes); err != nil {
		return nil, err
	}

	log.Printf("study plan task queued: task_id=%d user_id=%d goal=%q\n", task.ID, task.UserID, task.Goal)

	return task, nil
}

// GetTask 查询一条训练计划任务，并限制只能查看自己的任务。
func (s *StudyPlanService) GetTask(ctx context.Context, userID uint, taskID uint) (*model.StudyPlanTask, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if task.UserID != userID {
		return nil, apperror.ErrForbidden
	}

	return task, nil
}

func (s *StudyPlanService) SubmitFeedback(ctx context.Context, userID uint, taskID uint, helpful bool, comment string) (*model.StudyPlanFeedback, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if task.UserID != userID {
		return nil, apperror.ErrForbidden
	}

	feedback := &model.StudyPlanFeedback{
		TaskID:  taskID,
		UserID:  userID,
		Helpful: helpful,
		Comment: comment,
	}

	if err := s.repo.UpsertFeedback(ctx, feedback); err != nil {
		return nil, err
	}

	return feedback, nil
}

func (s *StudyPlanService) GetFeedback(ctx context.Context, userID uint, taskID uint) (*model.StudyPlanFeedback, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if task.UserID != userID {
		return nil, apperror.ErrForbidden
	}

	return s.repo.GetFeedbackByTaskIDAndUserID(ctx, taskID, userID)
}

func (s *StudyPlanService) GetAdminStats(ctx context.Context) (*dto.StudyPlanAdminStatsResponse, error) {
	return s.repo.GetAdminStats(ctx)
}

// GetUserACHistory 返回用户的 AC 历史，用作第一个 internal agent tool。
func (s *StudyPlanService) GetUserACHistory(ctx context.Context, userID uint) (*dto.UserACHistoryResponse, error) {
	if s.userProfile == nil {
		return nil, errors.New("user profile provider not configured")
	}

	profile, err := s.userProfile.GetUserProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &dto.UserACHistoryResponse{
		UserID:           profile.ID,
		Username:         profile.Username,
		SolvedCount:      profile.SolvedCount,
		SolvedProblemIDs: profile.SolvedList,
	}, nil
}

// GetUserFailedSubmissions 返回用户最近失败的提交，给后续 agent 分析薄弱点使用。
func (s *StudyPlanService) GetUserFailedSubmissions(ctx context.Context, userID uint, limit int) (*dto.UserFailedSubmissionsResponse, error) {
	if s.submissions == nil || s.problems == nil {
		return nil, errors.New("study plan providers not configured")
	}

	submissions, err := s.submissions.GetRecentFailedSubmissionsByUserID(ctx, userID, limit)
	if err != nil {
		return nil, err
	}

	problemMap, err := s.getProblemMap(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]dto.FailedSubmissionItem, 0, len(submissions))
	for _, sub := range submissions {
		title := ""
		if p, ok := problemMap[sub.ProblemID]; ok {
			title = p.Title
		}

		items = append(items, dto.FailedSubmissionItem{
			SubmissionID: sub.ID,
			ProblemID:    sub.ProblemID,
			ProblemTitle: title,
			Status:       sub.Status,
			Language:     sub.Language,
			ActualOutput: sub.ActualOutput,
			CreatedAt:    sub.CreatedAt,
		})
	}

	return &dto.UserFailedSubmissionsResponse{
		UserID: userID,
		Items:  items,
	}, nil
}

// GetUserTagStats 统计用户在各标签下的提交和解题情况。
// 这份数据适合后面的 Python agent 用来判断当前薄弱点。
func (s *StudyPlanService) GetUserTagStats(ctx context.Context, userID uint) (*dto.UserTagStatsResponse, error) {
	if s.userProfile == nil || s.submissions == nil || s.problems == nil {
		return nil, errors.New("study plan providers not configured")
	}

	profile, err := s.userProfile.GetUserProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	submissions, err := s.submissions.GetAllSubmissionsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	problemMap, err := s.getProblemMap(ctx)
	if err != nil {
		return nil, err
	}

	type stat struct {
		total  int
		failed int
		solved int
	}

	stats := make(map[string]*stat)
	for _, sub := range submissions {
		problem, ok := problemMap[sub.ProblemID]
		if !ok {
			continue
		}

		for _, tag := range problem.Tags {
			if _, exists := stats[tag.Name]; !exists {
				stats[tag.Name] = &stat{}
			}
			stats[tag.Name].total++
			if sub.Status != "AC" {
				stats[tag.Name].failed++
			}
		}
	}

	solvedSet := make(map[uint]struct{}, len(profile.SolvedList))
	for _, problemID := range profile.SolvedList {
		solvedSet[problemID] = struct{}{}
	}

	for problemID := range solvedSet {
		problem, ok := problemMap[problemID]
		if !ok {
			continue
		}

		for _, tag := range problem.Tags {
			if _, exists := stats[tag.Name]; !exists {
				stats[tag.Name] = &stat{}
			}
			stats[tag.Name].solved++
		}
	}

	items := make([]dto.TagStatItem, 0, len(stats))
	for tagName, statItem := range stats {
		items = append(items, dto.TagStatItem{
			TagName:           tagName,
			TotalSubmissions:  statItem.total,
			FailedSubmissions: statItem.failed,
			SolvedProblems:    statItem.solved,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].FailedSubmissions == items[j].FailedSubmissions {
			return items[i].TotalSubmissions > items[j].TotalSubmissions
		}
		return items[i].FailedSubmissions > items[j].FailedSubmissions
	})

	return &dto.UserTagStatsResponse{
		UserID: userID,
		Tags:   items,
	}, nil
}

// GetCandidateProblems 根据标签和排除列表返回候选题，作为推荐系统的候选集生成步骤。
func (s *StudyPlanService) GetCandidateProblems(ctx context.Context, requestedTags []string, excludeProblemIDs []uint, limit int) (*dto.CandidateProblemsResponse, error) {
	if s.problems == nil {
		return nil, errors.New("problem provider not configured")
	}

	if limit <= 0 {
		limit = 10
	}

	problems, err := s.problems.GetAllProblemsWithTags(ctx)
	if err != nil {
		return nil, err
	}

	requestedTagSet := make(map[string]struct{}, len(requestedTags))
	for _, tag := range requestedTags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		requestedTagSet[strings.ToLower(tag)] = struct{}{}
	}

	excludeSet := make(map[uint]struct{}, len(excludeProblemIDs))
	for _, id := range excludeProblemIDs {
		excludeSet[id] = struct{}{}
	}

	type candidate struct {
		item      dto.CandidateProblemItem
		matchHits int
	}

	candidates := make([]candidate, 0)
	for _, problem := range problems {
		if _, excluded := excludeSet[problem.ID]; excluded {
			continue
		}

		tagNames := make([]string, 0, len(problem.Tags))
		matchHits := 0
		for _, tag := range problem.Tags {
			tagNames = append(tagNames, tag.Name)
			if _, ok := requestedTagSet[strings.ToLower(tag.Name)]; ok {
				matchHits++
			}
		}

		if len(requestedTagSet) > 0 && matchHits == 0 {
			continue
		}

		candidates = append(candidates, candidate{
			item: dto.CandidateProblemItem{
				ProblemID:     problem.ID,
				Title:         problem.Title,
				Description:   problem.Description,
				TagNames:      tagNames,
				SubmitCount:   problem.SubmitCount,
				AcceptedCount: problem.AcceptedCount,
			},
			matchHits: matchHits,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].matchHits == candidates[j].matchHits {
			return candidates[i].item.SubmitCount < candidates[j].item.SubmitCount
		}
		return candidates[i].matchHits > candidates[j].matchHits
	})

	items := make([]dto.CandidateProblemItem, 0, min(limit, len(candidates)))
	for i := 0; i < len(candidates) && i < limit; i++ {
		items = append(items, candidates[i].item)
	}

	return &dto.CandidateProblemsResponse{
		RequestedTags: requestedTags,
		Items:         items,
	}, nil
}

// GetProblemDetail 返回单题详情，后面 agent 需要补更多题目上下文时可以直接复用。
func (s *StudyPlanService) GetProblemDetail(ctx context.Context, problemID uint) (*dto.ProblemDetailResponse, error) {
	if s.problems == nil {
		return nil, errors.New("problem provider not configured")
	}

	problem, err := s.problems.GetProblemByID(ctx, uintToString(problemID))
	if err != nil {
		return nil, err
	}

	tagNames := make([]string, 0, len(problem.Tags))
	for _, tag := range problem.Tags {
		tagNames = append(tagNames, tag.Name)
	}

	return &dto.ProblemDetailResponse{
		ProblemID:     problem.ID,
		Title:         problem.Title,
		Description:   problem.Description,
		TimeLimit:     problem.TimeLimit,
		MemoryLimit:   problem.MemoryLimit,
		SubmitCount:   problem.SubmitCount,
		AcceptedCount: problem.AcceptedCount,
		TagNames:      tagNames,
	}, nil
}

func (s *StudyPlanService) getProblemMap(ctx context.Context) (map[uint]problemModel.Problem, error) {
	problems, err := s.problems.GetAllProblemsWithTags(ctx)
	if err != nil {
		return nil, err
	}

	problemMap := make(map[uint]problemModel.Problem, len(problems))
	for _, problem := range problems {
		problemMap[problem.ID] = problem
	}

	return problemMap, nil
}

func uintToString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
