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

type NewsCommand struct{}

func NewNewsCommand() *NewsCommand { return &NewsCommand{} }

func (c *NewsCommand) Name() string     { return "news" }
func (c *NewsCommand) Synopsis() string { return "个股相关资讯" }

func (c *NewsCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s news <symbol> [--count <count>]

Examples:
  %s news 000001
  %s news 000001 --count 5
`, appName, appName, appName)
}

func (c *NewsCommand) Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	fs := flag.NewFlagSet(appName+" "+c.Name(), flag.ContinueOnError)
	fs.SetOutput(errOut)
	count := fs.Int("count", 10, "显示新闻数量")
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
	news, err := p.News(ctx, symbol, *count)
	if err != nil {
		return err
	}

	if len(news) == 0 {
		fmt.Fprintln(out, "暂无相关资讯")
		return nil
	}

	t := format.NewTable(out)
	t.Header("时间", "标题", "链接")
	for _, it := range news {
		t.Row(it.Time, it.Title, it.Url)
	}
	return t.Flush()
}
