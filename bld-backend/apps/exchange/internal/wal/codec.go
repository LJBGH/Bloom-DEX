package wal

import (
	"encoding/binary"
	"hash/crc32"
	"io"
)

const frameHeaderSize = 4 + 1 + 1 + 2 + 8 + 8 + 8 + 4 + 4

// encodeFrame 编码一个 Record 为字节切片。
func encodeFrame(rec Record) []byte {
	payloadLen := uint32(len(rec.Payload))
	frame := make([]byte, frameHeaderSize+len(rec.Payload))
	binary.LittleEndian.PutUint32(frame[0:4], frameMagic)
	frame[4] = frameVersion
	frame[5] = byte(rec.Type)
	binary.LittleEndian.PutUint16(frame[6:8], 0)
	binary.LittleEndian.PutUint64(frame[8:16], rec.LSN)
	// tx_id removed from logical record; keep header slot as 0 for compatibility.
	binary.LittleEndian.PutUint64(frame[16:24], 0)
	binary.LittleEndian.PutUint64(frame[24:32], uint64(rec.TsMs))
	binary.LittleEndian.PutUint32(frame[32:36], payloadLen)
	copy(frame[40:], rec.Payload)
	crc := crc32.ChecksumIEEE(frame[4:36])
	crc = crc32.Update(crc, crc32.IEEETable, rec.Payload)
	binary.LittleEndian.PutUint32(frame[36:40], crc)
	return frame
}

// decodeFrameFrom 从 io.Reader 中解码一个 Record。
func decodeFrameFrom(r io.Reader) (Record, error) {
	var rec Record
	hdr := make([]byte, frameHeaderSize)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return rec, err
	}
	if binary.LittleEndian.Uint32(hdr[0:4]) != frameMagic {
		return rec, ErrCorruptedWAL
	}
	if hdr[4] != frameVersion {
		return rec, ErrCorruptedWAL
	}
	rec.Type = RecordType(hdr[5])
	rec.LSN = binary.LittleEndian.Uint64(hdr[8:16])
	rec.TsMs = int64(binary.LittleEndian.Uint64(hdr[24:32]))
	payloadLen := binary.LittleEndian.Uint32(hdr[32:36])
	wantCRC := binary.LittleEndian.Uint32(hdr[36:40])
	rec.Payload = make([]byte, int(payloadLen))
	if payloadLen > 0 {
		if _, err := io.ReadFull(r, rec.Payload); err != nil {
			return Record{}, err
		}
	}
	crc := crc32.ChecksumIEEE(hdr[4:36])
	crc = crc32.Update(crc, crc32.IEEETable, rec.Payload)
	if crc != wantCRC {
		return Record{}, ErrCorruptedWAL
	}
	return rec, nil
}
