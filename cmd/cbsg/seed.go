package main

import (
	"crypto/rand"
	"encoding/binary"
)

func nowSeed() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 1
	}
	return int64(binary.LittleEndian.Uint64(b[:]))
}
