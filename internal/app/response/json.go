package response

import (
	"net/http"

	"gojo/internal/app/ecode"

	"github.com/gin-gonic/gin"
)

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Body{
		Code:    ecode.Success.Code,
		Message: ecode.Success.Message,
		Data:    data,
	})
}

func Fail(c *gin.Context, httpStatus int, ec ecode.ECode) {
	c.JSON(httpStatus, Body{
		Code:    ec.Code,
		Message: ec.Message,
		Data:    nil,
	})
}

func FailWithMessage(c *gin.Context, httpStatus int, ec ecode.ECode, msg string) {
	c.JSON(httpStatus, Body{
		Code:    ec.Code,
		Message: msg,
		Data:    nil,
	})
}
