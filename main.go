package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	defaultAlertmanagerURL = "http://alertmanager-main.monitoring.svc:9093/api/v2/alerts"

	pollInterval   = 30 * time.Second // 每个节点的轮询间隔
	requestTimeout = 5 * time.Second  // 单次高度查询超时
	stallThreshold = 15 * time.Minute // 高度超过该时长未增长则告警
	// 连续查询失败达到该次数才发 RPC 不可用告警，避免单次抖动误报（3 次约 1.5 分钟）
	rpcFailureThreshold = 3
)

// 需要告警的节点。Name 会作为告警的 chain 标签，Kind 决定高度的查询方式。
// taiko-node 是 taiko 的 driver，没有 eth RPC，它异常时 taiko-geth 高度会停止增长，由 taiko-geth 覆盖。
var nodes = []Node{
	{Name: "duck", Kind: KindEVM, URL: "http://duck.chainrpc.svc:8545"},
	{Name: "eth2beacon", Kind: KindBeacon, URL: "http://eth2beacon.chainrpc.svc:5052"},
	{Name: "gethnode", Kind: KindEVM, URL: "http://gethnode.chainrpc.svc:8545"},
	{Name: "goat-geth", Kind: KindEVM, URL: "http://goat-geth.chainrpc.svc:8545"},
	{Name: "iotex", Kind: KindEVM, URL: "http://iotex.chainrpc.svc:15014"},
	{Name: "manta-reth", Kind: KindEVM, URL: "http://manta-reth.chainrpc.svc:8545"},
	{Name: "merlin", Kind: KindEVM, URL: "http://merlin-rpc.chainrpc.svc:8545"},
	{Name: "rsk", Kind: KindEVM, URL: "http://rsk.chainrpc.svc:4444"},
	{Name: "tac", Kind: KindEVM, URL: "http://tac.chainrpc.svc:8545"},
	{Name: "taiko-geth", Kind: KindEVM, URL: "http://taiko-geth.chainrpc.svc:8545"},
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	amURL := os.Getenv("ALERTMANAGER_URL")
	if amURL == "" {
		amURL = defaultAlertmanagerURL
	}
	alerter := NewAlerter(amURL)
	log.Printf("monitoring %d nodes, alertmanager: %s", len(nodes), amURL)

	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()
			newMonitor(n, alerter).run(ctx)
		}(n)
	}

	<-ctx.Done()
	log.Println("received shutdown signal, stopping...")
	wg.Wait()
	log.Println("all monitors stopped")
}
