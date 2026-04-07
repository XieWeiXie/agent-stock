package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"agent-stock/internal/format"
	"agent-stock/internal/provider/eastmoney"
)

type IndexCommand struct{}

func NewIndexCommand() *IndexCommand { return &IndexCommand{} }

func (c *IndexCommand) Name() string     { return "index" }
func (c *IndexCommand) Synopsis() string { return "大盘主要指数总览" }

func (c *IndexCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s index [--market ab|hk|us]

Examples:
  %s index
  %s index --market ab
`, appName, appName, appName)
}

func (c *IndexCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	fs := flag.NewFlagSet(appName+" "+c.Name(), flag.ContinueOnError)
	fs.SetOutput(errOut)
	marketFlag := fs.String("market", "ab", "market: ab|hk|us")
	if err := fs.Parse(args); err != nil {
		return err
	}

	market, err := parseMarket(*marketFlag)
	if err != nil {
		return err
	}

	p := eastmoney.New()
	items, err := p.Index(ctx, market)
	if err != nil {
		return err
	}

	t := format.NewTable(out)
	t.Header("SYMBOL", "NAME", "PRICE", "CHG", "CHG%")
	for _, q := range items {
		t.Row(q.Symbol, q.Name, f2(q.Price), f2(q.Change), f2(q.ChangePct))
	}
	return t.Flush()
}
