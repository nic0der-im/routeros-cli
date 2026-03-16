package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// RenderTable writes data as a human-readable table using text/tabwriter.
// Headers are printed in ALL CAPS. Columns are tab-separated with minwidth=0,
// tabwidth=4, and padding=2.
func RenderTable(w io.Writer, data Renderable) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	headers := data.TableHeaders()
	upper := make([]string, len(headers))
	for i, h := range headers {
		upper[i] = strings.ToUpper(h)
	}
	if _, err := fmt.Fprintln(tw, strings.Join(upper, "\t")); err != nil {
		return err
	}

	for _, row := range data.TableRows() {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}

	return tw.Flush()
}
