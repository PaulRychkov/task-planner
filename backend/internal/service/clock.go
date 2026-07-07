package service

import (
	"time"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func NewClock() Clock { return realClock{} }

type FixedClock struct {
	T time.Time
}

func (c FixedClock) Now() time.Time { return c.T }

func Today(clock Clock, loc *time.Location) models.Date {
	return models.DateOf(clock.Now().In(loc))
}
