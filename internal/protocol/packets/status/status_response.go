package status

import (
	"fmt"
	"strconv"

	"github.com/vitismc/vitis/internal/protocol"
	"github.com/vitismc/vitis/internal/protocol/packetid"
)

// StatusResponse is a status-state response packet containing server metadata JSON.
type StatusResponse struct {
	JSONResponse string
}

// ResponsePayload is the structured payload used to build status response JSON.
type ResponsePayload struct {
	Version     ResponseVersion
	Players     ResponsePlayers
	Description ResponseDescription
	Favicon     string
}

// ResponseVersion contains status protocol version information.
type ResponseVersion struct {
	Name     string
	Protocol int32
}

// ResponsePlayers contains status online-player metadata.
type ResponsePlayers struct {
	Max    int
	Online int
	Sample []ResponsePlayerSample
}

// ResponsePlayerSample contains one sample player entry.
type ResponsePlayerSample struct {
	Name string
	ID   string
}

// ResponseDescription contains the MOTD text object.
type ResponseDescription struct {
	Text string
}

// NewStatusResponse constructs an empty status response packet.
func NewStatusResponse() protocol.Packet {
	return &StatusResponse{}
}

// ID returns the protocol packet id.
func (p *StatusResponse) ID() int32 {
	return int32(packetid.ClientboundStatusServerInfo)
}

// Decode reads StatusResponse payload from buffer.
func (p *StatusResponse) Decode(buf *protocol.Buffer) error {
	jsonResponse, err := buf.ReadString()
	if err != nil {
		return fmt.Errorf("decode status response json: %w", err)
	}
	p.JSONResponse = jsonResponse
	return nil
}

// Encode writes StatusResponse payload to buffer.
func (p *StatusResponse) Encode(buf *protocol.Buffer) error {
	if err := buf.WriteString(p.JSONResponse); err != nil {
		return fmt.Errorf("encode status response json: %w", err)
	}
	return nil
}

// BuildResponseJSON builds protocol-compliant status JSON with minimal allocations.
func BuildResponseJSON(payload ResponsePayload) string {
	out := make([]byte, 0, 256)
	out = append(out, '{')

	out = append(out, '"', 'v', 'e', 'r', 's', 'i', 'o', 'n', '"', ':', '{')
	out = append(out, '"', 'n', 'a', 'm', 'e', '"', ':')
	out = appendJSONString(out, payload.Version.Name)
	out = append(out, ',')
	out = append(out, '"', 'p', 'r', 'o', 't', 'o', 'c', 'o', 'l', '"', ':')
	out = strconv.AppendInt(out, int64(payload.Version.Protocol), 10)
	out = append(out, '}')

	out = append(out, ',')
	out = append(out, '"', 'p', 'l', 'a', 'y', 'e', 'r', 's', '"', ':', '{')
	out = append(out, '"', 'm', 'a', 'x', '"', ':')
	out = strconv.AppendInt(out, int64(payload.Players.Max), 10)
	out = append(out, ',')
	out = append(out, '"', 'o', 'n', 'l', 'i', 'n', 'e', '"', ':')
	out = strconv.AppendInt(out, int64(payload.Players.Online), 10)
	out = append(out, ',')
	out = append(out, '"', 's', 'a', 'm', 'p', 'l', 'e', '"', ':', '[')
	for i := range payload.Players.Sample {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, '{')
		out = append(out, '"', 'n', 'a', 'm', 'e', '"', ':')
		out = appendJSONString(out, payload.Players.Sample[i].Name)
		out = append(out, ',')
		out = append(out, '"', 'i', 'd', '"', ':')
		out = appendJSONString(out, payload.Players.Sample[i].ID)
		out = append(out, '}')
	}
	out = append(out, ']')
	out = append(out, '}')

	out = append(out, ',')
	out = append(out, '"', 'd', 'e', 's', 'c', 'r', 'i', 'p', 't', 'i', 'o', 'n', '"', ':', '{')
	out = append(out, '"', 't', 'e', 'x', 't', '"', ':')
	out = appendJSONString(out, payload.Description.Text)
	out = append(out, '}')

	if payload.Favicon != "" {
		out = append(out, ',')
		out = append(out, '"', 'f', 'a', 'v', 'i', 'c', 'o', 'n', '"', ':')
		out = appendJSONString(out, payload.Favicon)
	}

	out = append(out, '}')
	return string(out)
}

func appendJSONString(dst []byte, value string) []byte {
	dst = append(dst, '"')
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch ch {
		case '"', '\\':
			dst = append(dst, '\\', ch)
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\f':
			dst = append(dst, '\\', 'f')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			if ch < 0x20 {
				dst = append(dst, '\\', 'u', '0', '0', hexUpper[ch>>4], hexUpper[ch&0x0F])
				continue
			}
			dst = append(dst, ch)
		}
	}
	dst = append(dst, '"')
	return dst
}

const hexUpper = "0123456789ABCDEF"
