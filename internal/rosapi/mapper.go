// Package rosapi provides typed mapping of RouterOS API response sentences
// to Go structs and implements the Renderable interface for output formatting.
package rosapi

import (
	"fmt"
	"reflect"
	"strconv"
)

// MapSentences converts a slice of RouterOS API response sentences
// (each a map[string]string) into a slice of typed Go structs. Fields are
// matched using the `ros:"field_name"` struct tag. Fields without a ros tag
// are skipped. Missing keys in the sentence map produce zero values.
func MapSentences[T any](sentences []map[string]string) ([]T, error) {
	results := make([]T, 0, len(sentences))

	for i, sentence := range sentences {
		var t T
		rv := reflect.ValueOf(&t).Elem()
		rt := rv.Type()

		for j := 0; j < rt.NumField(); j++ {
			field := rt.Field(j)
			tag := field.Tag.Get("ros")
			if tag == "" {
				continue
			}

			val, ok := sentence[tag]
			if !ok {
				// Missing key: keep zero value.
				continue
			}

			fv := rv.Field(j)
			if err := setField(fv, val); err != nil {
				return nil, fmt.Errorf("sentence %d, field %q (tag %q): %w", i, field.Name, tag, err)
			}
		}

		results = append(results, t)
	}

	return results, nil
}

// setField assigns a string value from a RouterOS sentence to a reflect.Value,
// handling type conversions for string, bool, int, and int64 fields.
func setField(fv reflect.Value, val string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)

	case reflect.Bool:
		switch val {
		case "true", "yes":
			fv.SetBool(true)
		case "false", "no", "":
			fv.SetBool(false)
		default:
			return fmt.Errorf("cannot parse %q as bool", val)
		}

	case reflect.Int, reflect.Int64:
		if val == "" {
			fv.SetInt(0)
			return nil
		}
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as int: %w", val, err)
		}
		fv.SetInt(n)

	default:
		return fmt.Errorf("unsupported field kind %s", fv.Kind())
	}

	return nil
}
