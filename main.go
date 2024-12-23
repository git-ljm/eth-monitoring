package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	nodes := []string{
		"http://coredao.chainrpc.svc:8579",
		"http://gethnode.chainrpc.svc:8545",
		"http://linea.chainrpc.svc:8545",
		"http://manta-geth.chainrpc.svc:8545",
		"http://mantle-geth.chainrpc.svc:8545",
		"http://scroll-geth.chainrpc.svc:8545",
		"http://zklink-nova-node.chainrpc.svc:3060",
		"http://merlin-rpc.chainrpc.svc:8545",
		// add more
	}

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	for _, node := range nodes {
		wg.Add(1)
		go monitorNode(node, &wg, stopChan)
	}

	go func() {
		sig := <-signalChan
		log.Printf("Received signal: %v, stopping monitoring...", sig)
		close(stopChan)
	}()

	wg.Wait()
	log.Println("All monitoring stopped.")
}
