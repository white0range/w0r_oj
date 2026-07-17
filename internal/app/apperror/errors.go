package apperror

import "errors"

var (
	ErrUsernameExists     = errors.New("username exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrWrongPassword      = errors.New("wrong password")
	ErrPasswordHashFailed = errors.New("password hash failed")
	ErrTokenGeneration    = errors.New("token generation failed")
	ErrInvalidToken       = errors.New("invalid token")
	ErrUserBanned         = errors.New("user banned")

	ErrUnauthorizedAccess = errors.New("unauthorized access")
	ErrSubmissionNotFound = errors.New("submission not found")
	ErrForbidden          = errors.New("forbidden")
	ErrAlreadyAccepted    = errors.New("already accepted")
	ErrAIConnectFailed    = errors.New("ai connect failed")
	ErrChatSessionBusy    = errors.New("chat session has active turn")

	ErrInvalidID       = errors.New("invalid id")
	ErrProblemNotFound = errors.New("problem not found")
	ErrCaseNotFound    = errors.New("case not found")
	ErrTagNotFound     = errors.New("tag not found")
)
