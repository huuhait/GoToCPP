package protocol

import (
	"encoding/json"
	"strconv"
)

// A LocationList is a list of locations that encodes itself: the JSON of
// []Location, made by hand, without the reflection and the copies of
// encoding/json. A list of references of a popular name has thousands of
// them, and the encoding of the answer is a good part of its cost.
type LocationList []Location

// MarshalJSON implements json.Marshaler; the result is what json.Marshal
// gives for the same list as a []Location.
func (l LocationList) MarshalJSON() ([]byte, error) { return l.compactJSON(), nil }

// compactJSON is the encoding, which is valid and compact (the connection
// sends it as it is, see trustedEncoder).
func (l LocationList) compactJSON() []byte {
	if l == nil {
		return []byte("[]")
	}
	// The size of the answer, so that the buffer is not grown (and copied) as it is filled: the
	// fixed text of a location is 84 bytes, and the four numbers take about 4 digits each.
	size := 2
	for _, loc := range l {
		size += len(loc.URI) + 100
	}
	b := make([]byte, 0, size)
	b = append(b, '[')
	for i, loc := range l {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, `{"uri":`...)
		b = appendJSONString(b, string(loc.URI))
		b = append(b, `,"range":{"start":{"line":`...)
		b = strconv.AppendUint(b, uint64(loc.Range.Start.Line), 10)
		b = append(b, `,"character":`...)
		b = strconv.AppendUint(b, uint64(loc.Range.Start.Character), 10)
		b = append(b, `},"end":{"line":`...)
		b = strconv.AppendUint(b, uint64(loc.Range.End.Line), 10)
		b = append(b, `,"character":`...)
		b = strconv.AppendUint(b, uint64(loc.Range.End.Character), 10)
		b = append(b, `}}}`...)
	}
	return append(b, ']')
}

// appendJSONString appends s as a JSON string: as it is between quotes when
// it needs no escape (the paths of a workspace nearly all), else by
// encoding/json.
func appendJSONString(b []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c >= 0x7f || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&' {
			enc, err := json.Marshal(s)
			if err != nil {
				return append(b, `""`...)
			}
			return append(b, enc...)
		}
	}
	b = append(b, '"')
	b = append(b, s...)
	return append(b, '"')
}

// A trustedEncoder is a result that encodes itself to valid, compact JSON,
// so the connection sends it without the checks that json.Marshal makes of
// a Marshaler's output.
type trustedEncoder interface{ compactJSON() []byte }
