package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"agent-stock/internal/format"
	"agent-stock/internal/indicator"
	"agent-stock/internal/provider/eastmoney"
	"agent-stock/internal/provider/multi"
	"agent-stock/internal/provider/sina"
)

type KlineCommand struct{}

func NewKlineCommand() *KlineCommand { return &KlineCommand{} }

func (c *KlineCommand) Name() string     { return "kline" }
func (c *KlineCommand) Synopsis() string { return "日K数据以及技术指标（EMA/BOLL/KDJ/RSI）" }

func (c *KlineCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s kline [--limit N] [--tail N] <symbol>

Examples:
  %s kline 000001
  %s kline --limit 200 --tail 30 600519
`, appName, appName, appName)
}

func (c *KlineCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	fs := flag.NewFlagSet(appName+" "+c.Name(), flag.ContinueOnError)
	fs.SetOutput(errOut)
	limit := fs.Int("limit", 180, "fetch bars limit")
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
	kl, err := p.KlineDaily(ctx, symbol, *limit)
	if err != nil {
		return err
	}
	if len(kl.Bars) == 0 {
		return fmt.Errorf("no kline data")
	}

	closep := make([]float64, 0, len(kl.Bars))
	high := make([]float64, 0, len(kl.Bars))
	low := make([]float64, 0, len(kl.Bars))
	for _, b := range kl.Bars {
		closep = append(closep, b.Close)
		high = append(high, b.High)
		low = append(low, b.Low)
	}

	ema12 := indicator.EMA(closep, 12)
	ema26 := indicator.EMA(closep, 26)
	boll := indicator.BOLL(closep, 20, 2)
	kdj := indicator.KDJ(high, low, closep, 9)
	rsi6 := indicator.RSI(closep, 6)
	rsi12 := indicator.RSI(closep, 12)
	rsi24 := indicator.RSI(closep, 24)

	start := 0
	if *tail > 0 && len(kl.Bars) > *tail {
		start = len(kl.Bars) - *tail
	}

	t := format.NewTable(out)
	t.Header("DATE(日期)", "O(开)", "H(高)", "L(低)", "C(收)", "VOL(量)", "EMA12(12日)", "EMA26(26日)", "BOLL(M/U/L)", "KDJ(K/D/J)", "RSI(6/12/24)")
	for i := start; i < len(kl.Bars); i++ {
		b := kl.Bars[i]
		t.Row(
			b.Date,
			f2(b.Open),
			f2(b.High),
			f2(b.Low),
			f2(b.Close),
			f0(b.Volume),
			f2(ema12[i]),
			f2(ema26[i]),
			fmt.Sprintf("%s/%s/%s", f2(boll[i].Mid), f2(boll[i].Upper), f2(boll[i].Lower)),
			fmt.Sprintf("%s/%s/%s", f2(kdj[i].K), f2(kdj[i].D), f2(kdj[i].J)),
			fmt.Sprintf("%s/%s/%s", f2(rsi6[i]), f2(rsi12[i]), f2(rsi24[i])),
		)
	}
	return t.Flush()
}
