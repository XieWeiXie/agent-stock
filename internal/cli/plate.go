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

type PlateCommand struct{}

func NewPlateCommand() *PlateCommand { return &PlateCommand{} }

func (c *PlateCommand) Name() string     { return "plate" }
func (c *PlateCommand) Synopsis() string { return "个股所属板块" }

func (c *PlateCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s plate <symbol>

Examples:
  %s plate 000001
`, appName, appName)
}

func (c *PlateCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
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
	plates, err := p.Plate(ctx, symbol)
	if err != nil {
		return err
	}

	if len(plates) == 0 {
		fmt.Fprintln(out, "未找到所属板块")
		return nil
	}

	t := format.NewTable(out)
	t.Header("代码", "名称", "涨跌幅")
	for _, it := range plates {
		t.Row(it.Symbol, it.Name, f2(it.ChangePct)+"%")
	}
	return t.Flush()
}
