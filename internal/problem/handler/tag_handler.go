package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/problem/dto"
	"gojo/internal/problem/service"
)

type TagHandler struct {
	svc *service.TagService
}

func NewTagHandler(svc *service.TagService) *TagHandler {
	return &TagHandler{svc: svc}
}

func (h *TagHandler) GetTagList(c *gin.Context) {
	tags, err := h.svc.GetTagList(c.Request.Context())
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get tags failed")
		return
	}

	response.OK(c, tags)
}

func (h *TagHandler) CreateTag(c *gin.Context) {
	var req dto.CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "tag name is required")
		return
	}

	tag, err := h.svc.CreateTag(c.Request.Context(), req.Name)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "create tag failed")
		return
	}

	response.OK(c, tag)
}

func (h *TagHandler) DeleteTag(c *gin.Context) {
	tagID := c.Param("id")

	if err := h.svc.DeleteTag(c.Request.Context(), tagID); err != nil {
		if errors.Is(err, apperror.ErrTagNotFound) {
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "tag not found")
			return
		}
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "delete tag failed")
		return
	}

	response.OK(c, nil)
}
