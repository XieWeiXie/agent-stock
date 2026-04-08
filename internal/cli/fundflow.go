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

type FundFlowCommand struct{}

func NewFundFlowCommand() *FundFlowCommand { return &FundFlowCommand{} }

func (c *FundFlowCommand) Name() string     { return "fundflow" }
func (c *FundFlowCommand) Synopsis() string { return "个股资金流向" }

func (c *FundFlowCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s fundflow <symbol>

Examples:
  %s fundflow 000001
`, appName, appName)
}

func (c *FundFlowCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
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
	ff, err := p.FundFlow(ctx, symbol)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "股票: %s (%s)\n\n", ff.Name, ff.Symbol)

	t := format.NewTable(out)
	t.Header("指标", "金额(元)", "占比")
	t.Row("主力净流入", f0(ff.MainIn), f2(ff.MainPct)+"%")
	t.Row("超大单净流入", f0(ff.HugeIn), "-")
	t.Row("大单净流入", f0(ff.LargeIn), "-")
	t.Row("中单净流入", f0(ff.MediumIn), "-")
	t.Row("小单净流入", f0(ff.SmallIn), "-")
	return t.Flush()
}
