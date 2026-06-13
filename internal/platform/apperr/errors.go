package apperr

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

type AppError interface {
	error
	HTTPStatus() int
	GRPCStatus() codes.Code
}

type domainError struct {
	msg        string
	httpStatus int
	grpcStatus codes.Code
}

func (e *domainError) Error() string          { return e.msg }
func (e *domainError) HTTPStatus() int        { return e.httpStatus }
func (e *domainError) GRPCStatus() codes.Code { return e.grpcStatus }

var (
	ErrAlreadySubscribed = &domainError{
		msg:        "subscription already exists",
		httpStatus: http.StatusConflict,
		grpcStatus: codes.AlreadyExists,
	}
	ErrInvalidEmail = &domainError{
		msg:        "invalid email",
		httpStatus: http.StatusBadRequest,
		grpcStatus: codes.InvalidArgument,
	}
	ErrInvalidRepoFormat = &domainError{
		msg:        "invalid repo format",
		httpStatus: http.StatusBadRequest,
		grpcStatus: codes.InvalidArgument,
	}
	ErrInvalidToken = &domainError{
		msg:        "invalid token",
		httpStatus: http.StatusBadRequest,
		grpcStatus: codes.InvalidArgument,
	}
	ErrRepoNotFound = &domainError{
		msg:        "repository not found",
		httpStatus: http.StatusNotFound,
		grpcStatus: codes.NotFound,
	}
	ErrTokenNotFound = &domainError{
		msg:        "token not found",
		httpStatus: http.StatusNotFound,
		grpcStatus: codes.NotFound,
	}
	ErrRateLimited = &domainError{
		msg:        "rate limited by github",
		httpStatus: http.StatusServiceUnavailable,
		grpcStatus: codes.Unavailable,
	}
)
