package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"agent-stock/internal/format"
	"agent-stock/internal/provider/eastmoney"
	"agent-stock/internal/provider/multi"
	"agent-stock/internal/provider/sina"
)

type QuoteCommand struct{}

func NewQuoteCommand() *QuoteCommand { return &QuoteCommand{} }

func (c *QuoteCommand) Name() string     { return "quote" }
func (c *QuoteCommand) Synopsis() string { return "个股实时行情（支持批量）" }

func (c *QuoteCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s quote <symbols>

Notes:
  - symbols: 单个或多个股票代码，用逗号分隔，如 000001,600519

Examples:
  %s quote 000001
  %s quote 000001,600519
`, appName, appName, appName)
}

func (c *QuoteCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
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

	symbols := strings.Split(rest[0], ",")
	p := multi.New(eastmoney.New(), sina.New())
	quotes, err := p.Quote(ctx, symbols)
	if err != nil {
		return err
	}

	t := format.NewTable(out)
	t.Header("SYMBOL(股票代码)", "NAME(名称)", "PRICE(价格)", "CHG(涨跌额)", "CHG%(涨跌幅)", "OPEN(开)", "HIGH(高)", "LOW(低)", "VOL(量)", "AMT(额)", "TIME(时间)")
	for _, q := range quotes {
		t.Row(q.Symbol, q.Name, f2(q.Price), f2(q.Change), f2(q.ChangePct), f2(q.Open), f2(q.High), f2(q.Low), f0(q.Volume), f0(q.Amount), q.Time)
	}
	return t.Flush()
}
