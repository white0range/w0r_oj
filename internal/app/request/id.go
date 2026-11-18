package request

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
)

// PathID reads a positive integer identifier or writes a bad-request response.
func PathID(c *gin.Context, name, message string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, strconv.IntSize)
	if err != nil || id == 0 {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, message)
		return 0, false
	}
	return uint(id), true
}
