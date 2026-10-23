package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// alertState 记录一条告警是否处于 firing 状态及其开始时间。
type alertState struct {
	name  string
	since time.Time // 零值表示未触发
}

type monitor struct {
	node    Node
	alerter *Alerter

	interval         time.Duration
	stallThreshold   time.Duration
	failureThreshold int

	hasHeight    bool
	lastHeight   uint64
	lastProgress time.Time
	failures     int

	stalled  alertState
	rpcError alertState
}

func newMonitor(n Node, a *Alerter) *monitor {
	return &monitor{
		node:             n,
		alerter:          a,
		interval:         pollInterval,
		stallThreshold:   stallThreshold,
		failureThreshold: rpcFailureThreshold,
		stalled:          alertState{name: alertNameStalled},
		rpcError:         alertState{name: alertNameRPCError},
	}
}

func (m *monitor) run(ctx context.Context) {
	log.Printf("[%s] start monitoring %s (%s)", m.node.Name, m.node.URL, m.node.Kind)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		m.check(ctx)
		select {
		case <-ctx.Done():
			log.Printf("[%s] stopped", m.node.Name)
			return
		case <-ticker.C:
		}
	}
}

func (m *monitor) check(ctx context.Context) {
	height, err := fetchHeight(ctx, m.node)
	if ctx.Err() != nil {
		return // 正在退出
	}
	now := time.Now()

	if err != nil {
		m.failures++
		log.Printf("[%s] get height failed (%d consecutive): %v", m.node.Name, m.failures, err)
		if m.failures >= m.failureThreshold {
			msg := fmt.Sprintf("Node %s (%s) RPC failed %d times in a row: %v",
				m.node.Name, m.node.URL, m.failures, err)
			m.fire(ctx, &m.rpcError, now, msg)
		}
		return
	}

	if m.failures > 0 {
		log.Printf("[%s] RPC recovered after %d failures", m.node.Name, m.failures)
		m.failures = 0
	}
	m.resolve(ctx, &m.rpcError, now)

	switch {
	case !m.hasHeight || height > m.lastHeight:
		log.Printf("[%s] height %d", m.node.Name, height)
		m.hasHeight, m.lastHeight, m.lastProgress = true, height, now
	case height < m.lastHeight:
		// 节点回滚或重新同步，以新高度为基线重新计时
		log.Printf("[%s] height went backwards: %d -> %d", m.node.Name, m.lastHeight, height)
		m.lastHeight, m.lastProgress = height, now
	}

	if stuck := now.Sub(m.lastProgress); stuck >= m.stallThreshold {
		msg := fmt.Sprintf("Block height on node %s (%s) hasn't increased for %s. Last height: %d",
			m.node.Name, m.node.URL, stuck.Round(time.Second), m.lastHeight)
		m.fire(ctx, &m.stalled, now, msg)
	} else {
		m.resolve(ctx, &m.stalled, now)
	}
}

// fire 发送（或续期）告警。持续告警期间每次轮询都会重发，startsAt 保持不变，
// 通知频率由 Alertmanager 的 group_interval / repeat_interval 控制。
func (m *monitor) fire(ctx context.Context, s *alertState, now time.Time, msg string) {
	if s.since.IsZero() {
		s.since = now
		log.Printf("[%s] FIRING %s: %s", m.node.Name, s.name, msg)
	}
	err := m.alerter.Send(ctx, postableAlert{
		Labels:      alertLabels(m.node, s.name),
		Annotations: map[string]string{"summary": msg, "description": msg},
		StartsAt:    s.since,
		EndsAt:      now.Add(alertTTL),
	})
	if err != nil {
		log.Printf("[%s] send %s alert failed: %v", m.node.Name, s.name, err)
	}
}

// resolve 在告警处于 firing 状态时发送恢复通知（endsAt=now）。
func (m *monitor) resolve(ctx context.Context, s *alertState, now time.Time) {
	if s.since.IsZero() {
		return
	}
	log.Printf("[%s] RESOLVED %s (lasted %s)", m.node.Name, s.name, now.Sub(s.since).Round(time.Second))
	err := m.alerter.Send(ctx, postableAlert{
		Labels:   alertLabels(m.node, s.name),
		StartsAt: s.since,
		EndsAt:   now,
	})
	if err != nil {
		// 发送失败也清除状态，Alertmanager 中的告警会在 alertTTL 后自动过期
		log.Printf("[%s] send %s resolve failed: %v", m.node.Name, s.name, err)
	}
	s.since = time.Time{}
}
