package winbox

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

// Winbox 4 Addresses.cdb magic (reverse-engineered; undocumented).
var cdbMagicV4 = []byte{0x0d, 0xf0, 0x1d, 0xc0}

// Field IDs observed in Winbox 4 connection records.
// Winbox 3 CDB uses overlapping IDs for some fields; fid 4 is Comment in v4
// and Name in v3 — scan fallback treats it as comment/name interchangeably.
const (
	cdbFIDName     = 0
	cdbFIDAddr     = 1
	cdbFIDLogin    = 2
	cdbFIDPassword = 3
	cdbFIDComment  = 4
	cdbFIDGroup    = 8
	cdbFIDV3Note   = 0x0b
)

// ParseCDB best-effort parses a Winbox Addresses.cdb file.
// Primary path: Winbox 4 (magic 0d f0 1d c0, sized M2 records).
// Fallback: scan for length-prefixed string fields with known field IDs.
// The format is undocumented; future Winbox releases may break this parser.
func ParseCDB(data []byte) ([]Entry, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty CDB file")
	}

	if len(data) >= 4 &&
		data[0] == cdbMagicV4[0] && data[1] == cdbMagicV4[1] &&
		data[2] == cdbMagicV4[2] && data[3] == cdbMagicV4[3] {
		entries, err := parseCDBV4(data)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			return nil, fmt.Errorf("CDB file contains no address entries")
		}
		return entries, nil
	}

	// Unknown magic / Winbox 3 CDB: degrade gracefully via field scan.
	entries := scanCDBFields(data)
	if len(entries) == 0 {
		return nil, fmt.Errorf("unrecognized or empty CDB format (undocumented Winbox address book)")
	}
	return entries, nil
}

func parseCDBV4(data []byte) ([]Entry, error) {
	off := 4
	var entries []Entry
	for off < len(data) {
		if off+4 > len(data) {
			break
		}
		size := int(binary.LittleEndian.Uint32(data[off:]))
		off += 4
		if size < 6 || off+size > len(data) {
			// Truncated / corrupt — stop rather than panic.
			break
		}
		body := data[off : off+size]
		off += size
		if body[0] != 'M' || body[1] != '2' {
			continue
		}
		fields := parseCDBRecordFields(body)
		e := entryFromCDBFields(fields)
		if e.Address == "" && e.Username == "" && e.Comment == "" && e.Name == "" {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

type cdbField struct {
	fid   int
	value []byte
}

func parseCDBRecordFields(body []byte) []cdbField {
	var fields []cdbField
	i := 6 // past "M2" + 4-byte kind
	n := len(body)
	for i < n {
		if body[i] != 0x21 || i+2 > n {
			i++
			continue
		}
		ln := int(body[i+1])
		if i+2+ln > n {
			i++
			continue
		}
		if i < 3 {
			i++
			continue
		}
		prev3 := body[i-3 : i]
		fid := -1
		if prev3[0] == 0x09 && prev3[1] == 0x00 && prev3[2] == 0xfe {
			fid = cdbFIDName
		} else if prev3[1] == 0x00 && prev3[2] == 0x00 {
			fid = int(prev3[0])
		}
		if fid < 0 {
			i++
			continue
		}
		dataOff := i + 2
		fields = append(fields, cdbField{fid: fid, value: append([]byte(nil), body[dataOff:dataOff+ln]...)})
		i = dataOff + ln
	}
	return fields
}

func entryFromCDBFields(fields []cdbField) Entry {
	get := func(id int) string {
		for _, f := range fields {
			if f.fid == id {
				return decodeCDBString(f.value)
			}
		}
		return ""
	}
	e := Entry{
		Name:     get(cdbFIDName),
		Address:  get(cdbFIDAddr),
		Username: get(cdbFIDLogin),
		Password: get(cdbFIDPassword),
		Comment:  get(cdbFIDComment),
		Group:    get(cdbFIDGroup),
	}
	if e.Name == "" {
		e.Name = firstNonEmpty(e.Comment, e.Group, e.Address)
	}
	e.Address = NormalizeAddress(e.Address)
	return e
}

// scanCDBFields walks the whole file looking for
// <fid> 00 00 21 <len> <bytes> patterns and groups them into entries
// whenever an address field is seen after a gap.
func scanCDBFields(data []byte) []Entry {
	type hit struct {
		off int
		fid int
		val string
	}
	var hits []hit
	for i := 0; i+5 < len(data); i++ {
		if data[i+1] != 0x00 || data[i+2] != 0x00 || data[i+3] != 0x21 {
			continue
		}
		ln := int(data[i+4])
		if ln == 0 || i+5+ln > len(data) {
			continue
		}
		fid := int(data[i])
		switch fid {
		case cdbFIDName, cdbFIDAddr, cdbFIDLogin, cdbFIDPassword, cdbFIDComment, cdbFIDGroup, cdbFIDV3Note:
			val := decodeCDBString(data[i+5 : i+5+ln])
			if val == "" {
				continue
			}
			hits = append(hits, hit{off: i, fid: fid, val: val})
			i += 4 + ln
		}
	}
	if len(hits) == 0 {
		return nil
	}

	var entries []Entry
	var cur Entry
	has := false
	flush := func() {
		if !has {
			return
		}
		if cur.Address != "" || cur.Username != "" {
			if cur.Name == "" {
				cur.Name = firstNonEmpty(cur.Comment, cur.Group, cur.Address)
			}
			cur.Address = NormalizeAddress(cur.Address)
			entries = append(entries, cur)
		}
		cur = Entry{}
		has = false
	}

	lastOff := -1
	for _, h := range hits {
		if lastOff >= 0 && h.off-lastOff > 256 {
			flush()
		}
		lastOff = h.off
		has = true
		switch h.fid {
		case cdbFIDName:
			cur.Name = h.val
		case cdbFIDAddr:
			if cur.Address != "" {
				flush()
				has = true
			}
			cur.Address = h.val
		case cdbFIDLogin:
			cur.Username = h.val
		case cdbFIDPassword:
			cur.Password = h.val
		case cdbFIDComment:
			cur.Comment = h.val
			if cur.Name == "" {
				cur.Name = h.val
			}
		case cdbFIDV3Note:
			cur.Comment = h.val
		case cdbFIDGroup:
			cur.Group = h.val
		}
	}
	flush()
	return entries
}

func decodeCDBString(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	// Best-effort CP1251-ish: map bytes as latin-1 codepoints.
	out := make([]rune, len(b))
	for i, c := range b {
		out[i] = rune(c)
	}
	return string(out)
}
