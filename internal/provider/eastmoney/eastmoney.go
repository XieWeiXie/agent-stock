package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

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

	// Try Eastmoney first
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
	u := fmt.Sprintf("https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=%d&po=1&np=1&fltt=2&invt=2&fid=%s&fields=f2,f3,f12,f14,f6,f8,f20&fs=%s", count, fid, fs)
	b, err := p.http.Get(ctx, u)
	if err == nil {
		var resp struct {
			Data struct {
				Diff []struct {
					Code       string  `json:"f12"`
					Name       string  `json:"f14"`
					Price      float64 `json:"f2"`
					Pct        float64 `json:"f3"`
					Amount     float64 `json:"f6"`
					Turn       float64 `json:"f8"`
					MarketVal  float64 `json:"f20"` // 总市值，单位：元
				} `json:"diff"`
			} `json:"data"`
		}
		if err := json.Unmarshal(b, &resp); err == nil && len(resp.Data.Diff) > 0 {
			out := make([]provider.RankItem, 0, len(resp.Data.Diff))
			for _, it := range resp.Data.Diff {
				if strings.TrimSpace(it.Code) == "" {
					continue
				}
				out = append(out, provider.RankItem{
					Symbol:      it.Code,
					Name:        it.Name,
					Price:       it.Price,
					ChangePct:   it.Pct,
					Amount:      it.Amount,
					Turnover:    it.Turn,
					MarketValue: it.MarketVal,
				})
			}
			return out, nil
		}
	}

	// Fallback to Sina
	sinaSort := "amount"
	switch sortKey {
	case "turnover":
		sinaSort = "amount" // Sina doesn't support turnover sort in this API easily
	case "priceRatio":
		sinaSort = "changepercent"
	}
	uSina := fmt.Sprintf("https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=1&num=%d&sort=%s&asc=0&node=hs_a&symbol=&_s_r_a=init", count, sinaSort)
	bSina, errSina := p.http.Get(ctx, uSina)
	if errSina != nil {
		return nil, errSina
	}

	var sinaResp []struct {
		Code      string  `json:"code"`
		Name      string  `json:"name"`
		Price     string  `json:"trade"`
		Pct       float64 `json:"changepercent"`
		Amount    float64 `json:"amount"`
		Turnover  float64 `json:"turnoverratio"`
	}
	if err := json.Unmarshal(bSina, &sinaResp); err != nil {
		return nil, err
	}

	out := make([]provider.RankItem, 0, len(sinaResp))
	for _, it := range sinaResp {
		price, _ := strconv.ParseFloat(it.Price, 64)
		out = append(out, provider.RankItem{
			Symbol:    it.Code,
			Name:      it.Name,
			Price:     price,
			ChangePct: it.Pct,
			Amount:    it.Amount,
			Turnover:  it.Turnover,
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
	if err == nil {
		var resp struct {
			Data struct {
				Code   string   `json:"code"`
				Name   string   `json:"name"`
				Klines []string `json:"klines"`
			} `json:"data"`
		}
		if err := json.Unmarshal(b, &resp); err == nil && len(resp.Data.Klines) > 0 {
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
	}

	// Fallback: use Sina daily kline
	return p.klineDailyFromSina(ctx, symbol, limit)
}

// klineDailyFromSina fetches daily data from Sina
func (p *Provider) klineDailyFromSina(ctx context.Context, symbol string, limit int) (provider.Kline, error) {
	sinaSymbol := symbolToSina(symbol)
	if sinaSymbol == "" {
		return provider.Kline{}, fmt.Errorf("cannot convert symbol %s to sina format", symbol)
	}

	u := fmt.Sprintf("https://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=240&ma=no&datalen=%d", url.QueryEscape(sinaSymbol), limit)
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return provider.Kline{}, fmt.Errorf("sina daily kline failed: %w", err)
	}

	var dailyBars []sinaDailyBar
	if err := json.Unmarshal(b, &dailyBars); err != nil {
		return provider.Kline{}, fmt.Errorf("sina daily kline parse failed: %w", err)
	}

	if len(dailyBars) == 0 {
		return provider.Kline{}, fmt.Errorf("no daily data from sina for %s", symbol)
	}

	bars := make([]provider.KlineBar, 0, len(dailyBars))
	for _, db := range dailyBars {
		open, _ := strconv.ParseFloat(db.Open, 64)
		close, _ := strconv.ParseFloat(db.Close, 64)
		high, _ := strconv.ParseFloat(db.High, 64)
		low, _ := strconv.ParseFloat(db.Low, 64)
		vol, _ := strconv.ParseFloat(db.Volume, 64)
		bars = append(bars, provider.KlineBar{
			Date:   db.Day,
			Open:   open,
			Close:  close,
			High:   high,
			Low:    low,
			Volume: vol,
			Amount: 0,
		})
	}

	return provider.Kline{
		Symbol: symbol,
		Name:   "",
		Bars:   bars,
	}, nil
}

func (p *Provider) KlineWeekly(ctx context.Context, symbol string, limit int) (provider.Kline, error) {
	if limit <= 0 {
		limit = 52 // 默认取一年周线数据
	}
	if limit > 200 {
		limit = 200
	}

	secid, err := secIDFromSymbol(symbol)
	if err != nil {
		return provider.Kline{}, err
	}

	// klt=102 表示周线数据
	u := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58&klt=102&fqt=1&end=20500101&lmt=%d", url.QueryEscape(secid), limit)
	b, err := p.http.Get(ctx, u)
	if err == nil {
		var resp struct {
			Data struct {
				Code   string   `json:"code"`
				Name   string   `json:"name"`
				Klines []string `json:"klines"`
			} `json:"data"`
		}
		if err := json.Unmarshal(b, &resp); err == nil && len(resp.Data.Klines) > 0 {
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
		// Eastmoney API returned data but empty klines or parse error
		// Fall through to Sina fallback
	}

	// Fallback: use Sina daily data and aggregate to weekly
	k, err := p.klineWeeklyFromSinaDaily(ctx, symbol, limit)
	if err != nil {
		// Return error with context so multi provider knows to try next provider
		return provider.Kline{}, err
	}
	if len(k.Bars) == 0 {
		return provider.Kline{}, fmt.Errorf("sina fallback returned empty bars")
	}
	return k, nil
}

// klineWeeklyFromSinaDaily fetches daily data from Sina and aggregates into weekly bars
func (p *Provider) klineWeeklyFromSinaDaily(ctx context.Context, symbol string, limit int) (provider.Kline, error) {
	// Convert symbol to sina format (sz002463 or sh600111)
	sinaSymbol := symbolToSina(symbol)
	if sinaSymbol == "" {
		return provider.Kline{}, fmt.Errorf("cannot convert symbol %s to sina format", symbol)
	}

	// Fetch enough daily data to cover the weekly limit (5 trading days per week + buffer)
	dailyLimit := limit*7 + 10
	u := fmt.Sprintf("https://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=240&ma=no&datalen=%d", url.QueryEscape(sinaSymbol), dailyLimit)
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return provider.Kline{}, fmt.Errorf("sina kline HTTP failed: %w", err)
	}

	var dailyBars []sinaDailyBar
	if err := json.Unmarshal(b, &dailyBars); err != nil {
		return provider.Kline{}, fmt.Errorf("sina kline parse failed: %w", err)
	}

	if len(dailyBars) == 0 {
		return provider.Kline{}, fmt.Errorf("no daily data from sina for %s", symbol)
	}

	// Aggregate daily bars into weekly bars
	weeklyBars := aggregateWeekly(dailyBars)

	// Trim to requested limit
	if len(weeklyBars) > limit {
		weeklyBars = weeklyBars[len(weeklyBars)-limit:]
	}

	return provider.Kline{
		Symbol: symbol,
		Name:   "",
		Bars:   weeklyBars,
	}, nil
}

type sinaDailyBar struct {
	Day    string `json:"day"`
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

func aggregateWeekly(dailyBars []sinaDailyBar) []provider.KlineBar {
	if len(dailyBars) == 0 {
		return nil
	}

	var weeks []provider.KlineBar
	var weekBars []sinaDailyBar

	for _, bar := range dailyBars {
		t, err := time.Parse("2006-01-02", bar.Day)
		if err != nil {
			continue
		}

		// Get the ISO week number and year
		year, week := t.ISOWeek()

		if len(weekBars) > 0 {
			prevT, _ := time.Parse("2006-01-02", weekBars[0].Day)
			prevYear, prevWeek := prevT.ISOWeek()
			if year != prevYear || week != prevWeek {
				// New week started, aggregate previous week
				weeks = append(weeks, makeWeeklyBar(weekBars))
				weekBars = weekBars[:0]
			}
		}

		weekBars = append(weekBars, bar)
	}

	// Don't forget the last week
	if len(weekBars) > 0 {
		weeks = append(weeks, makeWeeklyBar(weekBars))
	}

	return weeks
}

func makeWeeklyBar(bars []sinaDailyBar) provider.KlineBar {
	if len(bars) == 0 {
		return provider.KlineBar{}
	}
	open, _ := strconv.ParseFloat(bars[0].Open, 64)
	openHigh, _ := strconv.ParseFloat(bars[0].High, 64)
	openLow, _ := strconv.ParseFloat(bars[0].Low, 64)
	closePrice, _ := strconv.ParseFloat(bars[len(bars)-1].Close, 64)
	vol, _ := strconv.ParseFloat(bars[0].Volume, 64)

	w := provider.KlineBar{
		Date:   bars[0].Day,
		Open:   open,
		High:   openHigh,
		Low:    openLow,
		Close:  closePrice,
		Volume: vol,
	}
	for _, b := range bars {
		high, _ := strconv.ParseFloat(b.High, 64)
		low, _ := strconv.ParseFloat(b.Low, 64)
		v, _ := strconv.ParseFloat(b.Volume, 64)
		if high > w.High {
			w.High = high
		}
		if low < w.Low {
			w.Low = low
		}
		w.Volume += v
	}
	return w
}

func symbolToSina(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if len(symbol) != 6 || !isDigits(symbol) {
		return ""
	}
	if strings.HasPrefix(symbol, "6") {
		return "sh" + symbol
	}
	return "sz" + symbol
}

func (p *Provider) FundFlow(ctx context.Context, symbol string) (provider.FundFlow, error) {
	secid, err := secIDFromSymbol(symbol)
	if err != nil {
		return provider.FundFlow{}, err
	}

	// Use stock/get which is more reliable for individual stocks
	fields := "f57,f58,f62,f184,f66,f69,f72,f75,f78,f81,f84,f87"
	u := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=%s", url.QueryEscape(secid), fields)
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return provider.FundFlow{}, err
	}

	var resp struct {
		Data struct {
			Code    string  `json:"f57"`
			Name    string  `json:"f58"`
			MainIn  float64 `json:"f62"`
			MainPct float64 `json:"f184"`
			HugeIn  float64 `json:"f66"`
			LargeIn float64 `json:"f72"`
			MedIn   float64 `json:"f78"`
			SmallIn float64 `json:"f84"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return provider.FundFlow{}, err
	}

	return provider.FundFlow{
		Symbol:   resp.Data.Code,
		Name:     resp.Data.Name,
		MainIn:   resp.Data.MainIn,
		MainPct:  resp.Data.MainPct,
		HugeIn:   resp.Data.HugeIn,
		LargeIn:  resp.Data.LargeIn,
		MediumIn: resp.Data.MedIn,
		SmallIn:  resp.Data.SmallIn,
	}, nil
}

func (p *Provider) News(ctx context.Context, symbol string, count int) ([]provider.NewsItem, error) {
	if count <= 0 {
		count = 10
	}
	// type=8 is news suggest API
	u := fmt.Sprintf("https://searchapi.eastmoney.com/api/suggest/get?input=%s&type=8&count=%d", url.QueryEscape(symbol), count)
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	// The response structure might be different for suggest API type 8
	// Let's assume it's like this for now, but usually it's wrapped
	var wrap struct {
		NewsTable struct {
			Data []struct {
				Title string `json:"Title"`
				Url   string `json:"Url"`
				Time  string `json:"Time"`
			} `json:"Data"`
		} `json:"NewsTable"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, err
	}

	out := make([]provider.NewsItem, 0, len(wrap.NewsTable.Data))
	for _, it := range wrap.NewsTable.Data {
		out = append(out, provider.NewsItem{
			Title: it.Title,
			Url:   it.Url,
			Time:  it.Time,
		})
	}
	return out, nil
}

func (p *Provider) Plate(ctx context.Context, symbol string) ([]provider.PlateItem, error) {
	secid, err := secIDFromSymbol(symbol)
	if err != nil {
		return nil, err
	}

	// Related plates API
	u := fmt.Sprintf("https://push2.eastmoney.com/api/qt/slist/get?secid=%s&fields=f12,f14,f2,f3", url.QueryEscape(secid))
	b, err := p.http.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Diff []struct {
				Code string  `json:"f12"`
				Name string  `json:"f14"`
				Pct  float64 `json:"f3"`
			} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}

	out := make([]provider.PlateItem, 0, len(resp.Data.Diff))
	for _, it := range resp.Data.Diff {
		out = append(out, provider.PlateItem{
			Symbol:    it.Code,
			Name:      it.Name,
			ChangePct: it.Pct,
		})
	}
	return out, nil
}

func (p *Provider) MarketDistribution(ctx context.Context, market provider.Market) (provider.MarketDistribution, error) {
	if market != provider.MarketAB {
		return provider.MarketDistribution{}, fmt.Errorf("market %s not supported yet", market)
	}

	fs := url.QueryEscape("m:0+t:6,m:0+t:13,m:0+t:80,m:1+t:2,m:1+t:23")
	var d provider.MarketDistribution
	// paginate to reduce server pressure and avoid EOF; 1000 per page
	for pn := 1; pn <= 10; pn++ {
		u := fmt.Sprintf("https://push2.eastmoney.com/api/qt/clist/get?pn=%d&pz=1000&po=1&np=1&fltt=2&invt=2&fid=f3&fields=f3&fs=%s", pn, fs)
		b, err := p.http.Get(ctx, u)
		if err != nil {
			u = u + "&ut=b2884a393a59ad64002292a3e90d46a5"
			b, err = p.http.Get(ctx, u)
		}
		if err != nil {
			// try http scheme
			u = strings.ReplaceAll(u, "https://", "http://")
			b, err = p.http.Get(ctx, u)
			if err != nil {
				return provider.MarketDistribution{}, err
			}
		}
		var wrap struct {
			Data struct {
				Diff json.RawMessage `json:"diff"`
			} `json:"data"`
		}
		if err := json.Unmarshal(b, &wrap); err != nil {
			return provider.MarketDistribution{}, err
		}
		if len(wrap.Data.Diff) == 0 {
			break
		}
		type row struct {
			Pct float64 `json:"f3"`
		}
		var rows []row
		if len(wrap.Data.Diff) > 0 && wrap.Data.Diff[0] == '{' {
			var obj map[string]row
			if err := json.Unmarshal(wrap.Data.Diff, &obj); err != nil {
				return provider.MarketDistribution{}, err
			}
			for _, v := range obj {
				rows = append(rows, v)
			}
		} else {
			if err := json.Unmarshal(wrap.Data.Diff, &rows); err != nil {
				return provider.MarketDistribution{}, err
			}
		}
		if len(rows) == 0 {
			break
		}
		for _, it := range rows {
			pct := it.Pct
			if pct > 0 {
				d.Up++
				if pct >= 9.9 {
					d.UpLimit++
				}
				if pct > 9 {
					d.Up9++
				} else if pct > 6 {
					d.Up6_9++
				} else if pct > 3 {
					d.Up3_6++
				} else {
					d.Up0_3++
				}
			} else if pct < 0 {
				d.Down++
				if pct <= -9.9 {
					d.DownLimit++
				}
				if pct < -9 {
					d.Down9++
				} else if pct < -6 {
					d.Down6_9++
				} else if pct < -3 {
					d.Down3_6++
				} else {
					d.Down0_3++
				}
			} else {
				d.Flat++
			}
		}
		// if less than page size, stop
		if len(rows) < 1000 {
			break
		}
	}
	return d, nil
}

func (p *Provider) quoteBySecIDs(ctx context.Context, secids []string) ([]provider.Quote, error) {
	out := make([]provider.Quote, 0, len(secids))
	fields := url.QueryEscape("f57,f58,f43,f44,f45,f46,f60,f47,f48")
	for _, secid := range secids {
		u := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=%s", url.QueryEscape(secid), fields)
		b, err := p.http.Get(ctx, u)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Data struct {
				Code  string  `json:"f57"`
				Name  string  `json:"f58"`
				P     float64 `json:"f43"`
				H     float64 `json:"f44"`
				L     float64 `json:"f45"`
				O     float64 `json:"f46"`
				Pre   float64 `json:"f60"`
				Vol   float64 `json:"f47"`
				Amt   float64 `json:"f48"`
			} `json:"data"`
		}
		if err := json.Unmarshal(b, &resp); err != nil {
			return nil, err
		}
		px := resp.Data.P / 100
		pre := resp.Data.Pre / 100
		var chg, pct float64
		if pre != 0 {
			chg = px - pre
			pct = chg / pre * 100
		}
		out = append(out, provider.Quote{
			Symbol:    strings.TrimSpace(resp.Data.Code),
			Name:      strings.TrimSpace(resp.Data.Name),
			Price:     px,
			Change:    chg,
			ChangePct: pct,
			Open:      resp.Data.O / 100,
			High:      resp.Data.H / 100,
			Low:       resp.Data.L / 100,
			PrevClose: pre,
			Volume:    resp.Data.Vol,
			Amount:    resp.Data.Amt,
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

// GetMarketCaps fetches market cap for multiple stocks using the ulist API
// Returns map of symbol -> market cap in 元 (yuan), or nil if API fails
func (p *Provider) GetMarketCaps(ctx context.Context, symbols []string) map[string]float64 {
	if len(symbols) == 0 {
		return nil
	}

	// Build secids list
	secids := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		sym = strings.TrimSpace(sym)
		if sym == "" {
			continue
		}
		secid, err := secIDFromSymbol(sym)
		if err != nil {
			continue
		}
		secids = append(secids, secid)
	}

	if len(secids) == 0 {
		return nil
	}

	// Batch query - ulist API supports up to ~200 stocks per request
	result := make(map[string]float64)

	// Process in batches of 50 to avoid URL too long
	batchSize := 50
	for i := 0; i < len(secids); i += batchSize {
		end := i + batchSize
		if end > len(secids) {
			end = len(secids)
		}
		batch := secids[i:end]

		secidsParam := strings.Join(batch, ",")
		u := fmt.Sprintf("https://push2.eastmoney.com/api/qt/ulist.np/get?fltt=2&invt=2&fields=f12,f20&secids=%s", url.QueryEscape(secidsParam))

		b, err := p.http.Get(ctx, u)
		if err != nil {
			continue
		}

		var resp struct {
			Data struct {
				Diff []struct {
					Code  string  `json:"f12"`
					MktVal float64 `json:"f20"` // 总市值，单位：元
				} `json:"diff"`
			} `json:"data"`
		}
		if err := json.Unmarshal(b, &resp); err != nil {
			continue
		}

		for _, it := range resp.Data.Diff {
			code := strings.TrimSpace(it.Code)
			if code != "" {
				result[code] = it.MktVal
			}
		}
	}

	return result
}
