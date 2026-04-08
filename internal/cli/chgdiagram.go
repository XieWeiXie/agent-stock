package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"agent-stock/internal/provider"
	"agent-stock/internal/provider/eastmoney"
	"agent-stock/internal/provider/multi"
	"agent-stock/internal/provider/sina"
)

type ChgDiagramCommand struct{}

func NewChgDiagramCommand() *ChgDiagramCommand { return &ChgDiagramCommand{} }

func (c *ChgDiagramCommand) Name() string     { return "chgdiagram" }
func (c *ChgDiagramCommand) Synopsis() string { return "全市场涨跌分布" }

func (c *ChgDiagramCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s chgdiagram [--market <market>]

Examples:
  %s chgdiagram
`, appName, appName)
}

func (c *ChgDiagramCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	fs := flag.NewFlagSet(appName+" "+c.Name(), flag.ContinueOnError)
	fs.SetOutput(errOut)
	market := fs.String("market", "ab", "市场类型 (ab)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	p := multi.New(eastmoney.New(), sina.New())
	d, err := p.MarketDistribution(ctx, provider.Market(*market))
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "全市场涨跌分布 (%s)\n\n", *market)
	fmt.Fprintf(out, "上涨: %d (涨停: %d)  下跌: %d (跌停: %d)  平盘: %d\n\n", d.Up, d.UpLimit, d.Down, d.DownLimit, d.Flat)

	printBar := func(label string, count int, total int) {
		const maxBarLen = 40
		barLen := 0
		if total > 0 {
			barLen = int(float64(count) / float64(total) * maxBarLen)
		}
		bar := ""
		for i := 0; i < barLen; i++ {
			bar += "■"
		}
		fmt.Fprintf(out, "%-10s [%-40s] %d\n", label, bar, count)
	}

	total := d.Up + d.Down + d.Flat
	printBar("> 9%", d.Up9, total)
	printBar("6% - 9%", d.Up6_9, total)
	printBar("3% - 6%", d.Up3_6, total)
	printBar("0% - 3%", d.Up0_3, total)
	printBar("0%", d.Flat, total)
	printBar("-3% - 0%", d.Down0_3, total)
	printBar("-6% - -3%", d.Down3_6, total)
	printBar("-9% - -6%", d.Down6_9, total)
	printBar("< -9%", d.Down9, total)

	return nil
}
