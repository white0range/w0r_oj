package handler

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gojo/config"
	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/judge/trial"
)

func (h *ProblemHandler) RunCode(c *gin.Context) {
	if !config.GlobalConfig.Judge.TrialEnabled {
		response.FailWithMessage(c, http.StatusServiceUnavailable, ecode.InternalError, "代码运行暂未启用")
		return
	}
	problemID, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil || problemID == 0 {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}
	var request struct {
		Code  string  `json:"code"`
		Input *string `json:"input"`
	}
	// JSON can use up to six bytes to encode one byte of the submitted text.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 6*(trial.MaxCodeBytes+trial.MaxInputBytes)+1024)
	if err := c.ShouldBindJSON(&request); err != nil || request.Input == nil || strings.TrimSpace(request.Code) == "" {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "请填写 Go 代码和标准输入，标准输入可以为空")
		return
	}
	if len(request.Code) > trial.MaxCodeBytes || len(*request.Input) > trial.MaxInputBytes {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "代码最多 64 KiB，输入最多 4 KiB")
		return
	}
	deadline := time.Now().Add(trial.Timeout)
	ctx, cancel := context.WithDeadline(c.Request.Context(), deadline)
	defer cancel()
	if _, err := h.svc.GetProblemDetail(ctx, strconv.FormatUint(problemID, 10)); err != nil {
		if errors.Is(err, apperror.ErrProblemNotFound) {
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "题目不存在")
			return
		}
		response.FailWithMessage(c, http.StatusServiceUnavailable, ecode.InternalError, "暂时无法读取题目，请稍后再试")
		return
	}

	task := trial.Task{
		Input: *request.Input, Deadline: deadline,
		CodeRun: &trial.CodeRun{ID: rand.Text(), UserID: c.GetUint("userID"), ProblemID: uint(problemID), Code: request.Code},
	}
	if err := trial.Enqueue(ctx, task); err != nil {
		switch {
		case errors.Is(err, trial.ErrBusy):
			response.FailWithMessage(c, http.StatusTooManyRequests, ecode.TooManyRequests, "运行队列已满，请稍后再试")
		case errors.Is(err, trial.ErrUsed):
			response.FailWithMessage(c, http.StatusTooManyRequests, ecode.TooManyRequests, "上一次运行仍在处理中")
		default:
			response.FailWithMessage(c, http.StatusServiceUnavailable, ecode.InternalError, "运行服务暂不可用")
		}
		return
	}
	result, err := trial.Wait(ctx, task)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		response.FailWithMessage(c, http.StatusGatewayTimeout, ecode.InternalError, "等待运行结果超时，请稍后再试")
	case err != nil || result.Status == "system_error":
		response.FailWithMessage(c, http.StatusServiceUnavailable, ecode.InternalError, "运行服务暂不可用")
	default:
		response.OK(c, result)
	}
}
