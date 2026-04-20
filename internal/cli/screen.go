package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"agent-stock/internal/format"
	"agent-stock/internal/indicator"
	"agent-stock/internal/provider"
	"agent-stock/internal/provider/eastmoney"
	"agent-stock/internal/provider/multi"
	"agent-stock/internal/provider/sina"
	"agent-stock/internal/provider/tencent"
)

// ScreenResult represents a stock that passed the screen criteria
type ScreenResult struct {
	Symbol        string
	Name          string
	Price         float64
	ChangePct     float64
	MarketValue   float64 // 亿元
	WeeklyPattern string  // 周线形态
	PatternDesc   string  // 形态描述
	Score         int     // 评分 1-5
	MAStatus      string  // MA状态
	TrendStatus   string  // 趋势状态
	EntryZone     string  // 入场区间
	RiskLevel     string  // 风险等级

	// 量价分析
	VolumeAnalysis string  // 量能分析
	VolumeRatio   string  // 量比
	AmountTrend   string  // 成交额趋势

	// 缠论分析
	ChanLevel      string  // 缠论级别
	ChanStatus     string  // 缠论状态
	Chan中枢       string  // 中枢区间
	ChanBuyPoint   string  // 缠论买点
	ChanTrend      string  // 趋势方向

	// 日线级别买点分析
	DailyBuySignal string  // 日线买点信号
	DailyDesc      string  // 日线描述
	DailyEntry     string  // 日线入场价
	DailyStopLoss  string  // 止损位
	DailySupport   string  // 支撑位
}

// ScreenCommand implements weekly MA breakout screening
type ScreenCommand struct{}

func NewScreenCommand() *ScreenCommand { return &ScreenCommand{} }

func (c *ScreenCommand) Name() string     { return "screen" }
func (c *ScreenCommand) Synopsis() string { return "周线级别选股：MA有效突破、缠论买点、市值筛选" }

func (c *ScreenCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s screen [flags]

Flags:
  --market-cap value    最小市值筛选，单位：亿元 (default: 500)
  --limit count         最多分析股票数量 (default: 200)
  --count result        最多输出结果数量 (default: 30)
  --workers n           并发数 (default: 3)
  --delay ms            每次请求间隔（毫秒）(default: 300)

Description:
  周线级别选股策略，筛选条件：
  1. 市值大于指定值
  2. 周线MA有效突破（MA5>MA10>MA20）
  3. 周线级别回调形成1买/2买
  4. 应用缠论理论识别趋势和买点

Examples:
  %s screen
  %s screen --market-cap 500
  %s screen --limit 200 --count 30 --workers 3 --delay 300
`, appName, appName, appName, appName)
}

func (c *ScreenCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	fs := flag.NewFlagSet(appName+" "+c.Name(), flag.ContinueOnError)
	fs.SetOutput(errOut)
	marketCap := fs.Float64("market-cap", 500, "最小市值筛选，单位：亿元")
	limit := fs.Int("limit", 200, "最多分析股票数量")
	count := fs.Int("count", 30, "最多输出结果数量")
	workers := fs.Int("workers", 3, "并发数")
	delay := fs.Int("delay", 300, "每次请求间隔（毫秒）")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Fprintf(out, "正在获取股票列表（市值>%v亿）...\n", *marketCap)

	// Step 1: Get all A-share stocks with market cap > threshold
	stocks, err := c.fetchStocksByMarketCap(ctx, *marketCap, *limit)
	if err != nil {
		return fmt.Errorf("获取股票列表失败: %w", err)
	}

	fmt.Fprintf(out, "获取到 %d 只股票，开始分析周线（并发%d，请求间隔%dms）...\n\n", len(stocks), *workers, *delay)

	// Step 2: Analyze weekly patterns for each stock
	results := c.analyzeStocks(ctx, stocks, *workers, time.Duration(*delay)*time.Millisecond)

	// Step 3: Sort by score and filter
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].MarketValue > results[j].MarketValue
	})

	if len(results) > *count {
		results = results[:*count]
	}

	// Step 4: Output results
	c.printResults(out, results)

	return nil
}

type stockInfo struct {
	Symbol      string
	Name        string
	Price       float64
	ChangePct   float64
	MarketValue float64 // 亿元
}

func (c *ScreenCommand) fetchStocksByMarketCap(ctx context.Context, minCap float64, maxCount int) ([]stockInfo, error) {
	allStocks := make([]stockInfo, 0)
	seen := make(map[string]bool)

	// Use provider rank to get stocks first
	p := multi.New(eastmoney.New(), sina.New(), tencent.New())

	// Fetch high turnover stocks
	rankItems, err := p.Rank(ctx, "ab", "turnover", maxCount)
	if err == nil && len(rankItems) > 0 {
		for _, item := range rankItems {
			if seen[item.Symbol] {
				continue
			}
			seen[item.Symbol] = true
			allStocks = append(allStocks, stockInfo{
				Symbol:      item.Symbol,
				Name:        item.Name,
				Price:       item.Price,
				ChangePct:   item.ChangePct,
				MarketValue: item.MarketValue / 1e8, // Convert from 元 to 亿
			})
		}
	}

	// If we didn't get enough, try fetching more
	if len(allStocks) < maxCount {
		rankItems2, _ := p.Rank(ctx, "ab", "priceRatio", maxCount)
		for _, item := range rankItems2 {
			if seen[item.Symbol] {
				continue
			}
			seen[item.Symbol] = true
			allStocks = append(allStocks, stockInfo{
				Symbol:      item.Symbol,
				Name:        item.Name,
				Price:       item.Price,
				ChangePct:   item.ChangePct,
				MarketValue: item.MarketValue / 1e8, // Convert from 元 to 亿
			})
		}
	}

	// Fill in missing market caps using GetMarketCaps
	missingSymbols := make([]string, 0)
	for _, s := range allStocks {
		if s.MarketValue == 0 {
			missingSymbols = append(missingSymbols, s.Symbol)
		}
	}
	if len(missingSymbols) > 0 {
		// Try Tencent first (usually reliable)
		tc := tencent.New()
		marketCaps := tc.GetMarketCaps(ctx, missingSymbols)
		if len(marketCaps) > 0 {
			for i := range allStocks {
				if allStocks[i].MarketValue == 0 {
					if mv, ok := marketCaps[allStocks[i].Symbol]; ok {
						allStocks[i].MarketValue = mv / 1e8 // Convert from 元 to 亿
					}
				}
			}
		}
		// Fallback to Eastmoney if Tencent failed
		if len(missingSymbols) > 0 {
			em := eastmoney.New()
			marketCaps2 := em.GetMarketCaps(ctx, missingSymbols)
			for i := range allStocks {
				if allStocks[i].MarketValue == 0 {
					if mv, ok := marketCaps2[allStocks[i].Symbol]; ok {
						allStocks[i].MarketValue = mv / 1e8
					}
				}
			}
		}
	}

	// Sort by market cap descending
	sort.Slice(allStocks, func(i, j int) bool {
		return allStocks[i].MarketValue > allStocks[j].MarketValue
	})

	// Filter by minimum market cap
	filtered := make([]stockInfo, 0)
	for _, s := range allStocks {
		if s.MarketValue >= minCap {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) > maxCount {
		return filtered[:maxCount], nil
	}
	return filtered, nil
}

func (c *ScreenCommand) analyzeStocks(ctx context.Context, stocks []stockInfo, workers int, delay time.Duration) []ScreenResult {
	results := make([]ScreenResult, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, workers)
	p := multi.New(eastmoney.New(), sina.New(), tencent.New())

	// Use a ticker to pace requests globally across all workers
	throttle := time.NewTicker(delay)
	defer throttle.Stop()

	// Progress counter
	var processed int32
	total := len(stocks)
	progressTick := time.NewTicker(5 * time.Second)
	defer progressTick.Stop()

	// Progress reporter
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-progressTick.C:
				cur := atomic.LoadInt32(&processed)
				fmt.Printf("  分析进度: %d/%d (%.0f%%)\n", cur, total, float64(cur)/float64(total)*100)
			case <-done:
				return
			}
		}
	}()

	for _, s := range stocks {
		wg.Add(1)

		// Wait for throttle before launching next goroutine
		<-throttle.C
		sem <- struct{}{}

		go func(stock stockInfo) {
			defer wg.Done()
			defer func() { <-sem }()

			result := c.analyzeStock(ctx, p, stock)
			atomic.AddInt32(&processed, 1)
			if result != nil && result.Score >= 2 {
				mu.Lock()
				results = append(results, *result)
				mu.Unlock()
			}
		}(s)
	}

	wg.Wait()
	close(done)
	return results
}

func (c *ScreenCommand) analyzeStock(ctx context.Context, p provider.Provider, stock stockInfo) *ScreenResult {
	wk, err := p.KlineWeekly(ctx, stock.Symbol, 60)
	if err != nil || len(wk.Bars) < 20 {
		return nil
	}

	closes := make([]float64, len(wk.Bars))
	highs := make([]float64, len(wk.Bars))
	lows := make([]float64, len(wk.Bars))

	for i, b := range wk.Bars {
		closes[i] = b.Close
		highs[i] = b.High
		lows[i] = b.Low
	}

	// Calculate weekly MAs
	ma5 := indicator.MA(closes, 5)
	ma10 := indicator.MA(closes, 10)
	ma20 := indicator.MA(closes, 20)

	last := len(closes) - 1
	if ma5[last] == 0 || ma10[last] == 0 || ma20[last] == 0 {
		return nil
	}

	result := &ScreenResult{
		Symbol:      stock.Symbol,
		Name:        stock.Name,
		Price:       stock.Price,
		ChangePct:   stock.ChangePct,
		MarketValue: stock.MarketValue,
	}

	// Analyze weekly pattern
	pattern := c.detectWeeklyPattern(wk.Bars, closes, ma5, ma10, ma20)
	result.WeeklyPattern = pattern.Type
	result.PatternDesc = pattern.Desc
	result.Score = pattern.Score
	result.MAStatus = pattern.MAStatus
	result.TrendStatus = pattern.TrendStatus
	result.EntryZone = pattern.EntryZone
	result.RiskLevel = pattern.RiskLevel

	// Add volume analysis
	result.VolumeAnalysis = pattern.VolumeAnalysis
	result.VolumeRatio = fmt.Sprintf("%.2f", pattern.VolumeRatio)
	result.AmountTrend = pattern.AmountTrend

	// Add Chan theory analysis
	result.ChanLevel = pattern.ChanLevel
	result.ChanStatus = pattern.ChanStatus
	result.Chan中枢 = pattern.Chan中枢
	result.ChanBuyPoint = pattern.ChanBuyPoint
	result.ChanTrend = pattern.ChanTrend

	// Fetch daily K-line for buy point analysis
	dk, err := p.KlineDaily(ctx, stock.Symbol, 60)
	if err == nil && len(dk.Bars) >= 20 {
		daily := c.analyzeDailyBuyPoint(dk.Bars)
		result.DailyBuySignal = daily.Signal
		result.DailyDesc = daily.Desc
		result.DailyEntry = daily.Entry
		result.DailyStopLoss = daily.StopLoss
		result.DailySupport = daily.Support
	} else {
		result.DailyBuySignal = "待分析"
		result.DailyDesc = "日线数据获取失败"
		result.DailyEntry = "-"
		result.DailyStopLoss = "-"
		result.DailySupport = "-"
	}

	return result
}

// DailyBuyPoint represents daily-level buy point analysis
type DailyBuyPoint struct {
	Signal    string // 买点信号
	Desc      string // 描述
	Entry     string // 入场价
	StopLoss  string // 止损价
	Support   string // 支撑位
}

func (c *ScreenCommand) analyzeDailyBuyPoint(bars []provider.KlineBar) DailyBuyPoint {
	n := len(bars)
	if n < 20 {
		return DailyBuyPoint{Signal: "数据不足"}
	}

	last := n - 1
	secLast := n - 2
	thrLast := n - 3

	// Extract OHLCV
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	for i, b := range bars {
		closes[i] = b.Close
		highs[i] = b.High
		lows[i] = b.Low
	}

	curClose := closes[last]
	prevClose := closes[secLast]
	thrClose := closes[thrLast]

	// Calculate daily indicators
	ma5 := indicator.MA(closes, 5)
	ma10 := indicator.MA(closes, 10)
	ma20 := indicator.MA(closes, 20)
	ma60 := indicator.MA(closes, 60)

	macd := indicator.MACD(closes)
	kdj := indicator.KDJ(highs, lows, closes, 9)
	rsi := indicator.RSI(closes, 14)

	curMA5 := ma5[last]
	curMA10 := ma10[last]
	curMA20 := ma20[last]
	curMA60 := ma60[last]

	thrMA5 := ma5[thrLast]
	thrMA10 := ma10[thrLast]

	// Current indicator values
	curMACD := macd[last]
	prevMACD := macd[secLast]
	curKDJ := kdj[last]
	prevKDJ := kdj[secLast]
	curRSI := rsi[last]

	// Find recent support levels
	var recentLows []float64
	startIdx := n - 20
	if startIdx < 0 {
		startIdx = 0
	}
	for i := startIdx; i < n; i++ {
		recentLows = append(recentLows, lows[i])
	}
	minLow := recentLows[0]
	for _, l := range recentLows {
		if l < minLow {
			minLow = l
		}
	}

	// Calculate ATR for stop loss (currently not used but available)
	_ = indicator.ATR(highs, lows, closes, 14)

	var result DailyBuyPoint

	// === Buy Signal Detection ===

	// 1. MACD Golden Cross (日线MACD金叉)
	if prevMACD.DIF <= prevMACD.DEA && curMACD.DIF > curMACD.DEA {
		if curMACD.DIF > 0 && curMACD.MACD > 0 {
			// MACD金叉在水上（多头）
			stopLoss := curMA20 * 0.97
			result = DailyBuyPoint{
				Signal:   "MACD金叉",
				Desc:     fmt.Sprintf("MACD金叉( DIF>0 )，RSI=%.0f", curRSI),
				Entry:    fmt.Sprintf("%.2f", curClose),
				StopLoss: fmt.Sprintf("%.2f", stopLoss),
				Support:  fmt.Sprintf("%.2f", curMA20),
			}
			return result
		} else if curMACD.DIF > 0 {
			result = DailyBuyPoint{
				Signal:   "MACD金叉",
				Desc:     fmt.Sprintf("MACD零轴上方金叉，RSI=%.0f", curRSI),
				Entry:    fmt.Sprintf("%.2f", curClose),
				StopLoss: fmt.Sprintf("%.2f", curMA10*0.97),
				Support:  fmt.Sprintf("%.2f", curMA10),
			}
			return result
		}
	}

	// 2. KDJ Golden Cross at oversold level (KDJ低位金叉)
	if prevKDJ.K <= prevKDJ.D && curKDJ.K > curKDJ.D {
		if prevKDJ.K < 30 || prevKDJ.D < 30 {
			// KDJ低位金叉
			stopLoss := math.Min(lows[last]*0.97, curMA20*0.96)
			result = DailyBuyPoint{
				Signal:   "KDJ金叉",
				Desc:     fmt.Sprintf("KDJ低位金叉(K=%.0f,D=%.0f)，超跌反弹", curKDJ.K, curKDJ.D),
				Entry:    fmt.Sprintf("%.2f", curClose),
				StopLoss: fmt.Sprintf("%.2f", stopLoss),
				Support:  fmt.Sprintf("%.2f", curMA10),
			}
			return result
		}
	}

	// 3. Daily MA Golden Cross (日线均线金叉)
	if thrMA5 <= thrMA10 && curMA5 > curMA10 {
		stopLoss := curMA20 * 0.97
		result = DailyBuyPoint{
			Signal:   "均线金叉",
			Desc:     fmt.Sprintf("MA5上穿MA10，RSI=%.0f", curRSI),
			Entry:    fmt.Sprintf("%.2f", curClose),
			StopLoss: fmt.Sprintf("%.2f", stopLoss),
			Support:  fmt.Sprintf("%.2f", curMA10),
		}
		return result
	}

	// 4. Pullback to MA10 (回踩MA10)
	if curMA5 > curMA10 && curMA10 > curMA20 {
		tolerance := math.Max(curMA10*0.02, 0.5)
		if math.Abs(curClose-curMA10) <= tolerance {
			stopLoss := curMA20 * 0.97
			result = DailyBuyPoint{
				Signal:   "回踩MA10",
				Desc:     fmt.Sprintf("回踩MA10(%.2f)反弹，RSI=%.0f", curMA10, curRSI),
				Entry:    fmt.Sprintf("%.2f", curClose),
				StopLoss: fmt.Sprintf("%.2f", stopLoss),
				Support:  fmt.Sprintf("%.2f", curMA20),
			}
			return result
		}
	}

	// 5. Pullback to MA20 (回踩MA20)
	if curMA5 > curMA20 {
		tolerance := math.Max(curMA20*0.03, 0.5)
		if math.Abs(curClose-curMA20) <= tolerance {
			stopLoss := curMA60 * 0.97
			result = DailyBuyPoint{
				Signal:   "回踩MA20",
				Desc:     fmt.Sprintf("回踩MA20(%.2f)获得支撑，RSI=%.0f", curMA20, curRSI),
				Entry:    fmt.Sprintf("%.2f", curClose),
				StopLoss: fmt.Sprintf("%.2f", stopLoss),
				Support:  fmt.Sprintf("%.2f", curMA60),
			}
			return result
		}
	}

	// 6. Bounce from recent low (触底反弹)
	if curClose > prevClose && prevClose < thrClose {
		// 连续下跌后出现阳线
		reboundPct := (curClose - prevClose) / prevClose * 100
		if reboundPct > 1 {
			stopLoss := math.Min(prevClose*0.98, curClose*0.97)
			result = DailyBuyPoint{
				Signal:   "触底反弹",
				Desc:     fmt.Sprintf("触底反弹%.1f%%，RSI=%.0f超卖", reboundPct, curRSI),
				Entry:    fmt.Sprintf("%.2f", curClose),
				StopLoss: fmt.Sprintf("%.2f", stopLoss),
				Support:  fmt.Sprintf("%.2f", prevClose),
			}
			return result
		}
	}

	// 7. MACD divergence at bottom (MACD底部背离)
	if curMACD.DIF > prevMACD.DIF && curClose < prevClose {
		// 价格新低但MACD没有新低，底背离
		result = DailyBuyPoint{
			Signal:   "MACD底背离",
			Desc:     fmt.Sprintf("MACD底背离，RSI=%.0f", curRSI),
			Entry:    fmt.Sprintf("%.2f", curClose),
			StopLoss: fmt.Sprintf("%.2f", minLow*0.98),
			Support:  fmt.Sprintf("%.2f", minLow),
		}
		return result
	}

	// 8. Strong trend continuation (强势延续)
	if curMA5 > curMA10 && curMA10 > curMA20 {
		if curClose > curMA5 {
			stopLoss := curMA5 * 0.98
			result = DailyBuyPoint{
				Signal:   "强势延续",
				Desc:     fmt.Sprintf("均线多头排列，股价在MA5上方，RSI=%.0f", curRSI),
				Entry:    fmt.Sprintf("%.2f", curClose),
				StopLoss: fmt.Sprintf("%.2f", stopLoss),
				Support:  fmt.Sprintf("%.2f", curMA5),
			}
			return result
		}
	}

	// Default: observe
	stopLossDef := curClose * 0.95
	result = DailyBuyPoint{
		Signal:   "观望",
		Desc:     fmt.Sprintf("等待买点，RSI=%.0f", curRSI),
		Entry:    "-",
		StopLoss: fmt.Sprintf("%.2f", stopLossDef),
		Support:  fmt.Sprintf("%.2f", curMA10),
	}
	return result
}

type weeklyPattern struct {
	Type        string
	Desc        string
	Score       int
	MAStatus    string
	TrendStatus string
	EntryZone   string
	RiskLevel   string

	// 量价分析
	VolumeAnalysis string
	VolumeRatio   float64
	AmountTrend   string

	// 缠论分析
	ChanLevel    string
	ChanStatus   string
	Chan中枢      string
	ChanBuyPoint string
	ChanTrend    string
}

func (c *ScreenCommand) detectWeeklyPattern(bars []provider.KlineBar, closes, ma5, ma10, ma20 []float64) weeklyPattern {
	n := len(bars)
	if n < 20 {
		return weeklyPattern{Type: "数据不足", Score: 0}
	}

	last := n - 1
	secLast := n - 2

	// Current values
	curClose := bars[last].Close
	curMA5 := ma5[last]
	curMA10 := ma10[last]
	curMA20 := ma20[last]

	prevClose := bars[secLast].Close
	prevMA5 := ma5[secLast]
	prevMA10 := ma10[secLast]
	prevMA20 := ma20[secLast]

	// Check MA arrangement
	maRising := curMA5 > curMA10 && curMA10 > curMA20
	maFalling := curMA5 < curMA10 && curMA10 < curMA20

	// === 量价分析 ===
	volumes := make([]float64, n)
	amounts := make([]float64, n)
	for i, b := range bars {
		volumes[i] = b.Volume
		amounts[i] = b.Volume * b.Close
	}

	// Calculate average volume for last 5, 10, 20 weeks
	avgVol5 := 0.0
	avgVol10 := 0.0
	avgVol20 := 0.0
	for i := 0; i < 5 && i < n; i++ {
		avgVol5 += volumes[n-1-i]
	}
	avgVol5 /= float64(min(5, n))
	for i := 0; i < 10 && i < n; i++ {
		avgVol10 += volumes[n-1-i]
	}
	avgVol10 /= float64(min(10, n))
	for i := 0; i < 20 && i < n; i++ {
		avgVol20 += volumes[n-1-i]
	}
	avgVol20 /= float64(min(20, n))

	curVol := volumes[last]
	volRatio := 0.0
	if avgVol5 > 0 {
		volRatio = curVol / avgVol5
	}

	// Volume trend analysis
	volumeAnalysis := "量能正常"
	amountTrend := "成交额稳定"
	if volRatio > 1.5 {
		volumeAnalysis = "量能放大"
	} else if volRatio > 2.0 {
		volumeAnalysis = "量能大幅放大"
	} else if volRatio < 0.5 {
		volumeAnalysis = "量能萎缩"
	}

	// Amount trend
	avgAmount5 := 0.0
	avgAmount10 := 0.0
	for i := 0; i < 5 && i < n; i++ {
		avgAmount5 += amounts[n-1-i]
	}
	avgAmount5 /= float64(min(5, n))
	for i := 0; i < 10 && i < n; i++ {
		avgAmount10 += amounts[n-1-i]
	}
	avgAmount10 /= float64(min(10, n))
	if avgAmount5 > avgAmount10*1.2 {
		amountTrend = "成交额增加"
	} else if avgAmount5 < avgAmount10*0.8 {
		amountTrend = "成交额减少"
	}

	// === 缠论分析 ===
	// Simplified 中枢 (center) detection using recent highs and lows
	chanLevel := "周线级别"
	chanTrend := "待确认"
	chanStatus := "筑底中"
	chan中枢 := "-"
	chanBuyPoint := "-"

	// Find recent swing highs and lows for 中枢
	var swingHighs, swingLows []float64
	for i := 2; i < n-2 && i < 30; i++ {
		// Local high (not highest in absolute, but relative)
		if bars[i].High > bars[i-1].High && bars[i].High > bars[i-1].High && bars[i].High >= bars[i+1].High && bars[i].High >= bars[i+2].High {
			swingHighs = append(swingHighs, bars[i].High)
		}
		if bars[i].Low < bars[i-1].Low && bars[i].Low < bars[i-1].Low && bars[i].Low <= bars[i+1].Low && bars[i].Low <= bars[i+2].Low {
			swingLows = append(swingLows, bars[i].Low)
		}
	}

	// Calculate 中枢 based on overlapping highs and lows
	if len(swingHighs) >= 2 && len(swingLows) >= 2 {
		// Sort and find overlapping zone
		minHigh := 0.0
		maxLow := 0.0
		if len(swingHighs) >= 2 {
			minHigh = swingHighs[len(swingHighs)-2]
			for _, h := range swingHighs {
				if h < minHigh {
					minHigh = h
				}
			}
		}
		if len(swingLows) >= 2 {
			maxLow = swingLows[0]
			for _, l := range swingLows {
				if l > maxLow {
					maxLow = l
				}
			}
		}
		if minHigh > 0 && maxLow > 0 && minHigh > maxLow {
			chan中枢 = fmt.Sprintf("%.2f-%.2f", maxLow, minHigh)
		}
	}

	// Determine trend direction based on 中枢 and price position
	if curClose > curMA20 && maRising {
		chanTrend = "上涨趋势"
		chanStatus = "上升中"
	} else if curClose < curMA20 && maFalling {
		chanTrend = "下跌趋势"
		chanStatus = "下降中"
	} else if maRising {
		chanTrend = "震荡偏强"
		chanStatus = "震荡中"
	} else if maFalling {
		chanTrend = "震荡偏弱"
		chanStatus = "震荡中"
	} else {
		chanTrend = "震荡整理"
		chanStatus = "筑底中"
	}

	// Identify 缠论买点 based on pullback levels
	if maRising && curClose > curMA20 {
		// 回撤到均线附近形成买点
		if math.Abs(curClose-curMA20)/curMA20 < 0.05 {
			chanBuyPoint = "1买(MA20)"
		} else if math.Abs(curClose-curMA10)/curMA10 < 0.03 {
			chanBuyPoint = "2买(MA10)"
		} else if math.Abs(curClose-curMA5)/curMA5 < 0.02 {
			chanBuyPoint = "3买(MA5)"
		}
	} else if curClose > curMA20 && prevClose <= prevMA20 {
		chanBuyPoint = "类2买(突破)"
	} else if !maRising && curClose > curMA20 {
		chanBuyPoint = "类1买(反弹)"
	}

	// Calculate recent highs/lows for pullback analysis
	recentHighs := make([]float64, 0)
	recentLows := make([]float64, 0)
	startIdx := n - 12
	if startIdx < 0 {
		startIdx = 0
	}
	for i := startIdx; i < n; i++ {
		recentHighs = append(recentHighs, bars[i].High)
		recentLows = append(recentLows, bars[i].Low)
	}

	// Find recent high
	maxHigh := recentHighs[0]
	for _, h := range recentHighs {
		if h > maxHigh {
			maxHigh = h
		}
	}

	pullbackPct := 0.0
	if maxHigh > 0 {
		pullbackPct = (maxHigh - curClose) / maxHigh * 100
	}

	// Helper to populate common fields on pattern
	fillPattern := func(p weeklyPattern) weeklyPattern {
		p.VolumeAnalysis = volumeAnalysis
		p.VolumeRatio = volRatio
		p.AmountTrend = amountTrend
		p.ChanLevel = chanLevel
		p.ChanTrend = chanTrend
		p.ChanStatus = chanStatus
		p.Chan中枢 = chan中枢
		p.ChanBuyPoint = chanBuyPoint
		return p
	}

	// Pattern 1: Fresh breakout (周线突破) - 最重要
	// Price crosses above MA20, and MA is rising
	if curClose > curMA20 && prevClose <= prevMA20 {
		if maRising {
			return fillPattern(weeklyPattern{
				Type:        "周线突破",
				Desc:        fmt.Sprintf("价格突破MA20(%.2f)，MA5>MA10>MA20多头排列", curMA20),
				Score:       5,
				MAStatus:    "多头排列",
				TrendStatus: "上涨趋势",
				EntryZone:   fmt.Sprintf("%.2f-%.2f", curMA20*0.99, curClose*1.02),
				RiskLevel:   "中等",
			})
		} else if curMA5 > curMA20 && prevMA5 <= prevMA20 {
			return fillPattern(weeklyPattern{
				Type:        "周线反弹",
				Desc:        fmt.Sprintf("价格突破MA20(%.2f)，MA5上穿MA20", curMA20),
				Score:       4,
				MAStatus:    "均线发散",
				TrendStatus: "反弹中",
				EntryZone:   fmt.Sprintf("%.2f-%.2f", curMA20*0.98, curClose*1.02),
				RiskLevel:   "较高",
			})
		}
	}

	// Pattern 2: Pullback to MA10 (2买)
	if maRising && curClose > curMA20 {
		// Price near MA10
		ma10Tolerance := math.Max(curMA10*0.02, 0.5)
		if math.Abs(curClose-curMA10) <= ma10Tolerance {
			if pullbackPct > 5 && pullbackPct < 20 {
				return fillPattern(weeklyPattern{
					Type:        "周线2买",
					Desc:        fmt.Sprintf("回调至MA10(%.2f)附近，MA多头排列完好，回调%.1f%%", curMA10, pullbackPct),
					Score:       5,
					MAStatus:    "多头排列",
					TrendStatus: "上涨趋势-回调",
					EntryZone:   fmt.Sprintf("%.2f-%.2f", curMA10*0.99, curMA10*1.02),
					RiskLevel:   "较低",
				})
			}
		}
	}

	// Pattern 3: Pullback to MA20 (1买)
	if maRising && curClose > curMA20 {
		ma20Tolerance := math.Max(curMA20*0.03, 0.5)
		if math.Abs(curClose-curMA20) <= ma20Tolerance {
			if pullbackPct > 10 && pullbackPct < 30 {
				return fillPattern(weeklyPattern{
					Type:        "周线1买",
					Desc:        fmt.Sprintf("深度回调至MA20(%.2f)附近，MA多头排列完好，回调%.1f%%", curMA20, pullbackPct),
					Score:       4,
					MAStatus:    "多头排列",
					TrendStatus: "上涨趋势-深度回调",
					EntryZone:   fmt.Sprintf("%.2f-%.2f", curMA20*0.98, curMA20*1.02),
					RiskLevel:   "较低",
				})
			}
		}
	}

	// Pattern 4: Healthy pullback to MA5
	if maRising && curClose > curMA20 {
		ma5Tolerance := math.Max(curMA5*0.015, 0.3)
		if math.Abs(curClose-curMA5) <= ma5Tolerance {
			if pullbackPct > 3 && pullbackPct < 15 {
				return fillPattern(weeklyPattern{
					Type:        "健康回踩",
					Desc:        fmt.Sprintf("回踩MA5(%.2f)后反弹，均线支撑有效", curMA5),
					Score:       4,
					MAStatus:    "多头排列",
					TrendStatus: "上涨趋势-整理",
					EntryZone:   fmt.Sprintf("%.2f-%.2f", curMA5*0.98, curMA5*1.02),
					RiskLevel:   "中等",
				})
			}
		}
	}

	// Pattern 5: MA Golden Cross recently
	if prevMA5 <= prevMA10 && curMA5 > curMA10 {
		if curClose > curMA20 {
			return fillPattern(weeklyPattern{
				Type:        "均线金叉",
				Desc:        fmt.Sprintf("MA5上穿MA10形成金叉，突破MA20(%.2f)", curMA20),
				Score:       4,
				MAStatus:    "金叉形成",
				TrendStatus: "趋势启动",
				EntryZone:   fmt.Sprintf("%.2f-%.2f", curMA10*0.98, curMA20*1.03),
				RiskLevel:   "中等",
			})
		} else if curClose > curMA10 {
			return fillPattern(weeklyPattern{
				Type:        "均线金叉",
				Desc:        fmt.Sprintf("MA5上穿MA10形成金叉，接近MA20(%.2f)", curMA20),
				Score:       3,
				MAStatus:    "金叉形成",
				TrendStatus: "蓄势中",
				EntryZone:   fmt.Sprintf("%.2f-%.2f", curClose*0.97, curClose*1.03),
				RiskLevel:   "较高",
			})
		}
	}

	// Pattern 6: Uptrend in progress
	if maRising && curClose > curMA5 {
		return fillPattern(weeklyPattern{
			Type:        "上升趋势",
			Desc:        "均线多头排列，股价在MA5上方运行",
			Score:       3,
			MAStatus:    "多头排列",
			TrendStatus: "上涨趋势-延续",
			EntryZone:   fmt.Sprintf("%.2f-%.2f(MA5上方)", curMA5*0.98, curMA5*1.02),
			RiskLevel:   "中等",
		})
	}

	// Pattern 7: MA consolidation (均线粘合)
	if math.Abs(curMA5-curMA10)/curMA10 < 0.02 && math.Abs(curMA10-curMA20)/curMA20 < 0.03 {
		if curClose > curMA20 {
			return fillPattern(weeklyPattern{
				Type:        "均线粘合",
				Desc:        "均线粘合后开始发散，有望启动",
				Score:       3,
				MAStatus:    "粘合发散",
				TrendStatus: "蓄势待发",
				EntryZone:   fmt.Sprintf("%.2f-%.2f", curMA20*0.98, curMA20*1.05),
				RiskLevel:   "中等",
			})
		}
	}

	// Pattern 8: Near MA20 support
	if curMA5 > curMA20 && prevMA5 <= prevMA20 {
		// Just crossed MA20
		if math.Abs(curClose-curMA20)/curMA20 < 0.01 {
			return fillPattern(weeklyPattern{
				Type:        "突破确认",
				Desc:        "回踩MA20确认支撑有效",
				Score:       3,
				MAStatus:    "突破确认",
				TrendStatus: "趋势确立",
				EntryZone:   fmt.Sprintf("%.2f-%.2f", curMA20*0.98, curMA20*1.03),
				RiskLevel:   "中等",
			})
		}
	}

	// Default: Not matching criteria
	trendStatus := "待观察"
	if maRising {
		trendStatus = "上涨趋势"
	} else if maFalling {
		trendStatus = "下跌趋势"
	}

	return fillPattern(weeklyPattern{
		Type:        "不符合",
		Desc:        "未达到选股条件",
		Score:       0,
		MAStatus:    maStatusStr(curMA5, curMA10, curMA20),
		TrendStatus: trendStatus,
		EntryZone:   "-",
		RiskLevel:   "未知",
	})
}

func maStatusStr(ma5, ma10, ma20 float64) string {
	if ma5 > ma10 && ma10 > ma20 {
		return "多头排列"
	}
	if ma5 < ma10 && ma10 < ma20 {
		return "空头排列"
	}
	return "混乱排列"
}

func (c *ScreenCommand) printResults(out io.Writer, results []ScreenResult) {
	if len(results) == 0 {
		fmt.Fprintln(out, "未找到符合条件的股票")
		return
	}

	// === Part 1: Weekly analysis table ===
	fmt.Fprintln(out, "┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐")
	fmt.Fprintln(out, "│                                          周线级别分析（判断大趋势）                                                         │")
	fmt.Fprintln(out, "└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘")

	t := format.NewTable(out)
	t.Header("代码", "名称", "现价", "涨幅", "市值(亿)", "周线形态", "趋势状态", "入场区间", "评分")

	for _, r := range results {
		t.Row(
			r.Symbol,
			r.Name,
			fmt.Sprintf("%.2f", r.Price),
			fmt.Sprintf("%+.2f%%", r.ChangePct),
			fmt.Sprintf("%.0f", r.MarketValue),
			r.WeeklyPattern,
			r.TrendStatus,
			r.EntryZone,
			fmt.Sprintf("%d星", r.Score),
		)
	}
	t.Flush()

	// === Part 2: Daily analysis table ===
	fmt.Fprintln(out)
	fmt.Fprintln(out, "┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐")
	fmt.Fprintln(out, "│                                          日线级别分析（寻找买点）                                                         │")
	fmt.Fprintln(out, "└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘")

	t2 := format.NewTable(out)
	t2.Header("代码", "名称", "日线买点", "入场价", "止损位", "支撑位", "指标描述")

	for _, r := range results {
		t2.Row(
			r.Symbol,
			r.Name,
			r.DailyBuySignal,
			r.DailyEntry,
			r.DailyStopLoss,
			r.DailySupport,
			r.DailyDesc,
		)
	}
	t2.Flush()

	fmt.Fprintln(out)
	fmt.Fprintf(out, "共筛选出 %d 只符合条件的股票\n", len(results))
	fmt.Fprintln(out)

	// Summary
	fmt.Fprintln(out, "┌─────────────────────────────────────────────────────────────────────────┐")
	fmt.Fprintln(out, "│                              操作建议汇总                                     │")
	fmt.Fprintln(out, "└─────────────────────────────────────────────────────────────────────────┘")

	// Filter stocks with buy signals
	buySignals := make([]ScreenResult, 0)
	for _, r := range results {
		if r.DailyBuySignal != "观望" && r.DailyBuySignal != "待分析" {
			buySignals = append(buySignals, r)
		}
	}

	if len(buySignals) > 0 {
		for _, r := range buySignals {
			if r.DailyEntry != "-" {
				fmt.Printf("  【%s %s】%s信号 | 入场: %s | 止损: %s | 支撑: %s\n",
					r.Symbol, r.Name, r.DailyBuySignal, r.DailyEntry, r.DailyStopLoss, r.DailySupport)
			}
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "说明:")
	fmt.Fprintln(out, "  周线级别:")
	fmt.Fprintln(out, "    - 周线突破: 价格刚突破MA20，均线多头排列，是强势启动信号")
	fmt.Fprintln(out, "    - 周线1买: 回调至MA20附近，形成缠论第一类买点，安全边际高")
	fmt.Fprintln(out, "    - 周线2买: 回调至MA10附近，形成缠论第二类买点，性价比高")
	fmt.Fprintln(out, "    - 健康回踩: 回踩MA5后反弹，是最理想的入场时机")
	fmt.Fprintln(out, "    - 均线金叉: MA5上穿MA10，趋势开始转多")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  日线级别:")
	fmt.Fprintln(out, "    - MACD金叉: DIF上穿DEA，多头信号，可入场")
	fmt.Fprintln(out, "    - KDJ金叉: KDJ低位金叉，超跌反弹信号")
	fmt.Fprintln(out, "    - 均线金叉: MA5上穿MA10，短期趋势转多")
	fmt.Fprintln(out, "    - 回踩MA10/MA20: 回调均线获得支撑，是较好入场点")
	fmt.Fprintln(out, "    - 触底反弹: 连续下跌后出现阳线，可能见底")
	fmt.Fprintln(out, "    - MACD底背离: 价格新低但MACD未新低，底部信号")
	fmt.Fprintln(out, "    - 强势延续: 均线多头排列，持股待涨")
}
