package output

import (
	"bytes"
	"encoding/json"
	"io"
)

// JSONResponse is the stable envelope for successful JSON output.
type JSONResponse struct {
	OK   bool        `json:"ok"`
	Data interface{} `json:"data"`
	Meta Meta        `json:"meta"`
}

// jsonErrorResponse is the envelope for error JSON output.
type jsonErrorResponse struct {
	OK    bool      `json:"ok"`
	Error errorBody `json:"error"`
	Meta  Meta      `json:"meta"`
}

type errorBody struct {
	Code            string `json:"code"`
	Message         string `json:"message"`
	Device          string `json:"device"`
	SuggestedAction string `json:"suggested_action,omitempty"`
}

// RenderJSON writes data as a pretty-printed JSON envelope.
// Without Raw, known secret values are redacted (agent-safe default).
// With Raw + RawRenderable, full RouterOS maps are emitted including secrets.
// When the payload exceeds MaxBytes, rows are dropped and meta.truncated=true;
// if still over, output is hard-truncated with [OUTPUT TRUNCATED].
func RenderJSON(w io.Writer, data Renderable, meta Meta, opts Options) error {
	payload, records := buildJSONPayload(data, opts)
	// Keep caller Count when already set and matches; otherwise derive from payload.
	if meta.Count == 0 {
		meta.Count = len(records)
	}

	maxBytes := effectiveMaxBytes(opts)
	resp := JSONResponse{OK: true, Data: payload, Meta: meta}

	b, err := marshalJSON(resp)
	if err != nil {
		return err
	}
	if maxBytes <= 0 || len(b) <= maxBytes {
		_, err = w.Write(b)
		return err
	}

	// Shrink list payloads until under the cap, marking truncated.
	if shrinkable, ok := payload.([]map[string]string); ok && len(shrinkable) > 0 {
		lo, hi := 0, len(shrinkable)
		var best []byte
		for lo < hi {
			mid := (lo + hi + 1) / 2
			try := resp
			try.Meta.Truncated = true
			try.Meta.Count = mid
			try.Data = shrinkable[:mid]
			tb, mErr := marshalJSON(try)
			if mErr != nil {
				return mErr
			}
			if len(tb) <= maxBytes {
				best = tb
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		if best != nil {
			_, err = w.Write(best)
			return err
		}
		resp.Meta.Truncated = true
		resp.Meta.Count = 0
		resp.Data = []map[string]string{}
		b, err = marshalJSON(resp)
		if err != nil {
			return err
		}
		if len(b) <= maxBytes {
			_, err = w.Write(b)
			return err
		}
	}

	resp.Meta.Truncated = true
	b, err = marshalJSON(resp)
	if err != nil {
		return err
	}
	_, err = writeCapped(w, b, maxBytes)
	return err
}

func buildJSONPayload(data Renderable, opts Options) (payload interface{}, records []map[string]string) {
	if opts.Raw {
		if raw, ok := data.(RawRenderable); ok {
			recs := raw.RawRecords()
			return recs, recs
		}
	}

	headers := data.TableHeaders()
	rows := data.TableRows()
	records = make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		record := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(row) {
				record[h] = RedactValue(h, row[i])
			}
		}
		records = append(records, record)
	}
	return records, records
}

func marshalJSON(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderError writes an error envelope as pretty-printed JSON.
// meta.request_id (and truncated when set) appear under meta for all envelopes.
// Optional suggestedAction (first variadic arg) becomes error.suggested_action.
func RenderError(w io.Writer, code, message, device string, meta Meta, suggestedAction ...string) error {
	body := errorBody{
		Code:    code,
		Message: message,
		Device:  device,
	}
	if len(suggestedAction) > 0 {
		body.SuggestedAction = suggestedAction[0]
	}
	if meta.Device == "" {
		meta.Device = device
	}
	resp := jsonErrorResponse{
		OK:    false,
		Error: body,
		Meta:  meta,
	}

	b, err := marshalJSON(resp)
	if err != nil {
		return err
	}
	maxBytes := effectiveMaxBytes(Options{})
	_, err = writeCapped(w, b, maxBytes)
	return err
}

// RenderRawJSON writes an arbitrary payload inside the standard envelope.
// Without Raw, known secret fields nested in maps/slices are redacted (same
// agent-safe default as RenderJSON). Pass Options{Raw: true} (--raw) to keep
// secrets (e.g. WireGuard private-key in audit JSON).
func RenderRawJSON(w io.Writer, data interface{}, meta Meta, opts ...Options) error {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	payload := data
	if !o.Raw {
		payload = RedactPayload(data)
	}
	resp := JSONResponse{
		OK:   true,
		Data: payload,
		Meta: meta,
	}
	b, err := marshalJSON(resp)
	if err != nil {
		return err
	}
	maxBytes := effectiveMaxBytes(o)
	if maxBytes <= 0 || len(b) <= maxBytes {
		_, err = w.Write(b)
		return err
	}
	resp.Meta.Truncated = true
	b, err = marshalJSON(resp)
	if err != nil {
		return err
	}
	if len(b) <= maxBytes {
		_, err = w.Write(b)
		return err
	}
	_, err = writeCapped(w, b, maxBytes)
	return err
}
