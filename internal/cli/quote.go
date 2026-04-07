package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"agent-stock/internal/format"
	"agent-stock/internal/provider/eastmoney"
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
	p := eastmoney.New()
	quotes, err := p.Quote(ctx, symbols)
	if err != nil {
		return err
	}

	t := format.NewTable(out)
	t.Header("SYMBOL", "NAME", "PRICE", "CHG", "CHG%", "OPEN", "HIGH", "LOW", "VOL", "AMT", "TIME")
	for _, q := range quotes {
		t.Row(q.Symbol, q.Name, f2(q.Price), f2(q.Change), f2(q.ChangePct), f2(q.Open), f2(q.High), f2(q.Low), f0(q.Volume), f0(q.Amount), q.Time)
	}
	return t.Flush()
}

func f2(v float64) string { return fmt.Sprintf("%.2f", v) }
func f0(v float64) string { return fmt.Sprintf("%.0f", v) }
