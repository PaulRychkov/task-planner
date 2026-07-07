package service

import (
	"errors"

	"github.com/PaulRychkov/task-planner/backend/internal/repository"
)

var ErrNotFound = repository.ErrNotFound

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func invalid(msg string) error { return &ValidationError{Message: msg} }

type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string { return e.Message }

func conflict(msg string) error { return &ConflictError{Message: msg} }

func IsValidation(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

func IsConflict(err error) bool {
	var ce *ConflictError
	return errors.As(err, &ce) || errors.Is(err, repository.ErrConflict)
}

func IsNotFound(err error) bool {
	return errors.Is(err, repository.ErrNotFound)
}
