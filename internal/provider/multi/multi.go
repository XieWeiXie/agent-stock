package multi

import (
	"context"

	"agent-stock/internal/provider"
)

type Provider struct {
	ps []provider.Provider
}

func New(ps ...provider.Provider) *Provider {
	return &Provider{ps: ps}
}

func (m *Provider) Search(ctx context.Context, keyword string, market provider.Market) ([]provider.SearchResult, error) {
	for _, p := range m.ps {
		res, err := p.Search(ctx, keyword, market)
		if err == nil && len(res) > 0 {
			return res, nil
		}
	}
	return nil, nil
}

func (m *Provider) Quote(ctx context.Context, symbols []string) ([]provider.Quote, error) {
	for _, p := range m.ps {
		res, err := p.Quote(ctx, symbols)
		if err == nil && len(res) > 0 {
			return res, nil
		}
	}
	return nil, nil
}

func (m *Provider) Rank(ctx context.Context, market provider.Market, sort string, count int) ([]provider.RankItem, error) {
	for _, p := range m.ps {
		items, err := p.Rank(ctx, market, sort, count)
		if err == nil && len(items) > 0 {
			return items, nil
		}
	}
	return nil, nil
}

func (m *Provider) Index(ctx context.Context, market provider.Market) ([]provider.Quote, error) {
	for _, p := range m.ps {
		res, err := p.Index(ctx, market)
		if err == nil && len(res) > 0 {
			return res, nil
		}
	}
	return nil, nil
}

func (m *Provider) KlineDaily(ctx context.Context, symbol string, limit int) (provider.Kline, error) {
	for _, p := range m.ps {
		res, err := p.KlineDaily(ctx, symbol, limit)
		if err == nil && len(res.Bars) > 0 {
			return res, nil
		}
	}
	return provider.Kline{}, nil
}

func (m *Provider) FundFlow(ctx context.Context, symbol string) (provider.FundFlow, error) {
	for _, p := range m.ps {
		res, err := p.FundFlow(ctx, symbol)
		if err == nil {
			return res, nil
		}
	}
	return provider.FundFlow{}, nil
}

func (m *Provider) News(ctx context.Context, symbol string, count int) ([]provider.NewsItem, error) {
	for _, p := range m.ps {
		res, err := p.News(ctx, symbol, count)
		if err == nil && len(res) > 0 {
			return res, nil
		}
	}
	return nil, nil
}

func (m *Provider) Plate(ctx context.Context, symbol string) ([]provider.PlateItem, error) {
	for _, p := range m.ps {
		res, err := p.Plate(ctx, symbol)
		if err == nil && len(res) > 0 {
			return res, nil
		}
	}
	return nil, nil
}

func (m *Provider) MarketDistribution(ctx context.Context, market provider.Market) (provider.MarketDistribution, error) {
	for _, p := range m.ps {
		res, err := p.MarketDistribution(ctx, market)
		if err == nil && (res.Up+res.Down+res.Flat) > 0 {
			return res, nil
		}
	}
	return provider.MarketDistribution{}, nil
}
