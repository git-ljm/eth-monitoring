package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type NodeKind string

const (
	KindEVM    NodeKind = "evm"    // 执行层 JSON-RPC，使用 eth_blockNumber
	KindBeacon NodeKind = "beacon" // 共识层 Beacon API，使用 head slot
)

type Node struct {
	Name string
	Kind NodeKind
	URL  string
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// fetchHeight 返回节点当前高度：EVM 节点为区块号，Beacon 节点为 head slot。
func fetchHeight(ctx context.Context, n Node) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	switch n.Kind {
	case KindBeacon:
		return beaconHeadSlot(ctx, n.URL)
	case KindEVM:
		return evmBlockNumber(ctx, n.URL)
	default:
		return 0, fmt.Errorf("unknown node kind %q", n.Kind)
	}
}

func evmBlockNumber(ctx context.Context, url string) (uint64, error) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := doJSON(req, &out); err != nil {
		return 0, err
	}
	if out.Error != nil {
		return 0, fmt.Errorf("rpc error %d: %s", out.Error.Code, out.Error.Message)
	}

	hex, ok := strings.CutPrefix(out.Result, "0x")
	if !ok || hex == "" {
		return 0, fmt.Errorf("invalid eth_blockNumber result %q", out.Result)
	}
	n, err := strconv.ParseUint(hex, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid eth_blockNumber result %q: %w", out.Result, err)
	}
	return n, nil
}

func beaconHeadSlot(ctx context.Context, baseURL string) (uint64, error) {
	url := strings.TrimRight(baseURL, "/") + "/eth/v1/beacon/headers/head"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")

	var out struct {
		Data struct {
			Header struct {
				Message struct {
					Slot string `json:"slot"`
				} `json:"message"`
			} `json:"header"`
		} `json:"data"`
	}
	if err := doJSON(req, &out); err != nil {
		return 0, err
	}

	slot := out.Data.Header.Message.Slot
	n, err := strconv.ParseUint(slot, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid head slot %q: %w", slot, err)
	}
	return n, nil
}

func doJSON(req *http.Request, out any) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
