package wal_analysis

import (
	"encoding/binary"
	"hash/crc32"
	"io"
	"os"

	"bld-backend/apps/exchange/internal/wal"
)

const (
	frameHeaderSize = 4 + 1 + 1 + 2 + 8 + 8 + 8 + 4 + 4
	frameMagic      = 0x57414c32 // "WAL2"
	frameVersion    = 1
)

// 从 io.Reader 中解码一个 Record。
func decodeFrameFrom(r io.Reader) (wal.Record, error) {
	var rec wal.Record
	hdr := make([]byte, frameHeaderSize)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return rec, err
	}
	if binary.LittleEndian.Uint32(hdr[0:4]) != frameMagic {
		return rec, wal.ErrCorruptedWAL
	}
	if hdr[4] != frameVersion {
		return rec, wal.ErrCorruptedWAL
	}
	rec.Type = wal.RecordType(hdr[5])
	rec.LSN = binary.LittleEndian.Uint64(hdr[8:16])
	rec.TsMs = int64(binary.LittleEndian.Uint64(hdr[24:32]))
	payloadLen := binary.LittleEndian.Uint32(hdr[32:36])
	wantCRC := binary.LittleEndian.Uint32(hdr[36:40])
	rec.Payload = make([]byte, int(payloadLen))
	if payloadLen > 0 {
		if _, err := io.ReadFull(r, rec.Payload); err != nil {
			return wal.Record{}, err
		}
	}
	crc := crc32.ChecksumIEEE(hdr[4:36])
	crc = crc32.Update(crc, crc32.IEEETable, rec.Payload)
	if crc != wantCRC {
		return wal.Record{}, wal.ErrCorruptedWAL
	}
	return rec, nil
}

// 检查 WAL 文件是否为旧的 JSON 格式。
func isLegacyJSONWAL(f *os.File) (bool, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	buf := make([]byte, 1)
	for {
		n, err := f.Read(buf)
		if err != nil {
			if err == io.EOF {
				return false, nil
			}
			return false, err
		}
		if n == 0 {
			return false, nil
		}
		b := buf[0]
		if b == '{' {
			return true, nil
		}
		if b == '\n' || b == '\r' || b == '\t' || b == ' ' {
			continue
		}
		return false, nil
	}
}
