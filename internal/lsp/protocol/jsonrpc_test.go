package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func idp(id ID) *ID { return &id }

func TestDecodeMessage(t *testing.T) {
	for _, test := range []struct {
		in   string
		want Message
	}{
		{`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"a":1}}`,
			&Request{ID: idp(IntID(1)), Method: "initialize", Params: json.RawMessage(`{"a":1}`)}},
		{`{"jsonrpc":"2.0","id":"x7","method":"m","params":[1]}`,
			&Request{ID: idp(StringID("x7")), Method: "m", Params: json.RawMessage(`[1]`)}},
		{`{"jsonrpc":"2.0","id":-5,"method":"shutdown"}`,
			&Request{ID: idp(IntID(-5)), Method: "shutdown"}},
		{`{"jsonrpc":"2.0","id":2,"method":"shutdown","params":null}`,
			&Request{ID: idp(IntID(2)), Method: "shutdown"}},
		{`{"method":"exit","jsonrpc":"2.0"}`, &Request{Method: "exit"}},
		{` {"jsonrpc":"2.0","id":3,"result":null} `,
			&Response{ID: idp(IntID(3)), Result: json.RawMessage("null")}},
		{`{"jsonrpc":"2.0","id":"a","error":{"code":-32601,"message":"no"}}`,
			&Response{ID: idp(StringID("a")), Error: &ResponseError{Code: -32601, Message: "no"}}},
		{`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"bad","data":[1]}}`,
			&Response{Error: &ResponseError{Code: -32700, Message: "bad", Data: json.RawMessage("[1]")}}},
	} {
		got, err := DecodeMessage([]byte(test.in))
		if err != nil {
			t.Errorf("DecodeMessage(%s): %v", test.in, err)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("DecodeMessage(%s) = %+v, want %+v", test.in, got, test.want)
		}
	}
}

func TestDecodeMessageErrors(t *testing.T) {
	for _, test := range []struct {
		in   string
		code ErrorCodes
		id   *ID
	}{
		{``, ParseError, nil},
		{`{"jsonrpc":"2.0","id":1,"method":`, ParseError, nil},
		{`{"jsonrpc":"2.0", "id": 1, "method": "m"} x`, ParseError, nil},
		{`[{"jsonrpc":"2.0","id":1,"method":"m"}]`, InvalidRequest, nil}, // a batch
		{`"hello"`, InvalidRequest, nil},
		{`{"jsonrpc":"2.0","id":1,"method":7}`, InvalidRequest, nil},
		{`{"jsonrpc":"2.0","id":null,"method":"m"}`, InvalidRequest, nil},
		{`{"jsonrpc":"2.0","id":1.5,"method":"m"}`, InvalidRequest, nil},
		{`{"jsonrpc":"2.0","id":{},"method":"m"}`, InvalidRequest, nil},
		{`{"id":4,"method":"m"}`, InvalidRequest, idp(IntID(4))},
		{`{"jsonrpc":"1.0","id":"s","method":"m"}`, InvalidRequest, idp(StringID("s"))},
		{`{"jsonrpc":"2.0","id":5,"method":"m","params":"text"}`, InvalidRequest, idp(IntID(5))},
		{`{"jsonrpc":"2.0","id":5,"method":"m","params":3}`, InvalidRequest, idp(IntID(5))},
		{`{"jsonrpc":"2.0","id":6,"method":"m","result":1}`, InvalidRequest, idp(IntID(6))},
		{`{"jsonrpc":"2.0","id":7}`, InvalidRequest, idp(IntID(7))},
		{`{"jsonrpc":"2.0","id":8,"result":1,"error":{"code":1,"message":""}}`, InvalidRequest, idp(IntID(8))},
		{`{"jsonrpc":"2.0","result":1}`, InvalidRequest, nil},
		{`{"jsonrpc":"2.0","id":null,"result":1}`, InvalidRequest, nil},
	} {
		_, err := DecodeMessage([]byte(test.in))
		var de *DecodeError
		if !errors.As(err, &de) {
			t.Errorf("DecodeMessage(%s) = %v, want a DecodeError", test.in, err)
			continue
		}
		if de.Code != test.code || !reflect.DeepEqual(de.ID, test.id) {
			t.Errorf("DecodeMessage(%s): code %d, id %v; want code %d, id %v (%v)",
				test.in, de.Code, de.ID, test.code, test.id, err)
		}
	}
}

func TestEncodeMessage(t *testing.T) {
	for _, test := range []struct {
		m    Message
		want string
	}{
		{&Request{ID: idp(IntID(1)), Method: "m", Params: json.RawMessage(`{"a": 1}`)},
			`{"jsonrpc":"2.0","id":1,"method":"m","params":{"a":1}}`},
		{&Request{Method: "n"}, `{"jsonrpc":"2.0","method":"n"}`},
		{&Response{ID: idp(StringID(`q"`))}, `{"jsonrpc":"2.0","id":"q\"","result":null}`},
		{&Response{ID: idp(IntID(2)), Result: json.RawMessage(`[1, 2]`)}, `{"jsonrpc":"2.0","id":2,"result":[1,2]}`},
		{&Response{Error: Errorf(ParseError, "bad %s", "json")},
			`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"bad json"}}`},
		{&Response{ID: idp(IntID(3)), Error: Errorf(RequestCancelled, "cancelled")},
			`{"jsonrpc":"2.0","id":3,"error":{"code":-32800,"message":"cancelled"}}`},
	} {
		got, err := EncodeMessage(test.m)
		if err != nil || string(got) != test.want {
			t.Errorf("EncodeMessage(%+v) = %s, %v; want %s", test.m, got, err, test.want)
		}
		// What is encoded decodes to the same message (but a nil result
		// becomes null).
		if back, err := DecodeMessage(got); err != nil {
			t.Errorf("DecodeMessage(%s): %v", got, err)
		} else if again, _ := EncodeMessage(back); string(again) != string(got) {
			t.Errorf("%s decodes and encodes to %s", got, again)
		}
	}
}

func TestIDString(t *testing.T) {
	if s := IntID(42).String(); s != "42" {
		t.Errorf("IntID(42).String() = %s", s)
	}
	if s := StringID("a b").String(); s != `"a b"` {
		t.Errorf(`StringID("a b").String() = %s`, s)
	}
	// IDs are map keys: 1 and "1" are different requests.
	m := map[ID]int{IntID(1): 1, StringID("1"): 2}
	if len(m) != 2 || m[IntID(1)] != 1 || m[StringID("1")] != 2 {
		t.Errorf("1 and \"1\" are the same map key: %v", m)
	}
}

// TestEncodeTrustedResponse: a response whose result the connection marked
// as valid, compact JSON is put together to the bytes that the general
// encoder gives for the same response.
func TestEncodeTrustedResponse(t *testing.T) {
	id := ID{}
	for _, raw := range []string{`null`, `[]`, `{"a":[1,2,{"b":"<&>"}]}`, `"x"`} {
		for _, rid := range []*ID{&id, nil} {
			plain := &Response{ID: rid, Result: json.RawMessage(raw)}
			fast := &Response{ID: rid, Result: json.RawMessage(raw), trusted: true}
			want, err := EncodeMessage(plain)
			if err != nil {
				t.Fatal(err)
			}
			got, err := EncodeMessage(fast)
			if err != nil {
				t.Fatal(err)
			}
			// The general encoder escapes < > & in a raw result; the trusted
			// result came from encoding/json, which escaped them already.
			if raw != `{"a":[1,2,{"b":"<&>"}]}` && string(got) != string(want) {
				t.Errorf("result %s, id %v:\n got %s\nwant %s", raw, rid, got, want)
			}
			var back map[string]json.RawMessage
			if err := json.Unmarshal(got, &back); err != nil || string(back["result"]) != raw || string(back["jsonrpc"]) != `"2.0"` {
				t.Errorf("trusted response %s does not decode back: %v", got, err)
			}
		}
	}
}
