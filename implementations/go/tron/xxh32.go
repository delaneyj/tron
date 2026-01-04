package tron

import "encoding/binary"

const (
	xxh32Prime1 uint32 = 0x9E3779B1
	xxh32Prime2 uint32 = 0x85EBCA77
	xxh32Prime3 uint32 = 0xC2B2AE3D
	xxh32Prime4 uint32 = 0x27D4EB2F
	xxh32Prime5 uint32 = 0x165667B1
)

// XXH32 implements the xxh32 hash with a seed, per the spec.
func XXH32(data []byte, seed uint32) uint32 {
	rotl := func(x uint32, r uint32) uint32 {
		return (x << r) | (x >> (32 - r))
	}
	round := func(acc, input uint32) uint32 {
		acc += input * xxh32Prime2
		acc = rotl(acc, 13)
		acc *= xxh32Prime1
		return acc
	}

	p := 0
	length := len(data)
	var h32 uint32

	if length >= 16 {
		v1 := seed + xxh32Prime1 + xxh32Prime2
		v2 := seed + xxh32Prime2
		v3 := seed + 0
		v4 := seed - xxh32Prime1
		for p <= length-16 {
			v1 = round(v1, binary.LittleEndian.Uint32(data[p:]))
			p += 4
			v2 = round(v2, binary.LittleEndian.Uint32(data[p:]))
			p += 4
			v3 = round(v3, binary.LittleEndian.Uint32(data[p:]))
			p += 4
			v4 = round(v4, binary.LittleEndian.Uint32(data[p:]))
			p += 4
		}
		h32 = rotl(v1, 1) + rotl(v2, 7) + rotl(v3, 12) + rotl(v4, 18)
	} else {
		h32 = seed + xxh32Prime5
	}

	h32 += uint32(length)

	for p <= length-4 {
		h32 += binary.LittleEndian.Uint32(data[p:]) * xxh32Prime3
		p += 4
		h32 = rotl(h32, 17) * xxh32Prime4
	}
	for p < length {
		h32 += uint32(data[p]) * xxh32Prime5
		p++
		h32 = rotl(h32, 11) * xxh32Prime1
	}

	h32 ^= h32 >> 15
	h32 *= xxh32Prime2
	h32 ^= h32 >> 13
	h32 *= xxh32Prime3
	h32 ^= h32 >> 16
	return h32
}
