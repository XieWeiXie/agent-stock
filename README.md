# agent-stock

面向 AI Agent 的股市数据命令行工具：提供市场概览、个股行情、排行榜与日 K/技术指标等信息。

当前实现重点放在 A 股（`--market ab`），其它市场与部分子命令以占位形式提供，后续可继续补齐。

## 安装

```bash
go build -o stock ./cmd/stock
./stock --help
```

或安装到 GOPATH/bin：

```bash
go install ./cmd/stock
stock --help
```

## 快速开始

```bash
# 市场数据
stock index --market ab
stock rank --count 20
stock search 腾讯

# 个股数据
stock quote 000001
stock quote 000001,600519
stock kline 000001
```

## 命令

已实现（可用）：

- `stock index --market ab`
- `stock search --market ab <keyword>`
- `stock quote <symbols>`
- `stock rank --market ab --sort <sort> --count <count>`
- `stock kline <symbol>`
- `stock detail <symbol>`
- `stock news <symbol>`
- `stock fundflow <symbol>`
- `stock plate <symbol>`
- `stock chgdiagram`

占位（可运行但未实现业务逻辑）：

- `heatmap` / `query`

## 数据来源

Go 版当前使用 Eastmoney 的公开接口进行数据拉取与解析（未内置任何 Key）。

## 开发

```bash
gofmt -w .
go test ./...
go vet ./...
```

