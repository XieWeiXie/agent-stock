package tencent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"agent-stock/internal/netx"
	"agent-stock/internal/provider"
)

// Tencent qt API field indices
const (
	qtName    = 1  // 名称
	qtCode    = 2  // 代码
	qtPrice   = 3  // 现价
	qtYClose  = 4  // 昨收
	qtOpen    = 5  // 开盘
	qtPct     = 32 // 涨跌幅%
	qtHigh    = 33 // 最高
	qtLow     = 34 // 最低
	qtVol     = 36 // 成交量(手)
	qtAmt     = 37 // 成交额(万)
	qtMktCap  = 44 // 总市值(亿)
)

type Provider struct {
	http *netx.Client
}

func New() *Provider {
	return &Provider{http: netx.NewClient()}
}

// Ensure Provider implements the interface
var _ provider.Provider = (*Provider)(nil)

func (p *Provider) Search(ctx context.Context, keyword string, market provider.Market) ([]provider.SearchResult, error) {
	return nil, fmt.Errorf("search not supported by tencent")
}

func (p *Provider) Quote(ctx context.Context, symbols []string) ([]provider.Quote, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	syms := make([]string, 0, len(symbols))
	for _, s := range symbols {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		syms = append(syms, toTencentCode(s))
	}
	if len(syms) == 0 {
		return nil, nil
	}

	// Batch in groups of 50
	out := make([]provider.Quote, 0, len(syms))
	for i := 0; i < len(syms); i += 50 {
		end := i + 50
		if end > len(syms) {
			end = len(syms)
		}
		quotes, err := p.quoteBatch(ctx, syms[i:end])
		if err != nil {
			continue
		}
		out = append(out, quotes...)
	}
	return out, nil
}

func (p *Provider) quoteBatch(ctx context.Context, symbols []string) ([]provider.Quote, error) {
	u := "https://qt.gtimg.cn/q=" + strings.Join(symbols, ",")
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	var out []provider.Quote
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "v_") {
			continue
		}

		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}

		// Extract fields between quotes
		raw := line[strings.Index(line, `"`)+1:]
		if idx := strings.LastIndex(raw, `"`); idx >= 0 {
			raw = raw[:idx]
		}
		fields := strings.Split(raw, "~")
		if len(fields) < 45 {
			continue
		}

		price, _ := strconv.ParseFloat(fields[qtPrice], 64)
		yClose, _ := strconv.ParseFloat(fields[qtYClose], 64)
		mktCap, _ := strconv.ParseFloat(fields[qtMktCap], 64)

		symbol := fromTencentCode(line[2:eq])

		out = append(out, provider.Quote{
			Symbol:      symbol,
			Name:        fields[qtName],
			Price:       price,
			Change:      price - yClose,
			ChangePct:   parseFloat(fields[qtPct]),
			Open:        parseFloat(fields[qtOpen]),
			High:        parseFloat(fields[qtHigh]),
			Low:         parseFloat(fields[qtLow]),
			Volume:      parseFloat(fields[qtVol]) * 100,
			Amount:      parseFloat(fields[qtAmt]) * 10000,
			MarketValue: mktCap * 1e8, // 亿 -> 元
		})
	}
	return out, nil
}

func (p *Provider) Rank(ctx context.Context, market provider.Market, sortKey string, count int) ([]provider.RankItem, error) {
	return nil, fmt.Errorf("rank not supported by tencent")
}

func (p *Provider) Index(ctx context.Context, market provider.Market) ([]provider.Quote, error) {
	return p.Quote(ctx, []string{"000001", "399001", "399006", "000300", "000905"})
}

func (p *Provider) KlineDaily(ctx context.Context, symbol string, limit int) (provider.Kline, error) {
	return p.kline(ctx, symbol, "day", "qfqday", limit, 1000, 120)
}

func (p *Provider) KlineWeekly(ctx context.Context, symbol string, limit int) (provider.Kline, error) {
	return p.kline(ctx, symbol, "week", "qfqweek", limit, 500, 60)
}

func (p *Provider) kline(ctx context.Context, symbol, period, jsonKey string, limit, maxLimit, defaultLimit int) (provider.Kline, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	prefix := toTencentCode(symbol)
	u := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?_var=kline_%s&param=%s,%s,,,%d,qfq",
		period, prefix, symbol, limit)

	b, err := p.http.Get(ctx, u)
	if err != nil {
		return provider.Kline{}, err
	}

	// Strip "kline_day=..." prefix to get JSON
	respStr := string(b)
	if idx := strings.Index(respStr, "="); idx != -1 {
		respStr = respStr[idx+1:]
	}

	// Parse: {"code":0,"data":{"sz000001":{"qfqday":[[...]],"qt":{"sz000001":[...]}}}}
	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(respStr), &resp); err != nil {
		return provider.Kline{}, fmt.Errorf("tencent kline parse error for %s: %w", symbol, err)
	}

	// Extract kline data from the dynamic key
	var bars [][]string
	var name string
	for _, raw := range resp.Data {
		// Try to extract the kline array by key name
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if klineRaw, ok := m[jsonKey]; ok {
			json.Unmarshal(klineRaw, &bars)
		}
		// Extract name from qt
		if qtRaw, ok := m["qt"]; ok {
			var qt map[string][]string
			if json.Unmarshal(qtRaw, &qt) == nil {
				for _, v := range qt {
					if len(v) > 1 {
						name = v[1]
					}
					break
				}
			}
		}
		break
	}

	if len(bars) == 0 {
		return provider.Kline{}, fmt.Errorf("no %s kline data for %s", period, symbol)
	}

	kl := provider.Kline{Symbol: symbol, Name: name}
	for _, k := range bars {
		if len(k) < 6 {
			continue
		}
		kl.Bars = append(kl.Bars, provider.KlineBar{
			Date:   k[0],
			Open:   parseFloat(k[1]),
			Close:  parseFloat(k[2]),
			High:   parseFloat(k[3]),
			Low:    parseFloat(k[4]),
			Volume: parseFloat(k[5]),
		})
	}
	return kl, nil
}

func (p *Provider) FundFlow(ctx context.Context, symbol string) (provider.FundFlow, error) {
	return provider.FundFlow{}, fmt.Errorf("fund flow not supported by tencent")
}

func (p *Provider) News(ctx context.Context, symbol string, count int) ([]provider.NewsItem, error) {
	return nil, fmt.Errorf("news not supported by tencent")
}

func (p *Provider) Plate(ctx context.Context, symbol string) ([]provider.PlateItem, error) {
	return nil, fmt.Errorf("plate not supported by tencent")
}

func (p *Provider) MarketDistribution(ctx context.Context, market provider.Market) (provider.MarketDistribution, error) {
	return provider.MarketDistribution{}, fmt.Errorf("market distribution not supported by tencent")
}

// GetMarketCaps fetches market cap for multiple stocks using qt API
// Returns map[symbol]marketCapIn元
func (p *Provider) GetMarketCaps(ctx context.Context, symbols []string) map[string]float64 {
	quotes, err := p.Quote(ctx, symbols)
	if err != nil || len(quotes) == 0 {
		return nil
	}
	result := make(map[string]float64, len(quotes))
	for _, q := range quotes {
		result[q.Symbol] = q.MarketValue
	}
	return result
}

// Helpers

// toTencentCode converts "000001" -> "sz000001", "600519" -> "sh600519"
func toTencentCode(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	// Already has prefix
	if strings.HasPrefix(symbol, "sz") || strings.HasPrefix(symbol, "sh") {
		return symbol
	}
	if len(symbol) == 6 {
		switch {
		case symbol[0] == '6', symbol[0] == '9': // 沪市主板 + 科创板(688xxx)
			return "sh" + symbol
		default: // 深市(000/001/002/003/300)
			return "sz" + symbol
		}
	}
	return "sz" + symbol
}

// fromTencentCode converts "sz000001" -> "000001"
func fromTencentCode(code string) string {
	code = strings.TrimPrefix(code, "v_")
	code = strings.TrimPrefix(code, "sz")
	code = strings.TrimPrefix(code, "sh")
	return code
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
