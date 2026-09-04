package protocol

type Decoder struct {
	buffer     []byte
	maxPayload uint32
	failed     error
}

func NewDecoder(maxPayload uint32) *Decoder {
	if maxPayload == 0 {
		maxPayload = MaxControlPayload
	}
	return &Decoder{maxPayload: maxPayload}
}

func (d *Decoder) Feed(data []byte) ([]Frame, error) {
	if d.failed != nil {
		return nil, d.failed
	}
	d.buffer = append(d.buffer, data...)
	frames := make([]Frame, 0)
	for len(d.buffer) >= HeaderSize {
		header, err := DecodeHeader(d.buffer[:HeaderSize], d.maxPayload)
		if err != nil {
			d.failed = err
			return frames, err
		}
		frameLen := HeaderSize + int(header.PayloadLen)
		if len(d.buffer) < frameLen {
			break
		}
		payload := make([]byte, header.PayloadLen)
		copy(payload, d.buffer[HeaderSize:frameLen])
		frames = append(frames, Frame{Header: header, Payload: payload})
		d.buffer = d.buffer[frameLen:]
	}
	return frames, nil
}

func (d *Decoder) Buffered() int {
	return len(d.buffer)
}
