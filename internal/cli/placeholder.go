package cli

import (
	"context"
	"fmt"
	"io"
)

type PlaceholderCommand struct {
	name     string
	synopsis string
}

func NewPlaceholderCommand(name string, synopsis string) *PlaceholderCommand {
	return &PlaceholderCommand{name: name, synopsis: synopsis}
}

func (c *PlaceholderCommand) Name() string     { return c.name }
func (c *PlaceholderCommand) Synopsis() string { return c.synopsis }

func (c *PlaceholderCommand) Usage() string {
	return fmt.Sprintf(`Usage:
  %s %s

Note:
  该子命令在 Go 版中暂未实现。
`, appName, c.name)
}

func (c *PlaceholderCommand) Run(_ context.Context, _ []string, _ io.Writer, errOut io.Writer) error {
	fmt.Fprintln(errOut, "not implemented in Go replica yet")
	return nil
}
