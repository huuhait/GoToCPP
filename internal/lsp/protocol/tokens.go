package protocol

import "strconv"

// An EncodedTokens is the answer of a request for the semantic tokens of a whole file, encoded once
// when the tokens are made: a client that scrolls, or opens the file again, asks for the same
// tokens, and a long file has thousands of numbers that encoding/json would write each time.
type EncodedTokens struct {
	json []byte
}

// EncodeTokens returns the answer for the tokens data (five numbers each, see SemanticTokens). Its
// JSON is what json.Marshal gives for SemanticTokens{Data: data}.
func EncodeTokens(data []uint32) *EncodedTokens {
	if data == nil {
		return &EncodedTokens{json: []byte(`{"data":null}`)}
	}
	b := make([]byte, 0, len(`{"data":[]}`)+len(data)*5)
	b = append(b, `{"data":[`...)
	for i, n := range data {
		if i > 0 {
			b = append(b, ',')
		}
		b = strconv.AppendUint(b, uint64(n), 10)
	}
	return &EncodedTokens{json: append(b, `]}`...)}
}

// MarshalJSON implements json.Marshaler.
func (t *EncodedTokens) MarshalJSON() ([]byte, error) { return t.json, nil }

// compactJSON is the encoding, valid and compact (the connection sends it as it is, see
// trustedEncoder). The bytes are shared by every answer made from t: they are never changed.
func (t *EncodedTokens) compactJSON() []byte { return t.json }
