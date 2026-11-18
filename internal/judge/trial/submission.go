package trial

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"gojo/internal/app/apperror"
	"gojo/internal/submission/model"
	"gojo/internal/submission/repository"
)

var ErrIneligible = errors.New("trial requires a failed Go submission")

func LoadSubmission(ctx context.Context, userID, submissionID uint) (*model.Submission, error) {
	submission, err := repository.NewSubmissionRepository().GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return nil, err
	}
	if submission.UserID != userID {
		return nil, apperror.ErrForbidden
	}
	if submission.Language == "go" {
		switch submission.Status {
		case "WA", "RE", "TLE", "MLE", "OLE", "CE":
			return submission, nil
		}
	}
	return nil, ErrIneligible
}

func HashAttempt(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
