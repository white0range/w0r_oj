package ecode

type ECode struct {
	Code    int
	Message string
}

var (
	Success       = ECode{Code: 0, Message: "ok"}
	InvalidParams = ECode{Code: 40001, Message: "invalid params"}
	Unauthorized  = ECode{Code: 40101, Message: "unauthorized"}
	Forbidden     = ECode{Code: 40301, Message: "forbidden"}
	NotFound      = ECode{Code: 40401, Message: "resource not found"}
	Conflict      = ECode{Code: 40901, Message: "resource conflict"}
	InternalError = ECode{Code: 50001, Message: "internal server error"}
)
