package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/problem/dto"
	"gojo/internal/problem/service"
)

type ProblemHandler struct {
	svc *service.ProblemService
}

func NewProblemHandler(s *service.ProblemService) *ProblemHandler {
	return &ProblemHandler{svc: s}
}

func (h *ProblemHandler) CreateProblem(c *gin.Context) {
	var req dto.ProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	problem, err := h.svc.CreateProblem(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, apperror.ErrTagNotFound) {
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "tag not found")
			return
		}
		response.Fail(c, http.StatusInternalServerError, ecode.InternalError)
		return
	}

	response.OK(c, gin.H{
		"problem_id": problem.ID,
	})
}

func (h *ProblemHandler) GetProblemList(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	tagIDStr := c.Query("tag_id")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	var uid uint
	if id, exists := c.Get("userID"); exists {
		uid = id.(uint)
	}

	res, err := h.svc.GetProblemList(c.Request.Context(), page, limit, tagIDStr, uid)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get problem list failed")
		return
	}

	response.OK(c, res)
}

func (h *ProblemHandler) GetProblemDetail(c *gin.Context) {
	id := c.Param("id")

	problem, err := h.svc.GetProblemDetail(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrProblemNotFound) {
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "problem not found")
			return
		}
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get problem detail failed")
		return
	}

	response.OK(c, problem)
}

func (h *ProblemHandler) UpdateProblem(c *gin.Context) {
	problemID := c.Param("id")

	var req dto.ProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	if err := h.svc.UpdateProblem(c.Request.Context(), problemID, req); err != nil {
		if errors.Is(err, apperror.ErrProblemNotFound) {
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "problem not found")
			return
		}
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "update problem failed")
		return
	}

	response.OK(c, nil)
}

func (h *ProblemHandler) DeleteProblem(c *gin.Context) {
	problemID := c.Param("id")

	if err := h.svc.DeleteProblem(c.Request.Context(), problemID); err != nil {
		if errors.Is(err, apperror.ErrProblemNotFound) {
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "problem not found")
			return
		}
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "delete problem failed")
		return
	}

	response.OK(c, nil)
}

func (h *ProblemHandler) UpdateProblemTags(c *gin.Context) {
	problemID := c.Param("id")

	var req dto.UpdateProblemTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	if err := h.svc.UpdateProblemTags(c.Request.Context(), problemID, req.TagIDs); err != nil {
		switch {
		case errors.Is(err, apperror.ErrProblemNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "problem not found")
		case errors.Is(err, apperror.ErrTagNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "tag not found")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "update problem tags failed")
		}
		return
	}

	response.OK(c, nil)
}
