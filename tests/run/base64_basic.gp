// phase: 09
// encoding/base64: the four standard encodings, padding, line breaks, strict mode, streaming Encoder and Decoder, error values and texts, sizes.

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

func main() {
	inputs := []string{"", "f", "fo", "foo", "foob", "fooba", "foobar", "\x00\xff\xfe\xfb\xff", "Hello, 世界!", strings.Repeat("0123456789", 12)}
	encs := []struct {
		name string
		e    *base64.Encoding
	}{
		{"std", base64.StdEncoding},
		{"url", base64.URLEncoding},
		{"rawstd", base64.RawStdEncoding},
		{"rawurl", base64.RawURLEncoding},
	}
	for _, in := range inputs {
		for _, e := range encs {
			s := e.e.EncodeToString([]byte(in))
			back, err := e.e.DecodeString(s)
			if len(s) > 40 {
				s = s[:40] + "..."
			}
			fmt.Printf("%-6s %-24q -> %-44s back ok=%v err=%v\n", e.name, in[:min(len(in), 20)], s, string(back) == in, err)
		}
	}
	fmt.Println(base64.StdEncoding.EncodedLen(10), base64.RawStdEncoding.EncodedLen(10), base64.StdEncoding.DecodedLen(16), base64.RawStdEncoding.DecodedLen(14))

	// Errors.
	for _, in := range []string{"Zm9v!", "Zm9", "Zm9vYg", "Zm9vYg=", "Zm9vYg===", "=Zm9", "Zm=v", "Zm9v\nYmFy", "Zm9v\r\nYmFy\n", "Zg==Zg==", "Zh==", "Zm9vYmE=\n"} {
		b, err := base64.StdEncoding.DecodeString(in)
		var ce base64.CorruptInputError
		fmt.Printf("%-14q -> %q %v corrupt=%v\n", in, b, err, errors.As(err, &ce))
	}
	b, err := base64.StdEncoding.Strict().DecodeString("Zh==")
	fmt.Printf("strict: %q %v\n", b, err)
	b, err = base64.StdEncoding.Strict().DecodeString("Zg==")
	fmt.Printf("strict ok: %q %v\n", b, err)
	b, err = base64.URLEncoding.DecodeString("-_-_")
	fmt.Printf("url: %v %v\n", b, err)
	b, err = base64.StdEncoding.DecodeString("+/+/")
	fmt.Printf("std: %v %v\n", b, err)
	b, err = base64.StdEncoding.DecodeString("-_-_")
	fmt.Printf("std with url chars: %v %v\n", b, err)

	// A custom alphabet and padding.
	custom := base64.NewEncoding("ZYXWVUTSRQPONMLKJIHGFEDCBAzyxwvutsrqponmlkjihgfedcba9876543210+/").WithPadding('*')
	s := custom.EncodeToString([]byte("custom alphabet"))
	d, err := custom.DecodeString(s)
	fmt.Println(s, string(d), err)
	fmt.Println(base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte("ab")))

	// AppendEncode, AppendDecode, Encode, Decode.
	dst := []byte("prefix:")
	dst = base64.StdEncoding.AppendEncode(dst, []byte("hello"))
	fmt.Printf("%s\n", dst)
	dst, err = base64.StdEncoding.AppendDecode([]byte("decoded:"), []byte("aGVsbG8="))
	fmt.Printf("%s %v\n", dst, err)
	buf := make([]byte, base64.StdEncoding.EncodedLen(5))
	base64.StdEncoding.Encode(buf, []byte("12345"))
	out := make([]byte, base64.StdEncoding.DecodedLen(len(buf)))
	n, err := base64.StdEncoding.Decode(out, buf)
	fmt.Printf("%s %d %q %v\n", buf, n, out[:n], err)

	// Streaming.
	var sb bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &sb)
	for _, part := range []string{"The quick ", "brown fox ", "jumps over ", "the lazy dog"} {
		enc.Write([]byte(part))
	}
	fmt.Println(enc.Close(), sb.String())
	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(sb.String()))
	all, err := io.ReadAll(dec)
	fmt.Printf("%q %v\n", all, err)
	dec = base64.NewDecoder(base64.StdEncoding, strings.NewReader("Zm9v\nYmFy\r\nYg=="))
	all, err = io.ReadAll(dec)
	fmt.Printf("%q %v\n", all, err)
	dec = base64.NewDecoder(base64.StdEncoding, strings.NewReader("Zm9v!mFy"))
	all, err = io.ReadAll(dec)
	fmt.Printf("%q %v\n", all, err)
	dec = base64.NewDecoder(base64.RawStdEncoding, iotestOneByte{strings.NewReader("Zm9vYmFyYg")})
	all, err = io.ReadAll(dec)
	fmt.Printf("%q %v\n", all, err)
	var long bytes.Buffer
	e2 := base64.NewEncoder(base64.URLEncoding, &long)
	e2.Write(bytes.Repeat([]byte{0xfb, 0xff, 0x01}, 10000))
	e2.Close()
	fmt.Println(long.Len(), long.String()[:24])
}

type iotestOneByte struct{ r io.Reader }

func (o iotestOneByte) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return o.r.Read(p[:1])
}

// Output:
// std    ""                       ->                                              back ok=true err=<nil>
// url    ""                       ->                                              back ok=true err=<nil>
// rawstd ""                       ->                                              back ok=true err=<nil>
// rawurl ""                       ->                                              back ok=true err=<nil>
// std    "f"                      -> Zg==                                         back ok=true err=<nil>
// url    "f"                      -> Zg==                                         back ok=true err=<nil>
// rawstd "f"                      -> Zg                                           back ok=true err=<nil>
// rawurl "f"                      -> Zg                                           back ok=true err=<nil>
// std    "fo"                     -> Zm8=                                         back ok=true err=<nil>
// url    "fo"                     -> Zm8=                                         back ok=true err=<nil>
// rawstd "fo"                     -> Zm8                                          back ok=true err=<nil>
// rawurl "fo"                     -> Zm8                                          back ok=true err=<nil>
// std    "foo"                    -> Zm9v                                         back ok=true err=<nil>
// url    "foo"                    -> Zm9v                                         back ok=true err=<nil>
// rawstd "foo"                    -> Zm9v                                         back ok=true err=<nil>
// rawurl "foo"                    -> Zm9v                                         back ok=true err=<nil>
// std    "foob"                   -> Zm9vYg==                                     back ok=true err=<nil>
// url    "foob"                   -> Zm9vYg==                                     back ok=true err=<nil>
// rawstd "foob"                   -> Zm9vYg                                       back ok=true err=<nil>
// rawurl "foob"                   -> Zm9vYg                                       back ok=true err=<nil>
// std    "fooba"                  -> Zm9vYmE=                                     back ok=true err=<nil>
// url    "fooba"                  -> Zm9vYmE=                                     back ok=true err=<nil>
// rawstd "fooba"                  -> Zm9vYmE                                      back ok=true err=<nil>
// rawurl "fooba"                  -> Zm9vYmE                                      back ok=true err=<nil>
// std    "foobar"                 -> Zm9vYmFy                                     back ok=true err=<nil>
// url    "foobar"                 -> Zm9vYmFy                                     back ok=true err=<nil>
// rawstd "foobar"                 -> Zm9vYmFy                                     back ok=true err=<nil>
// rawurl "foobar"                 -> Zm9vYmFy                                     back ok=true err=<nil>
// std    "\x00\xff\xfe\xfb\xff"   -> AP/++/8=                                     back ok=true err=<nil>
// url    "\x00\xff\xfe\xfb\xff"   -> AP_--_8=                                     back ok=true err=<nil>
// rawstd "\x00\xff\xfe\xfb\xff"   -> AP/++/8                                      back ok=true err=<nil>
// rawurl "\x00\xff\xfe\xfb\xff"   -> AP_--_8                                      back ok=true err=<nil>
// std    "Hello, 世界!"             -> SGVsbG8sIOS4lueVjCE=                         back ok=true err=<nil>
// url    "Hello, 世界!"             -> SGVsbG8sIOS4lueVjCE=                         back ok=true err=<nil>
// rawstd "Hello, 世界!"             -> SGVsbG8sIOS4lueVjCE                          back ok=true err=<nil>
// rawurl "Hello, 世界!"             -> SGVsbG8sIOS4lueVjCE                          back ok=true err=<nil>
// std    "01234567890123456789"   -> MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5...  back ok=true err=<nil>
// url    "01234567890123456789"   -> MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5...  back ok=true err=<nil>
// rawstd "01234567890123456789"   -> MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5...  back ok=true err=<nil>
// rawurl "01234567890123456789"   -> MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5...  back ok=true err=<nil>
// 16 14 12 10
// "Zm9v!"        -> "foo" illegal base64 data at input byte 4 corrupt=true
// "Zm9"          -> "" illegal base64 data at input byte 0 corrupt=true
// "Zm9vYg"       -> "foo" illegal base64 data at input byte 4 corrupt=true
// "Zm9vYg="      -> "foo" illegal base64 data at input byte 7 corrupt=true
// "Zm9vYg==="    -> "foob" illegal base64 data at input byte 8 corrupt=true
// "=Zm9"         -> "" illegal base64 data at input byte 0 corrupt=true
// "Zm=v"         -> "" illegal base64 data at input byte 2 corrupt=true
// "Zm9v\nYmFy"   -> "foobar" <nil> corrupt=false
// "Zm9v\r\nYmFy\n" -> "foobar" <nil> corrupt=false
// "Zg==Zg=="     -> "f" illegal base64 data at input byte 4 corrupt=true
// "Zh=="         -> "f" <nil> corrupt=false
// "Zm9vYmE=\n"   -> "fooba" <nil> corrupt=false
// strict: "" illegal base64 data at input byte 2
// strict ok: "f" <nil>
// url: [251 255 191] <nil>
// std: [251 255 191] <nil>
// std with url chars: [] illegal base64 data at input byte 0
// B6EawT0gRTUhxTssBnE9 custom alphabet <nil>
// YWI
// prefix:aGVsbG8=
// decoded:hello <nil>
// MTIzNDU= 5 "12345" <nil>
// <nil> VGhlIHF1aWNrIGJyb3duIGZveCBqdW1wcyBvdmVyIHRoZSBsYXp5IGRvZw==
// "The quick brown fox jumps over the lazy dog" <nil>
// "foobarb" <nil>
// "foo" illegal base64 data at input byte 4
// "foobarb" <nil>
// 40000 -_8B-_8B-_8B-_8B-_8B-_8B
