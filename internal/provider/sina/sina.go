package sina

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"agent-stock/internal/netx"
	"agent-stock/internal/provider"
)

type Provider struct {
	http *netx.Client
}

func New() *Provider { return &Provider{http: netx.NewClient()} }

func (p *Provider) Search(ctx context.Context, keyword string, market provider.Market) ([]provider.SearchResult, error) {
	return nil, fmt.Errorf("not supported by sina")
}

func (p *Provider) Quote(ctx context.Context, symbols []string) ([]provider.Quote, error) {
	return nil, fmt.Errorf("not supported by sina")
}

func (p *Provider) Rank(ctx context.Context, market provider.Market, sortKey string, count int) ([]provider.RankItem, error) {
	if market != provider.MarketAB {
		return nil, fmt.Errorf("market %s not supported yet", market)
	}
	if count <= 0 {
		count = 20
	}
	if count > 100 {
		count = 100
	}
	sinaSort := "amount"
	switch sortKey {
	case "", "turnover":
		sinaSort = "amount"
	case "priceRatio":
		sinaSort = "changepercent"
	}
	u := fmt.Sprintf("https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=1&num=%d&sort=%s&asc=0&node=hs_a&symbol=&_s_r_a=init", count, url.QueryEscape(sinaSort))
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp []struct {
		Code     string  `json:"code"`
		Name     string  `json:"name"`
		Trade    string  `json:"trade"`
		Change   float64 `json:"changepercent"`
		Amount   float64 `json:"amount"`
		Turnover float64 `json:"turnoverratio"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}
	out := make([]provider.RankItem, 0, len(resp))
	for _, it := range resp {
		price, _ := strconv.ParseFloat(it.Trade, 64)
		out = append(out, provider.RankItem{
			Symbol:    it.Code,
			Name:      it.Name,
			Price:     price,
			ChangePct: it.Change,
			Amount:    it.Amount,
			Turnover:  it.Turnover,
		})
	}
	return out, nil
}

func (p *Provider) Index(ctx context.Context, market provider.Market) ([]provider.Quote, error) {
	return nil, fmt.Errorf("not supported by sina")
}

func (p *Provider) KlineDaily(ctx context.Context, symbol string, limit int) (provider.Kline, error) {
	return provider.Kline{}, fmt.Errorf("not supported by sina")
}

func (p *Provider) FundFlow(ctx context.Context, symbol string) (provider.FundFlow, error) {
	return provider.FundFlow{}, fmt.Errorf("not supported by sina")
}

func (p *Provider) News(ctx context.Context, symbol string, count int) ([]provider.NewsItem, error) {
	return nil, fmt.Errorf("not supported by sina")
}

func (p *Provider) Plate(ctx context.Context, symbol string) ([]provider.PlateItem, error) {
	return nil, fmt.Errorf("not supported by sina")
}

func (p *Provider) MarketDistribution(ctx context.Context, market provider.Market) (provider.MarketDistribution, error) {
	return provider.MarketDistribution{}, fmt.Errorf("not supported by sina")
}
