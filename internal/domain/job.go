package domain

import (
	"github.com/google/uuid"
)

type JobPriority int

const (
	PriorityHigh   JobPriority = 0
	PriorityMedium JobPriority = 1
	PriorityLow    JobPriority = 2
)

type Job struct {
	ID         uuid.UUID      `json:"id"`
	PhotoID    int64          `json:"photo_id"`
	Width      int            `json:"width"`
	Format     string         `json:"format"`
	Quality    int            `json:"quality"`
	Priority   JobPriority    `json:"priority"`
	CreatedAt  int64          `json:"created_at"`
	ResultChan chan JobResult `json:"-"` // канал для возврата результата
}

type JobResult struct {
	Job  Job
	Data []byte
	Err  error
}
