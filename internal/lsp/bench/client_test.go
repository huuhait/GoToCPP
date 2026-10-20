package bench

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

func TestClientReplyKinds(t *testing.T) {
	for _, kind := range []string{"closed stream", "missing result", "null", "refusal"} {
		t.Run(kind, func(t *testing.T) {
			inR, inW := io.Pipe()
			outR, outW := io.Pipe()
			defer inR.Close()
			defer inW.Close()
			defer outR.Close()
			defer outW.Close()
			c := NewClient(outR, inW, nil)
			serverDone := make(chan error, 1)
			go func() {
				defer outW.Close()
				body, err := protocol.NewReader(inR).Read()
				if err != nil {
					serverDone <- err
					return
				}
				m, err := protocol.DecodeMessage(body)
				if err != nil {
					serverDone <- err
					return
				}
				id := m.(*protocol.Request).ID
				var response []byte
				switch kind {
				case "closed stream":
					serverDone <- nil
					return
				case "missing result":
					response = []byte(`{"jsonrpc":"2.0","id":` + id.String() + `}`)
				case "null":
					response, err = protocol.EncodeMessage(&protocol.Response{
						ID:     id,
						Result: json.RawMessage("null"),
					})
				case "refusal":
					response, err = protocol.EncodeMessage(&protocol.Response{
						ID:    id,
						Error: protocol.Errorf(protocol.RequestFailed, "cannot rename"),
					})
				}
				if err == nil {
					err = protocol.NewWriter(outW).Write(response)
				}
				serverDone <- err
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var raw json.RawMessage
			_, err := c.Call(ctx, "textDocument/rename", nil, &raw)
			var rpc *protocol.ResponseError
			switch kind {
			case "closed stream", "missing result":
				if !errors.Is(err, ErrServerEnded) || errors.As(err, &rpc) {
					t.Fatalf("stream failure was not distinguished from a JSON-RPC refusal: %v", err)
				}
			case "null":
				if err != nil || string(raw) != "null" {
					t.Fatalf("JSON null result was lost: %q, %v", raw, err)
				}
			case "refusal":
				if !errors.As(err, &rpc) || rpc.Code != int64(protocol.RequestFailed) {
					t.Fatalf("explicit RPC refusal was lost: %v", err)
				}
			}
			select {
			case err := <-serverDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("fake server did not finish")
			}
		})
	}
}
