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

type TestCaseHandler struct {
	svc *service.TestCaseService
}

func NewTestCaseHandler(s *service.TestCaseService) *TestCaseHandler {
	return &TestCaseHandler{svc: s}
}

func (h *TestCaseHandler) AddTestCase(c *gin.Context) {
	problemIDStr := c.Param("id")

	var req dto.TestCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "input and expected_output are required")
		return
	}

	id, err := h.svc.AddTestCase(c.Request.Context(), problemIDStr, req)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrInvalidID):
			response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid problem id")
		case errors.Is(err, apperror.ErrProblemNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "problem not found")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "add test case failed")
		}
		return
	}

	response.OK(c, gin.H{
		"case_id": id,
	})
}

func (h *TestCaseHandler) DeleteTestCase(c *gin.Context) {
	caseID := c.Param("case_id")

	if err := h.svc.DeleteTestCase(c.Request.Context(), caseID); err != nil {
		if errors.Is(err, apperror.ErrCaseNotFound) {
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "test case not found")
			return
		}
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "delete test case failed")
		return
	}

	response.OK(c, nil)
}

func (h *TestCaseHandler) GetTestCases(c *gin.Context) {
	problemIDStr := c.Param("id")

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")
	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	res, err := h.svc.GetTestCases(c.Request.Context(), problemIDStr, page, limit)
	if err != nil {
		if errors.Is(err, apperror.ErrInvalidID) {
			response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid problem id")
			return
		}
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get test cases failed")
		return
	}

	response.OK(c, res)
}
