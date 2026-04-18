package event

import (
	"context"
	"sync"
)

type EventType string

const (
	EventCorrectSubmission EventType = "correct_submission"
)

type Event struct {
	Type          EventType
	UserID        string
	ChallengeID   string
	CompetitionID *string // nil 表示全局提交
	Ctx           context.Context
}

var (
	subscribers = make(map[EventType][]func(Event))
	mu          sync.RWMutex
)

// Subscribe 订阅事件
func Subscribe(t EventType, fn func(Event)) {
	mu.Lock()
	defer mu.Unlock()
	subscribers[t] = append(subscribers[t], fn)
}

// Publish 发布事件（异步通知订阅者）
func Publish(e Event) {
	mu.RLock()
	defer mu.RUnlock()
	for _, fn := range subscribers[e.Type] {
		go fn(e)
	}
}
