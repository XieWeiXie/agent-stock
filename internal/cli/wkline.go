package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"

	"agent-stock/internal/format"
	"agent-stock/internal/indicator"
	"agent-stock/internal/provider"
	"agent-stock/internal/provider/eastmoney"
	"agent-stock/internal/provider/multi"
	"agent-stock/internal/provider/sina"
)

type WKlineCommand struct{}

func NewWKlineCommand() *WKlineCommand { return &WKlineCommand{} }

func (c *WKlineCommand) Name() string     { return "wkline" }
func (c *WKlineCommand) Synopsis() string { return "周K数据及技术指标（MA5/MA10/MA20）" }

func (c *WKlineCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s wkline [--limit N] [--tail N] <symbol>

Examples:
  %s wkline 002463
  %s wkline --limit 100 --tail 30 300476
`, appName, appName, appName)
}

func (c *WKlineCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	fs := flag.NewFlagSet(appName+" "+c.Name(), flag.ContinueOnError)
	fs.SetOutput(errOut)
	limit := fs.Int("limit", 60, "fetch bars limit")
	tail := fs.Int("tail", 25, "print last N bars")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprint(errOut, c.Usage())
		return nil
	}

	symbol := rest[0]
	p := multi.New(eastmoney.New(), sina.New())
	kl, err := p.KlineWeekly(ctx, symbol, *limit)
	if err != nil {
		return fmt.Errorf("获取周线数据失败: %w", err)
	}
	if len(kl.Bars) == 0 {
		return fmt.Errorf("no weekly kline data")
	}

	closes := make([]float64, 0, len(kl.Bars))
	for _, b := range kl.Bars {
		closes = append(closes, b.Close)
	}

	// Calculate weekly MAs
	ma5 := indicator.MA(closes, 5)
	ma10 := indicator.MA(closes, 10)
	ma20 := indicator.MA(closes, 20)

	start := 0
	if *tail > 0 && len(kl.Bars) > *tail {
		start = len(kl.Bars) - *tail
	}

	// Analyze current pattern
	pattern := analyzeWeeklyPattern(kl.Bars, closes, ma5, ma10, ma20)
	fmt.Fprintf(out, "\n=== %s (%s) 周线分析 ===\n\n", kl.Name, symbol)
	fmt.Fprintf(out, "周线形态: %s\n", pattern)

	t := format.NewTable(out)
	t.Header("DATE(日期)", "O(开)", "H(高)", "L(低)", "C(收)", "VOL(量)", "MA5", "MA10", "MA20", "MA状态")
	for i := start; i < len(kl.Bars); i++ {
		b := kl.Bars[i]
		maStatus := ""
		if ma5[i] > 0 && ma10[i] > 0 && ma20[i] > 0 {
			if ma5[i] > ma10[i] && ma10[i] > ma20[i] {
				maStatus = "多头"
			} else if ma5[i] < ma10[i] && ma10[i] < ma20[i] {
				maStatus = "空头"
			} else {
				maStatus = "混乱"
			}
		}
		t.Row(
			b.Date,
			f2(b.Open),
			f2(b.High),
			f2(b.Low),
			f2(b.Close),
			f0(b.Volume),
			f2(ma5[i]),
			f2(ma10[i]),
			f2(ma20[i]),
			maStatus,
		)
	}
	return t.Flush()
}

func analyzeWeeklyPattern(bars []provider.KlineBar, closes []float64, ma5, ma10, ma20 []float64) string {
	n := len(closes)
	if n < 5 {
		return "数据不足"
	}

	last := n - 1
	secLast := n - 2

	curClose := closes[last]
	curMA5 := ma5[last]
	curMA10 := ma10[last]
	curMA20 := ma20[last]

	prevClose := closes[secLast]
	prevMA5 := ma5[secLast]
	prevMA10 := ma10[secLast]
	prevMA20 := ma20[secLast]

	// MA arrangement
	maRising := curMA5 > curMA10 && curMA10 > curMA20

	// Pattern detection
	if curClose > curMA20 && prevClose <= prevMA20 {
		if maRising {
			return "周线突破（强势启动）"
		} else {
			return "周线反弹"
		}
	}

	if maRising && curClose > curMA20 {
		tolerance := math.Max(curMA10*0.02, 0.5)
		if math.Abs(curClose-curMA10) <= tolerance {
			return "周线2买（回踩MA10）"
		}
		tolerance20 := math.Max(curMA20*0.03, 0.5)
		if math.Abs(curClose-curMA20) <= tolerance20 {
			return "周线1买（回踩MA20）"
		}
		tolerance5 := math.Max(curMA5*0.015, 0.3)
		if math.Abs(curClose-curMA5) <= tolerance5 {
			return "健康回踩"
		}
	}

	if prevMA5 <= prevMA10 && curMA5 > curMA10 {
		if curClose > curMA20 {
			return "均线金叉（趋势启动）"
		}
	}

	if maRising && curClose > curMA5 {
		return "上升趋势延续"
	}

	if math.Abs(curMA5-curMA10)/curMA10 < 0.02 && math.Abs(curMA10-curMA20)/curMA20 < 0.03 {
		if curClose > curMA20 {
			return "均线粘合蓄势"
		}
	}

	return "待观察"
}
