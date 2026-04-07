package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"agent-stock/internal/format"
	"agent-stock/internal/provider/eastmoney"
)

type SearchCommand struct{}

func NewSearchCommand() *SearchCommand { return &SearchCommand{} }

func (c *SearchCommand) Name() string     { return "search" }
func (c *SearchCommand) Synopsis() string { return "股票搜索" }

func (c *SearchCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s search [--market ab|hk|us] <keyword>

Examples:
  %s search 腾讯
  %s search --market ab 000001
`, appName, appName, appName)
}

func (c *SearchCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	fs := flag.NewFlagSet(appName+" "+c.Name(), flag.ContinueOnError)
	fs.SetOutput(errOut)
	marketFlag := fs.String("market", "ab", "market: ab|hk|us")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprint(errOut, c.Usage())
		return nil
	}

	market, err := parseMarket(*marketFlag)
	if err != nil {
		return err
	}

	p := eastmoney.New()
	items, err := p.Search(ctx, rest[0], market)
	if err != nil {
		return err
	}

	t := format.NewTable(out)
	t.Header("SYMBOL", "NAME", "MARKET")
	for _, it := range items {
		t.Row(it.Symbol, it.Name, string(it.Market))
	}
	return t.Flush()
}
