package apperr

import "errors"

var (
	ErrAlreadySubscribed = errors.New("subscription already exists")
	ErrInvalidEmail      = errors.New("invalid email")
	ErrInvalidRepoFormat = errors.New("invalid repo format")
	ErrInvalidToken      = errors.New("invalid token")
	ErrRepoNotFound      = errors.New("repository not found")
	ErrTokenNotFound     = errors.New("token not found")
	ErrRateLimited       = errors.New("rate limited by github")
)
