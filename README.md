# eth-monitoring

监控 `chainrpc` 命名空间下各链节点的区块高度，节点高度停止增长或 RPC 不可用时向 Alertmanager 发送告警，节点恢复后自动发送恢复通知。

## 工作原理

每个节点一个 goroutine，每 30 秒查询一次高度：

- EVM 节点（`KindEVM`）：JSON-RPC `eth_blockNumber`
- Beacon 节点（`KindBeacon`）：`GET /eth/v1/beacon/headers/head`，取 head slot

根据查询结果触发两类告警：

| alertname | 触发条件 | 恢复条件 |
|---|---|---|
| `EthereumBlockNotUpdated` | 高度连续 15 分钟没有增长 | 高度再次增长 |
| `EthereumNodeRPCError` | 连续 3 次（约 1.5 分钟）查询失败 | 查询成功一次 |

告警持续期间每次轮询都会重发给 Alertmanager（`startsAt` 不变，`endsAt` 为当前时间 + 5 分钟），因此：

- 重复通知的频率由 Alertmanager 路由的 `group_interval` / `repeat_interval` 决定，不由本程序决定。
- 如果本程序自身挂掉，告警会在最多 5 分钟后在 Alertmanager 中自动过期。

节点高度回退（回滚或重新同步）时，以新高度为基线重新计时。

## 告警标签

| 标签 | 示例 | 说明 |
|---|---|---|
| `alertname` | `EthereumBlockNotUpdated` | 见上表 |
| `severity` | `critical` | 固定值 |
| `chain` | `rsk` | 节点名称，即 `main.go` 中的 `Name` |
| `node` / `instance` | `http://rsk.chainrpc.svc:4444` | 节点地址 |
| `alert_group` | `ethereum_block_monitor` | 固定值 |

告警内容在 `summary` 和 `description` 两个 annotation 中。

## 监控的节点

节点列表在 `main.go` 的 `nodes` 变量中：

| 名称 | 类型 | 地址 |
|---|---|---|
| duck | EVM | `http://duck.chainrpc.svc:8545` |
| eth2beacon | Beacon | `http://eth2beacon.chainrpc.svc:5052` |
| gethnode | EVM | `http://gethnode.chainrpc.svc:8545` |
| goat-geth | EVM | `http://goat-geth.chainrpc.svc:8545` |
| iotex | EVM | `http://iotex.chainrpc.svc:15014` |
| manta-reth | EVM | `http://manta-reth.chainrpc.svc:8545` |
| merlin | EVM | `http://merlin-rpc.chainrpc.svc:8545` |
| rsk | EVM | `http://rsk.chainrpc.svc:4444` |
| tac | EVM | `http://tac.chainrpc.svc:8545` |
| taiko-geth | EVM | `http://taiko-geth.chainrpc.svc:8545` |

`taiko-node` 是 taiko 的 driver，没有 eth RPC，不单独监控。它异常时 taiko-geth 的高度会停止增长，由 taiko-geth 的告警覆盖。

### 添加或删除节点

编辑 `main.go` 中的 `nodes`，重新构建镜像并部署：

```go
{Name: "example", Kind: KindEVM, URL: "http://example.chainrpc.svc:8545"},
```

注意：geth 的 `--http.vhosts`、RSK 的 `rpc.providers.web.http.hosts` 默认只允许 localhost。节点需要放行 service 域名，否则请求会返回 403/400 并触发 `EthereumNodeRPCError`。

## 配置

| 配置 | 位置 | 默认值 |
|---|---|---|
| Alertmanager 地址 | 环境变量 `ALERTMANAGER_URL` | `http://alertmanager-main.monitoring.svc:9093/api/v2/alerts` |
| 轮询间隔 | `main.go` `pollInterval` | 30s |
| 单次查询超时 | `main.go` `requestTimeout` | 5s |
| 高度停滞告警阈值 | `main.go` `stallThreshold` | 15m |
| RPC 失败告警阈值 | `main.go` `rpcFailureThreshold` | 连续 3 次 |
| 告警过期时间 | `alertmanager.go` `alertTTL` | 5m |

## 构建与运行

需要 Go 1.22 及以上，无第三方依赖。

```bash
# 本地构建
go build -o eth-monitoring .

# 本地运行（需要能访问节点和 Alertmanager）
ALERTMANAGER_URL=http://localhost:9093/api/v2/alerts ./eth-monitoring

# 构建镜像
docker build -t eth-monitoring .
```

镜像以 nobody 用户（65534）运行，入口为 `/app/eth-monitoring`。

## 代码结构

| 文件 | 内容 |
|---|---|
| `main.go` | 节点列表、配置常量、启动与优雅退出 |
| `monitor.go` | 单节点轮询、告警状态（触发 / 续期 / 恢复） |
| `rpc.go` | EVM 与 Beacon 节点的高度查询 |
| `alertmanager.go` | Alertmanager v2 API 客户端、告警标签 |
