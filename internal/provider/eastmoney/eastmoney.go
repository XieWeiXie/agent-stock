package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"agent-stock/internal/netx"
	"agent-stock/internal/provider"
)

type Provider struct {
	http *netx.Client
}

func New() *Provider {
	return &Provider{http: netx.NewClient()}
}

func (p *Provider) Search(ctx context.Context, keyword string, market provider.Market) ([]provider.SearchResult, error) {
	if market != provider.MarketAB {
		return nil, fmt.Errorf("market %s not supported yet", market)
	}
	u := "https://searchapi.eastmoney.com/api/suggest/get?input=" + url.QueryEscape(keyword) + "&type=14&count=20"
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp struct {
		QuotationCodeTable struct {
			Data []struct {
				Code   string `json:"Code"`
				Name   string `json:"Name"`
				Market string `json:"Market"`
			} `json:"Data"`
		} `json:"QuotationCodeTable"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}

	out := make([]provider.SearchResult, 0, len(resp.QuotationCodeTable.Data))
	for _, it := range resp.QuotationCodeTable.Data {
		symbol := strings.TrimSpace(it.Code)
		if symbol == "" {
			continue
		}
		out = append(out, provider.SearchResult{
			Symbol: symbol,
			Name:   strings.TrimSpace(it.Name),
			Market: provider.MarketAB,
		})
	}
	return out, nil
}

func (p *Provider) Quote(ctx context.Context, symbols []string) ([]provider.Quote, error) {
	type req struct {
		orig string
		code string
	}

	var reqs []req
	secids := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		sym = strings.TrimSpace(sym)
		if sym == "" {
			continue
		}
		secid, err := secIDFromSymbol(sym)
		if err != nil {
			return nil, fmt.Errorf("quote %s: %w", sym, err)
		}
		secids = append(secids, secid)
		reqs = append(reqs, req{orig: sym, code: secIDToCode(secid)})
	}
	if len(secids) == 0 {
		return nil, nil
	}

	quotes, err := p.quoteBySecIDs(ctx, secids)
	if err != nil {
		return nil, err
	}

	byCode := make(map[string]provider.Quote, len(quotes))
	for _, q := range quotes {
		byCode[q.Symbol] = q
	}

	out := make([]provider.Quote, 0, len(reqs))
	for _, r := range reqs {
		if q, ok := byCode[r.code]; ok {
			out = append(out, q)
			continue
		}
		out = append(out, provider.Quote{Symbol: r.code})
	}
	return out, nil
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

	fid := "f6"
	switch sortKey {
	case "", "turnover":
		fid = "f6"
	case "amplitude":
		fid = "f7"
	case "volumeRatio":
		fid = "f10"
	case "exchange":
		fid = "f8"
	case "priceRatio":
		fid = "f3"
	default:
		return nil, fmt.Errorf("unknown sort: %s", sortKey)
	}

	fs := url.QueryEscape("m:0+t:6,m:0+t:13,m:0+t:80,m:1+t:2,m:1+t:23")
	u := fmt.Sprintf("https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=%d&po=1&np=1&fltt=2&invt=2&fid=%s&fields=f2,f3,f12,f14,f6,f8&fs=%s", count, fid, fs)
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Diff []struct {
				Code   string  `json:"f12"`
				Name   string  `json:"f14"`
				Price  float64 `json:"f2"`
				Pct    float64 `json:"f3"`
				Amount float64 `json:"f6"`
				Turn   float64 `json:"f8"`
			} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}

	out := make([]provider.RankItem, 0, len(resp.Data.Diff))
	for _, it := range resp.Data.Diff {
		if strings.TrimSpace(it.Code) == "" {
			continue
		}
		out = append(out, provider.RankItem{
			Symbol:    it.Code,
			Name:      it.Name,
			Price:     it.Price,
			ChangePct: it.Pct,
			Amount:    it.Amount,
			Turnover:  it.Turn,
		})
	}
	return out, nil
}

func (p *Provider) Index(ctx context.Context, market provider.Market) ([]provider.Quote, error) {
	if market != provider.MarketAB {
		return nil, fmt.Errorf("market %s not supported yet", market)
	}

	secids := []string{
		"1.000001",
		"0.399001",
		"0.399006",
		"1.000300",
		"1.000905",
	}
	return p.quoteBySecIDs(ctx, secids)
}

func (p *Provider) KlineDaily(ctx context.Context, symbol string, limit int) (provider.Kline, error) {
	if limit <= 0 {
		limit = 120
	}
	if limit > 1000 {
		limit = 1000
	}

	secid, err := secIDFromSymbol(symbol)
	if err != nil {
		return provider.Kline{}, err
	}

	u := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58&klt=101&fqt=1&end=20500101&lmt=%d", url.QueryEscape(secid), limit)
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return provider.Kline{}, err
	}

	var resp struct {
		Data struct {
			Code   string   `json:"code"`
			Name   string   `json:"name"`
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return provider.Kline{}, err
	}

	k := provider.Kline{
		Symbol: symbol,
		Name:   resp.Data.Name,
		Bars:   make([]provider.KlineBar, 0, len(resp.Data.Klines)),
	}

	for _, row := range resp.Data.Klines {
		parts := strings.Split(row, ",")
		if len(parts) < 7 {
			continue
		}
		open, _ := strconv.ParseFloat(parts[1], 64)
		closep, _ := strconv.ParseFloat(parts[2], 64)
		high, _ := strconv.ParseFloat(parts[3], 64)
		low, _ := strconv.ParseFloat(parts[4], 64)
		vol, _ := strconv.ParseFloat(parts[5], 64)
		amt, _ := strconv.ParseFloat(parts[6], 64)
		k.Bars = append(k.Bars, provider.KlineBar{
			Date:   parts[0],
			Open:   open,
			Close:  closep,
			High:   high,
			Low:    low,
			Volume: vol,
			Amount: amt,
		})
	}
	return k, nil
}

func (p *Provider) quoteBySecIDs(ctx context.Context, secids []string) ([]provider.Quote, error) {
	fields := url.QueryEscape("f12,f14,f2,f3,f4,f5,f6,f15,f16,f17,f18,f20,f124")
	u := fmt.Sprintf("https://push2.eastmoney.com/api/qt/ulist.np/get?fltt=2&invt=2&secids=%s&fields=%s", url.QueryEscape(strings.Join(secids, ",")), fields)
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Diff []struct {
				Code string          `json:"f12"`
				Name string          `json:"f14"`
				P    float64         `json:"f2"`
				Pct  float64         `json:"f3"`
				Chg  float64         `json:"f4"`
				Vol  float64         `json:"f5"`
				Amt  float64         `json:"f6"`
				H    float64         `json:"f15"`
				L    float64         `json:"f16"`
				O    float64         `json:"f17"`
				Pre  float64         `json:"f18"`
				MV   float64         `json:"f20"`
				Time json.RawMessage `json:"f124"`
			} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}

	out := make([]provider.Quote, 0, len(resp.Data.Diff))
	for _, it := range resp.Data.Diff {
		out = append(out, provider.Quote{
			Symbol:      strings.TrimSpace(it.Code),
			Name:        strings.TrimSpace(it.Name),
			Price:       it.P,
			Change:      it.Chg,
			ChangePct:   it.Pct,
			Open:        it.O,
			High:        it.H,
			Low:         it.L,
			PrevClose:   it.Pre,
			Volume:      it.Vol,
			Amount:      it.Amt,
			MarketValue: it.MV,
			Time:        rawToString(it.Time),
		})
	}
	return out, nil
}

func secIDFromSymbol(symbol string) (string, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return "", fmt.Errorf("empty symbol")
	}

	if strings.HasPrefix(strings.ToLower(symbol), "us.") {
		return "", fmt.Errorf("us market not supported yet")
	}
	if len(symbol) == 5 && isDigits(symbol) {
		return "", fmt.Errorf("hk market not supported yet")
	}
	if len(symbol) == 6 && isDigits(symbol) {
		if strings.HasPrefix(symbol, "6") {
			return "1." + symbol, nil
		}
		return "0." + symbol, nil
	}
	if strings.Contains(symbol, ".") && isSecID(symbol) {
		return symbol, nil
	}
	return "", fmt.Errorf("unsupported symbol format: %s", symbol)
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isSecID(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return false
	}
	if parts[0] != "0" && parts[0] != "1" {
		return false
	}
	return isDigits(parts[1])
}

func secIDToCode(secid string) string {
	parts := strings.Split(secid, ".")
	if len(parts) != 2 {
		return secid
	}
	return parts[1]
}

func rawToString(v json.RawMessage) string {
	if len(v) == 0 || string(v) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var n json.Number
	dec := json.NewDecoder(strings.NewReader(string(v)))
	dec.UseNumber()
	if err := dec.Decode(&n); err == nil {
		return n.String()
	}
	return strings.Trim(string(v), "\"")
}
