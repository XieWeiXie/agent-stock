package format

import (
	"fmt"
	"io"
	"text/tabwriter"
)

type Table struct {
	tw *tabwriter.Writer
}

func NewTable(w io.Writer) *Table {
	return &Table{
		tw: tabwriter.NewWriter(w, 0, 0, 2, ' ', 0),
	}
}

func (t *Table) Header(cols ...string) {
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(t.tw, "\t")
		}
		fmt.Fprint(t.tw, c)
	}
	fmt.Fprint(t.tw, "\n")
}

func (t *Table) Row(cols ...any) {
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(t.tw, "\t")
		}
		fmt.Fprint(t.tw, c)
	}
	fmt.Fprint(t.tw, "\n")
}

func (t *Table) Flush() error {
	return t.tw.Flush()
}
