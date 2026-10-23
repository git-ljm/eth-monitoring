package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	alertNameStalled  = "EthereumBlockNotUpdated"
	alertNameRPCError = "EthereumNodeRPCError"

	// 告警发送时 endsAt 设为 now+alertTTL，持续告警期间每次轮询都会续期；
	// 本程序挂掉时告警会在 TTL 后自动恢复，不会永久挂着。
	alertTTL = 5 * time.Minute
)

// postableAlert 对应 Alertmanager v2 API 的 PostableAlert。
type postableAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations,omitempty"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
}

type Alerter struct {
	url    string
	client *http.Client
}

func NewAlerter(url string) *Alerter {
	return &Alerter{url: url, client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *Alerter) Send(ctx context.Context, alerts ...postableAlert) error {
	data, err := json.Marshal(alerts)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("alertmanager status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func alertLabels(n Node, alertName string) map[string]string {
	return map[string]string{
		"alertname":   alertName,
		"severity":    "critical",
		"chain":       n.Name,
		"node":        n.URL,
		"instance":    n.URL,
		"alert_group": "ethereum_block_monitor",
	}
}
