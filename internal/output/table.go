package output

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// RenderTable writes data as a human-readable table using text/tabwriter.
// Headers are printed in ALL CAPS. Columns are tab-separated with minwidth=0,
// tabwidth=4, and padding=2. Known secret columns are always redacted.
// When MaxBytes is exceeded, output is hard-truncated with [OUTPUT TRUNCATED].
func RenderTable(w io.Writer, data Renderable, opts ...Options) error {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}

	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)

	headers := data.TableHeaders()
	upper := make([]string, len(headers))
	for i, h := range headers {
		upper[i] = strings.ToUpper(h)
	}
	if _, err := fmt.Fprintln(tw, strings.Join(upper, "\t")); err != nil {
		return err
	}

	for _, row := range data.TableRows() {
		redacted := make([]string, len(row))
		for i, cell := range row {
			key := ""
			if i < len(headers) {
				key = headers[i]
			}
			redacted[i] = RedactValue(key, cell)
		}
		if _, err := fmt.Fprintln(tw, strings.Join(redacted, "\t")); err != nil {
			return err
		}
	}

	if err := tw.Flush(); err != nil {
		return err
	}

	_, err := writeCapped(w, buf.Bytes(), effectiveMaxBytes(o))
	return err
}
