package protocol

import "fmt"

type ErrorCode string

const (
	ErrorInvalidArgument  ErrorCode = "invalid_argument"
	ErrorUnauthenticated  ErrorCode = "unauthenticated"
	ErrorPermissionDenied ErrorCode = "permission_denied"
	ErrorNotFound         ErrorCode = "not_found"
	ErrorConflict         ErrorCode = "conflict"
	ErrorUnsupported      ErrorCode = "unsupported"
	ErrorUnavailable      ErrorCode = "unavailable"
	ErrorInternal         ErrorCode = "internal"
)

type FieldViolation struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

type Problem struct {
	Code       ErrorCode        `json:"code"`
	Message    string           `json:"message"`
	Retryable  bool             `json:"retryable"`
	Violations []FieldViolation `json:"violations,omitempty"`
}

func (problem Problem) Validate() error {
	if !validErrorCode(problem.Code) {
		return fmt.Errorf("code: unsupported value %q", problem.Code)
	}
	if problem.Message == "" || len(problem.Message) > 500 {
		return fmt.Errorf("message: must contain 1 to 500 characters")
	}
	for index, violation := range problem.Violations {
		if violation.Field == "" || violation.Description == "" {
			return fmt.Errorf("violations[%d]: field and description are required", index)
		}
	}
	return nil
}

func validErrorCode(code ErrorCode) bool {
	switch code {
	case ErrorInvalidArgument, ErrorUnauthenticated, ErrorPermissionDenied, ErrorNotFound,
		ErrorConflict, ErrorUnsupported, ErrorUnavailable, ErrorInternal:
		return true
	default:
		return false
	}
}
