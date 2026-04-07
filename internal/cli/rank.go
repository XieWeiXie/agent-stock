package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"agent-stock/internal/format"
	"agent-stock/internal/provider/eastmoney"
)

type RankCommand struct{}

func NewRankCommand() *RankCommand { return &RankCommand{} }

func (c *RankCommand) Name() string     { return "rank" }
func (c *RankCommand) Synopsis() string { return "市场股票排序（限A股）" }

func (c *RankCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s rank [--market ab|hk|us] [--sort turnover|amplitude|volumeRatio|exchange|priceRatio] [--count 1-100]

Examples:
  %s rank --count 20
  %s rank --sort amplitude --count 50
`, appName, appName, appName)
}

func (c *RankCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	fs := flag.NewFlagSet(appName+" "+c.Name(), flag.ContinueOnError)
	fs.SetOutput(errOut)
	marketFlag := fs.String("market", "ab", "market: ab|hk|us")
	sortFlag := fs.String("sort", "turnover", "sort: turnover|amplitude|volumeRatio|exchange|priceRatio")
	countFlag := fs.Int("count", 20, "count: 1-100")
	if err := fs.Parse(args); err != nil {
		return err
	}

	market, err := parseMarket(*marketFlag)
	if err != nil {
		return err
	}

	p := eastmoney.New()
	items, err := p.Rank(ctx, market, *sortFlag, *countFlag)
	if err != nil {
		return err
	}

	t := format.NewTable(out)
	t.Header("SYMBOL", "NAME", "PRICE", "CHG%", "AMOUNT", "TURN%")
	for _, it := range items {
		t.Row(it.Symbol, it.Name, f2(it.Price), f2(it.ChangePct), f0(it.Amount), f2(it.Turnover))
	}
	return t.Flush()
}
