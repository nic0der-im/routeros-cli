package winbox

import (
	"fmt"
	"unicode/utf8"
)

// WBX signature used by Winbox 3 address books.
var wbxSignature = []byte{0x0f, 0x10, 0xc0, 0xbe}

// ParseWBX parses a Winbox 3 addresses.WBX file (TLV records).
// Passwords may be stored in plaintext under the "pwd" field.
func ParseWBX(data []byte) ([]Entry, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty WBX file")
	}
	body := data
	if len(data) >= 4 && data[0] == wbxSignature[0] && data[1] == wbxSignature[1] &&
		data[2] == wbxSignature[2] && data[3] == wbxSignature[3] {
		body = data[4:]
	} else if len(data) < 4 {
		return nil, fmt.Errorf("WBX file too short")
	}

	var entries []Entry
	var cur map[string]string
	i := 0
	n := len(body)

	flush := func() {
		if cur == nil {
			return
		}
		e := entryFromWBXFields(cur)
		if e.Address != "" || e.Username != "" || e.Name != "" || e.Comment != "" {
			entries = append(entries, e)
		}
		cur = nil
	}

	for i < n {
		if i+1 < n && body[i] == 0x00 && body[i+1] == 0x00 {
			flush()
			i += 2
			continue
		}
		end, key, val, ok := tryWBXTLV(body, i, n)
		if ok {
			if cur == nil {
				cur = make(map[string]string)
			}
			ks := decodeWBXString(key)
			if ks != "" && ks != "type" {
				cur[ks] = decodeWBXString(val)
			}
			i = end
			continue
		}
		i++
	}
	flush()

	if len(entries) == 0 && len(data) > 0 {
		// Distinguish corrupt vs empty address book.
		if len(body) <= 2 {
			return nil, fmt.Errorf("WBX file contains no address entries")
		}
	}
	return entries, nil
}

func entryFromWBXFields(f map[string]string) Entry {
	e := Entry{
		Address:  f["host"],
		Username: f["login"],
		Password: f["pwd"],
		Comment:  firstNonEmpty(f["note"], f["comment"]),
		Group:    f["group"],
		Name:     firstNonEmpty(f["note"], f["group"], f["host"]),
	}
	e.Address = NormalizeAddress(e.Address)
	return e
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func decodeWBXString(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	out := make([]rune, len(b))
	for i, c := range b {
		out[i] = rune(c)
	}
	return string(out)
}

func tryWBXTLV(b []byte, i, n int) (end int, key, val []byte, ok bool) {
	for _, probe := range []func([]byte, int, int) (int, []byte, []byte, bool){
		tryWBXL2, tryWBXL1, tryWBXL0,
	} {
		if e, k, v, good := probe(b, i, n); good {
			return e, k, v, true
		}
	}
	return 0, nil, nil, false
}

// L2: u16_le total (from after u16 through end), then klen, key, val.
func tryWBXL2(b []byte, i, n int) (int, []byte, []byte, bool) {
	if i+3 > n {
		return 0, nil, nil, false
	}
	total := int(b[i]) | (int(b[i+1]) << 8)
	klen := int(b[i+2])
	end := (i + 2) + total
	if end > n || klen == 0 || klen > total-1 {
		return 0, nil, nil, false
	}
	start := i + 3
	key := b[start : start+klen]
	val := b[start+klen : end]
	if !plausibleWBXKey(key) {
		return 0, nil, nil, false
	}
	return end, key, val, true
}

// L1: total, 0x00, klen, key, val  (classic Winbox layout family).
func tryWBXL1(b []byte, i, n int) (int, []byte, []byte, bool) {
	if i+3 > n {
		return 0, nil, nil, false
	}
	total := int(b[i])
	if b[i+1] != 0x00 || i+1+total > n {
		return 0, nil, nil, false
	}
	klen := int(b[i+2])
	start := i + 3
	end := i + 1 + total
	if klen == 0 || klen > end-start {
		return 0, nil, nil, false
	}
	key := b[start : start+klen]
	val := b[start+klen : end]
	if !plausibleWBXKey(key) {
		return 0, nil, nil, false
	}
	return end, key, val, true
}

// L0: total, klen, key, val.
func tryWBXL0(b []byte, i, n int) (int, []byte, []byte, bool) {
	if i+2 > n {
		return 0, nil, nil, false
	}
	total := int(b[i])
	end := i + 1 + total
	if end > n {
		return 0, nil, nil, false
	}
	klen := int(b[i+1])
	if klen == 0 || klen > total-1 {
		return 0, nil, nil, false
	}
	start := i + 2
	key := b[start : start+klen]
	val := b[start+klen : end]
	if !plausibleWBXKey(key) {
		return 0, nil, nil, false
	}
	return end, key, val, true
}

func plausibleWBXKey(key []byte) bool {
	if len(key) == 0 || len(key) > 32 {
		return false
	}
	for _, c := range key {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}
