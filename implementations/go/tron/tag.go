package tron

// ValueType represents the high 3-bit tag type.
type ValueType uint8

const (
	TypeNil ValueType = iota
	TypeBit
	TypeI64
	TypeF64
	TypeTxt
	TypeBin
	TypeArr
	TypeMap
)

const (
	TagNil      byte = 0x00 // 00000000
	TagBitFalse byte = 0x20 // 00100000
	TagBitTrue  byte = 0x21 // 00100001
	TagI64      byte = 0x40 // 01000000
	TagF64      byte = 0x60 // 01100000
)

// TypeFromTag returns the ValueType encoded in the top 3 bits.
func TypeFromTag(tag byte) ValueType {
	return ValueType(tag >> 5)
}

// LowBits returns the low 5 bits of the tag.
func LowBits(tag byte) byte {
	return tag & 0x1F
}

// IsPacked reports whether the tag uses inline packing (txt/bin/arr/map).
func IsPacked(tag byte) bool {
	return (tag & 0x10) != 0
}
