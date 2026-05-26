package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/problem/dto"
	"gojo/internal/problem/service"
)

type SearchHandler struct {
	svc *service.ProblemService
}

func NewSearchHandler(svc *service.ProblemService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) SearchProblems(c *gin.Context) {
	var req dto.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	result, err := h.svc.SearchProblems(c.Request.Context(), req)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "search failed")
		return
	}

	response.OK(c, result)
}
