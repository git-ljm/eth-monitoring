package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

func getLatestBlockNumber(client *rpc.Client, timeout time.Duration) (*big.Int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var result string
	err := client.CallContext(ctx, &result, "eth_blockNumber")
	if err != nil {
		return nil, err
	}

	blockNumber := new(big.Int)
	blockNumber.SetString(result[2:], 16) // 解析十六进制字符串
	return blockNumber, nil
}

func monitorNode(node string, wg *sync.WaitGroup, stopChan chan struct{}) {
	defer wg.Done()

	var client *rpc.Client
	var err error
	retryDelay := 10 * time.Second

	for {
		select {
		case <-stopChan:
			log.Printf("Stopping monitoring of node: %s", node)
			if client != nil {
				client.Close()
			}
			return
		default:
			if client == nil {
				log.Printf("Attempting to connect to node: %s", node)
				client, err = rpc.Dial(node)
				if err != nil {
					log.Printf("Failed to connect to node %s: %v. Retrying in %v...", node, err, retryDelay)
					time.Sleep(retryDelay)
					continue
				}
				log.Printf("Connected to Ethereum node: %s", node)
			}

			err = monitor(client, node, stopChan)
			if err != nil {
				log.Printf("Error monitoring node %s: %v. Closing connection and retrying...", node, err)
				client.Close()
				client = nil
				time.Sleep(retryDelay)
			}
		}
	}
}

func monitor(client *rpc.Client, node string, stopChan chan struct{}) error {
	var lastBlockNumber *big.Int
	var lastUpdateTime time.Time

	for {
		select {
		case <-stopChan:
			log.Printf("Stopping monitoring of node: %s", node)
			return nil
		default:
			currentBlockNumber, err := getLatestBlockNumber(client, 3*time.Second)
			if err != nil {
				log.Printf("Error getting latest block from node %s: %v", node, err)
				return err
			}

			if lastBlockNumber == nil || currentBlockNumber.Cmp(lastBlockNumber) > 0 {
				lastBlockNumber = currentBlockNumber
				lastUpdateTime = time.Now()
				log.Printf("Block updated on node %s: %s", node, lastBlockNumber.String())
			}

			// 检查是否超过 15 分钟未更新
			if time.Since(lastUpdateTime).Minutes() > alertThreshold {
				message := fmt.Sprintf("Block height on node %s hasn't been updated for more than %d minutes. Last block: %s", node, alertThreshold, lastBlockNumber.String())
				log.Println(message)

				err := sendAlert(node, message)
				if err != nil {
					log.Printf("Failed to send alert for node %s: %v", node, err)
				}

				lastUpdateTime = time.Now()
			}

			select {
			case <-stopChan:
				return nil
			case <-time.After(3 * time.Second):
			}
		}
	}
}
