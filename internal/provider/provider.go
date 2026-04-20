package provider

import "context"

type Market string

const (
	MarketAB Market = "ab"
	MarketHK Market = "hk"
	MarketUS Market = "us"
)

type SearchResult struct {
	Symbol string
	Name   string
	Market Market
}

type Quote struct {
	Symbol      string
	Name        string
	Price       float64
	Change      float64
	ChangePct   float64
	Open        float64
	High        float64
	Low         float64
	PrevClose   float64
	Volume      float64
	Amount      float64
	MarketValue float64
	Time        string
}

type RankItem struct {
	Symbol      string
	Name        string
	Price       float64
	ChangePct   float64
	Amount      float64
	Turnover    float64
	MarketValue float64 // 总市值，单位：元
}

type KlineBar struct {
	Date   string
	Open   float64
	Close  float64
	High   float64
	Low    float64
	Volume float64
	Amount float64
}

type Kline struct {
	Symbol string
	Name   string
	Bars   []KlineBar
}

type FundFlow struct {
	Symbol    string
	Name      string
	MainIn    float64 // 主力净流入
	MainPct   float64 // 主力净占比
	HugeIn    float64 // 超大单净流入
	LargeIn   float64 // 大单净流入
	MediumIn  float64 // 中单净流入
	SmallIn   float64 // 小单净流入
}

type NewsItem struct {
	Title string
	Url   string
	Time  string
}

type PlateItem struct {
	Symbol    string
	Name      string
	ChangePct float64
}

type MarketDistribution struct {
	Up        int // 涨
	Down      int // 跌
	Flat      int // 平
	UpLimit   int // 涨停
	DownLimit int // 跌停
	// 细分区间
	Up9       int // > 9%
	Up6_9     int // 6% - 9%
	Up3_6     int // 3% - 6%
	Up0_3     int // 0% - 3%
	Down0_3   int // -3% - 0%
	Down3_6   int // -6% - -3%
	Down6_9   int // -9% - -6%
	Down9     int // < -9%
}

type Provider interface {
	Search(ctx context.Context, keyword string, market Market) ([]SearchResult, error)
	Quote(ctx context.Context, symbols []string) ([]Quote, error)
	Rank(ctx context.Context, market Market, sort string, count int) ([]RankItem, error)
	Index(ctx context.Context, market Market) ([]Quote, error)
	KlineDaily(ctx context.Context, symbol string, limit int) (Kline, error)
	KlineWeekly(ctx context.Context, symbol string, limit int) (Kline, error)
	FundFlow(ctx context.Context, symbol string) (FundFlow, error)
	News(ctx context.Context, symbol string, count int) ([]NewsItem, error)
	Plate(ctx context.Context, symbol string) ([]PlateItem, error)
	MarketDistribution(ctx context.Context, market Market) (MarketDistribution, error)
}
