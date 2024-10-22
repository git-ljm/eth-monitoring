package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const alertmanagerURL = "http://alertmanager-main.monitoring.svc:9093/api/v2/alerts"
const alertThreshold = 15

type Alert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	GroupLabels map[string]string `json:"groupLabels"`
	Status      string            `json:"status"`
	StartsAt    *time.Time        `json:"startsAt,omitempty"`
	EndsAt      *time.Time        `json:"endsAt,omitempty"`
}

func sendAlert(node string, message string) error {
	// startTime := time.Now()
	alert := []Alert{
		{
			Labels: map[string]string{
				"alertname": "EthereumBlockNotUpdated",
				"severity":  "critical",
				"node":      node,
				"instance":  node,
			},
			Annotations: map[string]string{
				"summary":     message,
				"description": message,
			},
			GroupLabels: map[string]string{
				"alert_group": "ethereum_block_monitor",
			},
			// StartsAt: &startTime,
			Status: "firing",
		},
	}

	data, err := json.Marshal(alert)
	if err != nil {
		return err
	}

	resp, err := http.Post(alertmanagerURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send alert, status: %d", resp.StatusCode)
	}

	return nil
}
