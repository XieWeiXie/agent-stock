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
	Symbol    string
	Name      string
	Price     float64
	ChangePct float64
	Amount    float64
	Turnover  float64
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

type Provider interface {
	Search(ctx context.Context, keyword string, market Market) ([]SearchResult, error)
	Quote(ctx context.Context, symbols []string) ([]Quote, error)
	Rank(ctx context.Context, market Market, sort string, count int) ([]RankItem, error)
	Index(ctx context.Context, market Market) ([]Quote, error)
	KlineDaily(ctx context.Context, symbol string, limit int) (Kline, error)
}
