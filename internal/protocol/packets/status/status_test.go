package status

import (
	"encoding/json"
	"testing"

	"github.com/vitismc/vitis/internal/protocol"
)

func TestStatusResponseEncodeDecode(t *testing.T) {
	source := &StatusResponse{JSONResponse: `{"description":{"text":"Vitis"}}`}
	buf := protocol.NewBuffer(64)
	if err := source.Encode(buf); err != nil {
		t.Fatalf("encode status response failed: %v", err)
	}

	decoded := &StatusResponse{}
	if err := decoded.Decode(protocol.WrapBuffer(buf.Bytes())); err != nil {
		t.Fatalf("decode status response failed: %v", err)
	}
	if decoded.JSONResponse != source.JSONResponse {
		t.Fatalf("unexpected json response: got %q want %q", decoded.JSONResponse, source.JSONResponse)
	}
}

func TestPingEncodeDecode(t *testing.T) {
	tests := []struct {
		name   string
		encode func(*protocol.Buffer) error
		decode func(*protocol.Buffer) (int64, error)
		value  int64
	}{
		{
			name: "ping request",
			encode: func(buf *protocol.Buffer) error {
				return (&PingRequest{Payload: 12345}).Encode(buf)
			},
			decode: func(buf *protocol.Buffer) (int64, error) {
				packet := &PingRequest{}
				err := packet.Decode(buf)
				return packet.Payload, err
			},
			value: 12345,
		},
		{
			name: "ping response",
			encode: func(buf *protocol.Buffer) error {
				return (&PingResponse{Payload: 99999}).Encode(buf)
			},
			decode: func(buf *protocol.Buffer) (int64, error) {
				packet := &PingResponse{}
				err := packet.Decode(buf)
				return packet.Payload, err
			},
			value: 99999,
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			buf := protocol.NewBuffer(16)
			if err := tests[i].encode(buf); err != nil {
				t.Fatalf("encode failed: %v", err)
			}

			value, err := tests[i].decode(protocol.WrapBuffer(buf.Bytes()))
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if value != tests[i].value {
				t.Fatalf("unexpected payload: got %d want %d", value, tests[i].value)
			}
		})
	}
}

func TestBuildResponseJSON(t *testing.T) {
	jsonPayload := BuildResponseJSON(ResponsePayload{
		Version: ResponseVersion{Name: "1.21.4", Protocol: 767},
		Players: ResponsePlayers{
			Max:    200,
			Online: 5,
			Sample: []ResponsePlayerSample{{Name: "Alice", ID: "uuid-a"}},
		},
		Description: ResponseDescription{Text: "Vitis Server"},
		Favicon:     "data:image/png;base64,abc",
	})

	if !json.Valid([]byte(jsonPayload)) {
		t.Fatalf("generated invalid json: %s", jsonPayload)
	}

	var decoded struct {
		Version struct {
			Name     string `json:"name"`
			Protocol int32  `json:"protocol"`
		} `json:"version"`
		Players struct {
			Max    int `json:"max"`
			Online int `json:"online"`
			Sample []struct {
				Name string `json:"name"`
				ID   string `json:"id"`
			} `json:"sample"`
		} `json:"players"`
		Description struct {
			Text string `json:"text"`
		} `json:"description"`
		Favicon string `json:"favicon"`
	}

	if err := json.Unmarshal([]byte(jsonPayload), &decoded); err != nil {
		t.Fatalf("unmarshal generated json failed: %v", err)
	}
	if decoded.Version.Protocol != 767 {
		t.Fatalf("unexpected protocol version: %d", decoded.Version.Protocol)
	}
	if decoded.Players.Max != 200 || decoded.Players.Online != 5 {
		t.Fatalf("unexpected players: max=%d online=%d", decoded.Players.Max, decoded.Players.Online)
	}
	if decoded.Description.Text != "Vitis Server" {
		t.Fatalf("unexpected description: %s", decoded.Description.Text)
	}
	if decoded.Favicon == "" {
		t.Fatal("expected favicon to be present")
	}
}
