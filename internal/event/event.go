// Package event 实现简单的进程内事件发布/订阅机制。
// 用于解耦核心提交逻辑与插件（如排行榜、仪表盘、三血等）之间的依赖。
// 当用户正确提交 Flag 时，service 层发布事件，各插件异步消费处理。
package event

import (
	"context"
	"log"
	"sync"
	"time"
)

// EventType 是事件类型的字符串枚举。
type EventType string

const (
	// EventCorrectSubmission 表示正确提交 Flag 的事件
	EventCorrectSubmission EventType = "correct_submission"
)

// Event 是事件的载体结构体。
// 当用户正确提交 Flag 时发布此事件，携带用户、题目、比赛等上下文信息。
type Event struct {
	Type          EventType       // 事件类型
	UserID        string          // 提交用户的 ID
	TeamID        string          // 队伍 ID（队伍模式时填写）
	ChallengeID   string          // 题目的 res_id
	CompetitionID string          // 所属比赛的 res_id
	SubmittedAt   time.Time       // 实际提交时间
	Ctx           context.Context // 请求上下文
}

var (
	// subscribers 按事件类型存储订阅者回调列表
	subscribers = make(map[EventType][]func(Event))
	// mu 保护 subscribers 的并发读写
	mu          sync.RWMutex
)

// Subscribe 订阅指定类型的事件。
// 当该类型事件发布时，注册的回调函数会被异步调用。
// 参数 t: 要订阅的事件类型；参数 fn: 事件处理回调函数。
func Subscribe(t EventType, fn func(Event)) {
	mu.Lock()
	defer mu.Unlock()
	subscribers[t] = append(subscribers[t], fn)
}

// Publish 发布事件，异步通知所有订阅者。
// 每个订阅者的回调函数在独立的 goroutine 中执行，不阻塞发布者。
// 参数 e: 要发布的事件对象。
func Publish(e Event) {
	mu.RLock()
	defer mu.RUnlock()
	for _, fn := range subscribers[e.Type] {
		go func(fn func(Event), e Event) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[event] panic in subscriber for %s: %v", e.Type, r)
				}
			}()
			fn(e)
		}(fn, e)
	}
}
