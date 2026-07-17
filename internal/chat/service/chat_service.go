package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"gojo/config"
	"gojo/internal/app/apperror"
	"gojo/internal/chat/dto"
	"gojo/internal/chat/model"
	"gojo/internal/chat/repository"
	problemModel "gojo/internal/problem/model"
	submissionModel "gojo/internal/submission/model"
	userDTO "gojo/internal/user/dto"

	"gorm.io/gorm"
)

// UserProfileProvider 閹跺€熻杽閸戦缚顔勭紒鍐吀閸掓帗婀囬崝锛勬埂濮濓絽鍙ц箛鍐畱閻劍鍩涢悽璇插剼閼宠棄濮忛妴?
// 鏉╂瑦鐗遍崥搴ㄦ桨閺冪姾顔戦弰顖氼槻閻?userService閿涘矁绻曢弰顖涙禌閹广垺鍨氶崚顐ゆ畱鐎圭偟骞囬敍宀勫厴娑撳秶鏁ら弨?ChatService閵?
type UserProfileProvider interface {
	GetUserProfile(ctx context.Context, userID uint) (*userDTO.UserProfileResponse, error)
}

// SubmissionDataProvider 閹绘劒绶电拋顓犵矊鐠佲€冲灊闂団偓鐟曚胶娈戦幓鎰唉閸樺棗褰堕弫鐗堝祦閵?
type SubmissionDataProvider interface {
	GetAllSubmissionsByUserID(ctx context.Context, userID uint) ([]submissionModel.Submission, error)
	GetRecentFailedSubmissionsByUserID(ctx context.Context, userID uint, limit int) ([]submissionModel.Submission, error)
}

// ProblemCatalogProvider 閹绘劒绶电拋顓犵矊鐠佲€冲灊闂団偓鐟曚胶娈戞０妯兼窗閻╊喖缍嶆穱鈩冧紖閵?
type ProblemCatalogProvider interface {
	GetAllProblemsWithTags(ctx context.Context) ([]problemModel.Problem, error)
	GetProblemByID(ctx context.Context, id string) (*problemModel.Problem, error)
}

type ChatService struct {
	repo        repository.ChatRepository
	userProfile UserProfileProvider
	submissions SubmissionDataProvider
	problems    ProblemCatalogProvider
}

func NewChatService(
	r repository.ChatRepository,
	up UserProfileProvider,
	sub SubmissionDataProvider,
	pro ProblemCatalogProvider,
) *ChatService {
	return &ChatService{
		repo:        r,
		userProfile: up,
		submissions: sub,
		problems:    pro,
	}
}

// GetUserACHistory 鏉╂柨娲栭悽銊﹀煕閻?AC 閸樺棗褰堕敍宀€鏁ゆ担婊咁儑娑撯偓娑?internal agent tool閵?
func (s *ChatService) GetUserACHistory(ctx context.Context, userID uint) (*dto.UserACHistoryResponse, error) {
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

// GetUserFailedSubmissions 鏉╂柨娲栭悽銊﹀煕閺堚偓鏉╂垵銇戠拹銉ф畱閹绘劒姘﹂敍宀€绮伴崥搴ｇ敾 agent 閸掑棙鐎介挅鍕€ラ悙閫涘▏閻劊鈧?
func (s *ChatService) GetUserFailedSubmissions(ctx context.Context, userID uint, limit int) (*dto.UserFailedSubmissionsResponse, error) {
	if s.submissions == nil || s.problems == nil {
		return nil, errors.New("chat providers not configured")
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

// GetUserTagStats 缂佺喕顓搁悽銊﹀煕閸︺劌鎮囬弽鍥╊劮娑撳娈戦幓鎰唉閸滃矁袙妫版ɑ鍎忛崘鐐光偓?
// 鏉╂瑤鍞ら弫鐗堝祦闁倸鎮庨崥搴ㄦ桨閻?Python agent 閻劍娼甸崚銈嗘焽瑜版挸澧犻挅鍕€ラ悙骞库偓?
func (s *ChatService) GetUserTagStats(ctx context.Context, userID uint) (*dto.UserTagStatsResponse, error) {
	if s.userProfile == nil || s.submissions == nil || s.problems == nil {
		return nil, errors.New("chat providers not configured")
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

// GetCandidateProblems 閺嶈宓侀弽鍥╊劮閸滃本甯撻梽銈呭灙鐞涖劏绻戦崶鐐测偓娆撯偓澶愵暯閿涘奔缍旀稉鐑樺腹閼芥劗閮寸紒鐔烘畱閸婃瑩鈧娉﹂悽鐔稿灇濮濄儵顎冮妴?
func (s *ChatService) GetCandidateProblems(ctx context.Context, requestedTags []string, excludeProblemIDs []uint, limit int) (*dto.CandidateProblemsResponse, error) {
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

// GetProblemDetail 鏉╂柨娲栭崡鏇㈩暯鐠囷附鍎忛敍灞芥倵闂?agent 闂団偓鐟曚浇藟閺囨潙顦挎０妯兼窗娑撳﹣绗呴弬鍥ㄦ閸欘垯浜掗惄瀛樺复婢跺秶鏁ら妴?
func (s *ChatService) GetProblemDetail(ctx context.Context, problemID uint) (*dto.ProblemDetailResponse, error) {
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

func (s *ChatService) getProblemMap(ctx context.Context) (map[uint]problemModel.Problem, error) {
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

func (s *ChatService) chatRepo() (repository.ChatRepository, error) {
	return s.repo, nil
}

func (s *ChatService) CreateChatSession(ctx context.Context, userID uint, title string) (*model.ChatSession, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}

	session := &model.ChatSession{
		UserID: userID,
		Title:  strings.TrimSpace(title),
		Status: model.ChatSessionStatusActive,
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ChatService) ListChatSessions(ctx context.Context, userID uint, limit int) ([]model.ChatSession, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return repo.ListSessionsByUserID(ctx, userID, limit)
}

func (s *ChatService) GetChatSession(ctx context.Context, userID uint, sessionID uint) (*model.ChatSession, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}
	session, err := repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, apperror.ErrForbidden
	}
	if session.Status != model.ChatSessionStatusActive {
		return nil, gorm.ErrRecordNotFound
	}
	return session, nil
}

func (s *ChatService) ArchiveChatSession(ctx context.Context, userID uint, sessionID uint) error {
	repo, err := s.chatRepo()
	if err != nil {
		return err
	}

	session, err := repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.UserID != userID {
		return apperror.ErrForbidden
	}
	if session.Status != model.ChatSessionStatusActive {
		return gorm.ErrRecordNotFound
	}

	hasActiveTurn, err := repo.HasActiveTurn(ctx, sessionID)
	if err != nil {
		return err
	}
	if hasActiveTurn {
		return apperror.ErrChatSessionBusy
	}

	return repo.ArchiveSession(ctx, sessionID)
}

func (s *ChatService) GetChatMessages(ctx context.Context, userID uint, sessionID uint, limit int) ([]model.ChatMessage, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}
	if _, err := s.GetChatSession(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	if limit < 0 {
		limit = 0
	}
	return repo.ListMessagesBySessionID(ctx, sessionID, limit)
}

func (s *ChatService) SendChatMessage(ctx context.Context, userID uint, sessionID uint, content string) (*model.ChatTurn, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}

	session, err := s.GetChatSession(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("message content is empty")
	}

	modelName := config.GlobalConfig.AI.Model
	if strings.TrimSpace(modelName) == "" {
		modelName = "chat-agent"
	}

	_, turn, err := repo.CreateUserMessageTurn(ctx, session, content, modelName)
	if err != nil {
		return nil, err
	}

	queueTask := dto.ChatTurnQueueTask{TurnID: turn.ID}
	taskBytes, err := json.Marshal(queueTask)
	if err != nil {
		return nil, err
	}
	if err := repo.PushTurnToQueue(ctx, taskBytes); err != nil {
		return nil, err
	}

	return turn, nil
}

func (s *ChatService) GetChatTurn(ctx context.Context, userID uint, turnID uint) (*model.ChatTurn, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}
	turn, err := repo.GetTurnByID(ctx, turnID)
	if err != nil {
		return nil, err
	}
	session, err := repo.GetSessionByID(ctx, turn.SessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, apperror.ErrForbidden
	}
	if session.Status != model.ChatSessionStatusActive {
		return nil, gorm.ErrRecordNotFound
	}
	return turn, nil
}

func FormatChatAssistantContentForWorker(rawResult string) string {
	type recommendedProblem struct {
		ProblemID uint   `json:"problem_id"`
		Title     string `json:"title"`
		Reason    string `json:"reason"`
	}
	type chatResult struct {
		Answer              string               `json:"answer"`
		WeakTags            []string             `json:"weak_tags"`
		RecommendedProblems []recommendedProblem `json:"recommended_problems"`
	}

	var parsed chatResult
	if err := json.Unmarshal([]byte(rawResult), &parsed); err != nil {
		return strings.TrimSpace(rawResult)
	}

	lines := make([]string, 0, 8)
	if strings.TrimSpace(parsed.Answer) != "" {
		lines = append(lines, parsed.Answer)
	}
	if len(parsed.WeakTags) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Weak tags: "+strings.Join(parsed.WeakTags, ", "))
	}
	if len(parsed.RecommendedProblems) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Recommended problems:")
		for _, item := range parsed.RecommendedProblems {
			lines = append(lines, "- #"+uintToString(item.ProblemID)+" "+item.Title+": "+item.Reason)
		}
	}
	if len(lines) == 0 {
		return strings.TrimSpace(rawResult)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
func BuildChatGoalForWorker(messages []model.ChatMessage) string {
	if len(messages) == 0 {
		return "general OJ learning guidance"
	}

	lines := []string{
		"Continue this OJ learning chat.",
		"Answer the latest user message directly and recommend problems only when useful.",
		"Conversation history:",
	}
	for _, message := range messages {
		lines = append(lines, chatRoleLabel(message.Role)+": "+strings.TrimSpace(message.Content))
	}
	return strings.Join(lines, "\n")
}

func chatRoleLabel(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case model.ChatMessageRoleAssistant:
		return "Assistant"
	case model.ChatMessageRoleSystem:
		return "System"
	default:
		return "User"
	}
}
func (s *ChatService) SubmitChatPlanFeedback(ctx context.Context, userID uint, turnID uint, helpful bool, comment string) (*model.ChatPlanFeedback, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}

	turn, err := repo.GetTurnByID(ctx, turnID)
	if err != nil {
		return nil, err
	}
	if turn.UserID != userID {
		return nil, apperror.ErrForbidden
	}
	if turn.Status != model.TaskStatusSucceeded {
		return nil, apperror.ErrChatSessionBusy
	}

	feedback := &model.ChatPlanFeedback{
		ChatTurnID: turnID,
		UserID:     userID,
		Helpful:    helpful,
		Comment:    strings.TrimSpace(comment),
	}
	if err := repo.UpsertPlanFeedback(ctx, feedback); err != nil {
		return nil, err
	}
	return feedback, nil
}

func (s *ChatService) GetChatPlanFeedback(ctx context.Context, userID uint, turnID uint) (*model.ChatPlanFeedback, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}

	turn, err := repo.GetTurnByID(ctx, turnID)
	if err != nil {
		return nil, err
	}
	if turn.UserID != userID {
		return nil, apperror.ErrForbidden
	}
	return repo.GetPlanFeedbackByTurnIDAndUserID(ctx, turnID, userID)
}
