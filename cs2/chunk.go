package cs2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

const (
	minimumLength = 18
)

var (
	ErrInsufficientData   = errors.New("insufficient amount of data to chunk")
	ErrInvalidVoicePacket = errors.New("invalid voice packet")
	ErrMismatchChecksum   = errors.New("mismatching voice data checksum")
)

type Chunk struct {
	SteamID    uint64
	SampleRate uint16
	Length     uint16
	Data       []byte
	Checksum   uint32
}

func DecodeChunk(b []byte) (*Chunk, error) {
	bLen := len(b)

	if bLen < minimumLength {
		return nil, fmt.Errorf("%w (received: %d bytes, expected at least %d bytes)", ErrInsufficientData, bLen, minimumLength)
	}

	chunk := &Chunk{}

	buf := bytes.NewBuffer(b)

	if err := binary.Read(buf, binary.LittleEndian, &chunk.SteamID); err != nil {
		return nil, err
	}

	var payloadType byte
	if err := binary.Read(buf, binary.LittleEndian, &payloadType); err != nil {
		return nil, err
	}

	if payloadType != 0x0B {
		return nil, fmt.Errorf("%w (received %x, expected %x)", ErrInvalidVoicePacket, payloadType, 0x0B)
	}

	if err := binary.Read(buf, binary.LittleEndian, &chunk.SampleRate); err != nil {
		return nil, err
	}

	var voiceType byte
	if err := binary.Read(buf, binary.LittleEndian, &voiceType); err != nil {
		return nil, err
	}

	if err := binary.Read(buf, binary.LittleEndian, &chunk.Length); err != nil {
		return nil, err
	}

	switch voiceType {
	case 0x6:
		remaining := buf.Len()
		chunkLen := int(chunk.Length)

		if remaining < chunkLen {
			return nil, fmt.Errorf("%w (received: %d bytes, expected at least %d bytes)", ErrInsufficientData, bLen, (bLen + (chunkLen - remaining)))
		}

		data := make([]byte, chunkLen)
		n, err := buf.Read(data)

		if err != nil {
			return nil, err
		}

		// Is this even possible
		if n != chunkLen {
			return nil, fmt.Errorf("%w (expected to read %d bytes, but read %d bytes)", ErrInsufficientData, chunkLen, n)
		}

		chunk.Data = data
	case 0x0:
		// no-op, detect silence if chunk.Data is empty
		// the length would the number of silence frames
	default:
		return nil, fmt.Errorf("%w (expected 0x6 or 0x0 voice data, received %x)", ErrInvalidVoicePacket, voiceType)
	}

	remaining := buf.Len()

	if remaining != 4 {
		return nil, fmt.Errorf("%w (has %d bytes remaining, expected 4 bytes remaining)", ErrInvalidVoicePacket, remaining)
	}

	if err := binary.Read(buf, binary.LittleEndian, &chunk.Checksum); err != nil {
		return nil, err
	}

	actualChecksum := crc32.ChecksumIEEE(b[0 : bLen-4])

	if chunk.Checksum != actualChecksum {
		return nil, fmt.Errorf("%w (received %x, expected %x)", ErrMismatchChecksum, chunk.Checksum, actualChecksum)
	}

	return chunk, nil
}
