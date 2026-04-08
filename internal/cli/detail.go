package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"agent-stock/internal/format"
	"agent-stock/internal/provider/eastmoney"
	"agent-stock/internal/provider/multi"
	"agent-stock/internal/provider/sina"
)

type DetailCommand struct{}

func NewDetailCommand() *DetailCommand { return &DetailCommand{} }

func (c *DetailCommand) Name() string     { return "detail" }
func (c *DetailCommand) Synopsis() string { return "个股综合详情" }

func (c *DetailCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s detail <symbol>

Examples:
  %s detail 000001
`, appName, appName)
}

func (c *DetailCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	fs := flag.NewFlagSet(appName+" "+c.Name(), flag.ContinueOnError)
	fs.SetOutput(errOut)
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

	// 1. Get Quote
	quotes, err := p.Quote(ctx, []string{symbol})
	if err != nil {
		return err
	}
	if len(quotes) == 0 {
		return fmt.Errorf("stock not found: %s", symbol)
	}
	q := quotes[0]

	fmt.Fprintf(out, "=== %s (%s) ===\n", q.Name, q.Symbol)
	fmt.Fprintf(out, "当前价格: %s (%s%%)  成交量: %s  成交额: %s\n", f2(q.Price), f2(q.ChangePct), f0(q.Volume), f0(q.Amount))
	fmt.Fprintf(out, "最高: %s  最低: %s  开盘: %s  昨收: %s\n\n", f2(q.High), f2(q.Low), f2(q.Open), f2(q.PrevClose))

	// 2. Get FundFlow
	ff, err := p.FundFlow(ctx, symbol)
	if err == nil {
		fmt.Fprintln(out, "--- 资金流向 ---")
		t := format.NewTable(out)
		t.Header("主力净流入", "超大单", "大单", "占比")
		t.Row(f0(ff.MainIn), f0(ff.HugeIn), f0(ff.LargeIn), f2(ff.MainPct)+"%")
		t.Flush()
		fmt.Fprintln(out)
	}

	// 3. Get Plates
	plates, err := p.Plate(ctx, symbol)
	if err == nil && len(plates) > 0 {
		fmt.Fprintln(out, "--- 所属板块 ---")
		t := format.NewTable(out)
		t.Header("名称", "涨跌幅")
		for i, p := range plates {
			if i >= 5 {
				break
			}
			t.Row(p.Name, f2(p.ChangePct)+"%")
		}
		t.Flush()
	}

	return nil
}
