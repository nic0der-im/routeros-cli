package output

import (
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
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Device  string `json:"device"`
}

// RenderJSON writes data as a pretty-printed JSON envelope.
func RenderJSON(w io.Writer, data Renderable, meta Meta, opts Options) error {
	var payload interface{}

	if opts.Raw {
		if raw, ok := data.(RawRenderable); ok {
			payload = raw.RawRecords()
		}
	}

	if payload == nil {
		headers := data.TableHeaders()
		rows := data.TableRows()

		records := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			record := make(map[string]string, len(headers))
			for i, h := range headers {
				if i < len(row) {
					record[h] = row[i]
				}
			}
			records = append(records, record)
		}
		payload = records
	}

	resp := JSONResponse{
		OK:   true,
		Data: payload,
		Meta: meta,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

// RenderError writes an error envelope as pretty-printed JSON.
func RenderError(w io.Writer, code, message, device string) error {
	resp := jsonErrorResponse{
		OK: false,
		Error: errorBody{
			Code:    code,
			Message: message,
			Device:  device,
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

// RenderRawJSON writes an arbitrary payload inside the standard envelope.
func RenderRawJSON(w io.Writer, data interface{}, meta Meta) error {
	resp := JSONResponse{
		OK:   true,
		Data: data,
		Meta: meta,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}
