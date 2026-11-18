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

	ErrForbidden       = errors.New("forbidden")
	ErrAlreadyAccepted = errors.New("already accepted")
	ErrChatSessionBusy = errors.New("chat session has active turn")

	ErrProblemNotFound      = errors.New("problem not found")
	ErrUnsupportedLanguage  = errors.New("unsupported language")
	ErrInvalidProblemLimits = errors.New("invalid problem resource limits")
	ErrCaseNotFound         = errors.New("case not found")
	ErrTagNotFound          = errors.New("tag not found")
)
