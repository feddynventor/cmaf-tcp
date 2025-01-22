package main

import (
	"encoding/binary"
	"fmt"
)

func extractSequenceNumber(moof []byte) uint32 {
	offset := 8 // Skip moof header
	for offset+8 <= len(moof) {
		size := binary.BigEndian.Uint32(moof[offset : offset+4])
		typ := string(moof[offset+4 : offset+8])
		if typ == "mfhd" {
			return binary.BigEndian.Uint32(moof[offset+12 : offset+16])
		}
		offset += int(size)
	}
	// by specs, sequence number starts from 1
	return 0
}

func getTrafAtom(moov []byte) uint32 {
	offset := 8 // Skip moof header
	for offset+8 <= len(moov) {
		size := binary.BigEndian.Uint32(moov[offset : offset+4])
		typ := string(moov[offset+4 : offset+8])
		if typ == "traf" {
			return extractTrunFlags(moov[offset : offset+int(size)])
		}
		offset += int(size)
	}
	fmt.Println("No traf found in moov")
	return 0
}

func extractTrunFlags(traf []byte) uint32 {
	offset := 8 // Skip traf header
	for offset+8 <= len(traf) {
		size := binary.BigEndian.Uint32(traf[offset : offset+4])
		typ := string(traf[offset+4 : offset+8])
		if typ == "trun" {
			flags := binary.BigEndian.Uint32([]byte{0, traf[offset+9], traf[offset+10], traf[offset+11]})
			return flags
		}
		offset += int(size)
	}
	fmt.Println("No trun found in traf")
	return 0
}
