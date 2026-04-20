package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

const appName = "stock"

var (
	version   = "dev"
	commitSHA = "unknown"
)

type Command interface {
	Name() string
	Synopsis() string
	Usage() string
	Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error
}

func Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	rootFlags := flag.NewFlagSet(appName, flag.ContinueOnError)
	rootFlags.SetOutput(io.Discard)
	help := rootFlags.Bool("help", false, "")
	rootFlags.BoolVar(help, "h", false, "")
	ver := rootFlags.Bool("version", false, "")
	rootFlags.BoolVar(ver, "v", false, "")

	if err := rootFlags.Parse(args); err != nil {
		return formatFlagError(err, rootFlags, errOut)
	}

	rest := rootFlags.Args()
	if *help || len(rest) == 0 {
		printRootUsage(out)
		return nil
	}
	if *ver {
		fmt.Fprintf(out, "%s %s (%s)\n", appName, version, commitSHA)
		return nil
	}

	cmdName := rest[0]
	cmdArgs := rest[1:]

	cmds := availableCommands()
	cmd, ok := cmds[cmdName]
	if !ok {
		printRootUsage(errOut)
		return fmt.Errorf("unknown command: %s", cmdName)
	}
	if len(cmdArgs) > 0 && (cmdArgs[0] == "-h" || cmdArgs[0] == "--help") {
		fmt.Fprint(out, cmd.Usage())
		return nil
	}
	return cmd.Run(ctx, cmdArgs, out, errOut)
}

func availableCommands() map[string]Command {
	cmds := []Command{
		NewSearchCommand(),
		NewQuoteCommand(),
		NewRankCommand(),
		NewIndexCommand(),
		NewKlineCommand(),
		NewWKlineCommand(),
		NewDetailCommand(),
		NewNewsCommand(),
		NewFundFlowCommand(),
		NewPlateCommand(),
		NewChgDiagramCommand(),
		NewScreenCommand(),
		NewPlaceholderCommand("heatmap", "板块热力图（占位）"),
		NewPlaceholderCommand("query", "条件选股（占位）"),
	}
	m := make(map[string]Command, len(cmds))
	for _, c := range cmds {
		m[c.Name()] = c
	}
	return m
}

func printRootUsage(w io.Writer) {
	cmds := availableCommands()
	desired := []string{
		"chgdiagram",
		"detail",
		"fundflow",
		"heatmap",
		"index",
		"kline",
		"news",
		"plate",
		"query",
		"quote",
		"rank",
		"screen",
		"search",
		"wkline",
	}
	// Build final ordered list: first desired order, then any remaining alphabetically
	seen := make(map[string]bool, len(cmds))
	ordered := make([]string, 0, len(cmds))
	for _, name := range desired {
		if _, ok := cmds[name]; ok {
			ordered = append(ordered, name)
			seen[name] = true
		}
	}
	rest := make([]string, 0, len(cmds))
	for name := range cmds {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	ordered = append(ordered, rest...)

	fmt.Fprintf(w, "%s - 面向 AI Agent 的股市数据命令行工具（Go 复刻版）\n\n", appName)
	fmt.Fprintf(w, "Usage:\n  %s [--help|-h] [--version|-v]\n  %s <command> [args]\n\n", appName, appName)
	fmt.Fprintf(w, "Commands:\n")
	for _, name := range ordered {
		c := cmds[name]
		fmt.Fprintf(w, "  %-10s %s\n", c.Name(), c.Synopsis())
	}
	fmt.Fprintf(w, "\nRun \"%s <command> --help\" for more information on a command.\n", appName)
}

func formatFlagError(err error, fs *flag.FlagSet, errOut io.Writer) error {
	if errors.Is(err, flag.ErrHelp) {
		printRootUsage(errOut)
		return nil
	}
	msg := err.Error()
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return err
	}
	return fmt.Errorf("%s", msg)
}
