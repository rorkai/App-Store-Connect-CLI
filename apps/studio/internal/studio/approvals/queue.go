package approvals

import (
	"errors"
	"sync"
	"time"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

type Action struct {
	ID              string    `json:"id"`
	ThreadID        string    `json:"threadId"`
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	CommandPreview  []string  `json:"commandPreview"`
	MutationSurface string    `json:"mutationSurface"`
	Status          Status    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	ResolvedAt      time.Time `json:"resolvedAt,omitempty"`
}

type Queue struct {
	mu      sync.Mutex
	order   []string
	actions map[string]Action
}

func NewQueue() *Queue {
	return &Queue{
		actions: make(map[string]Action),
	}
}

func (q *Queue) Enqueue(action Action) Action {
	q.mu.Lock()
	defer q.mu.Unlock()
	if action.Status == "" {
		action.Status = StatusPending
	}
	q.actions[action.ID] = action
	q.order = append(q.order, action.ID)
	return action
}

func (q *Queue) Pending() []Action {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []Action
	for _, id := range q.order {
		action := q.actions[id]
		if action.Status == StatusPending {
			out = append(out, action)
		}
	}
	return out
}

func (q *Queue) Approve(id string) (Action, error) {
	return q.resolve(id, StatusApproved)
}

func (q *Queue) Reject(id string) (Action, error) {
	return q.resolve(id, StatusRejected)
}

func (q *Queue) resolve(id string, status Status) (Action, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	action, ok := q.actions[id]
	if !ok {
		return Action{}, errors.New("approval action not found")
	}
	action.Status = status
	action.ResolvedAt = time.Now().UTC()
	q.actions[id] = action
	return action, nil
}
