package path

import (
	"encoding/binary"
	"fmt"

	tron "tron"
)

const hamtMaxDepth = 7

func mapGetBytes(doc []byte, off uint32, key []byte, depth int) (tron.Value, bool, error) {
	return mapGetBytesHashed(doc, off, key, tron.XXH32(key, 0), depth)
}

func mapGetBytesHashed(doc []byte, off uint32, key []byte, hash uint32, depth int) (tron.Value, bool, error) {
	if depth > hamtMaxDepth {
		return tron.Value{}, false, fmt.Errorf("map depth exceeds max")
	}
	h, node, err := tron.NodeSliceAt(doc, off)
	if err != nil {
		return tron.Value{}, false, err
	}
	if h.KeyType != tron.KeyMap {
		return tron.Value{}, false, fmt.Errorf("node is not a map")
	}
	if h.Kind == tron.NodeLeaf {
		p := 8
		for i := uint32(0); i < h.EntryCount; i++ {
			keyVal, n, err := tron.DecodeValue(node[p:])
			if err != nil {
				return tron.Value{}, false, fmt.Errorf("key decode failed: %w", err)
			}
			if keyVal.Type != tron.TypeTxt {
				return tron.Value{}, false, fmt.Errorf("map key is not txt")
			}
			p += n
			val, m, err := tron.DecodeValue(node[p:])
			if err != nil {
				return tron.Value{}, false, fmt.Errorf("value decode failed: %w", err)
			}
			p += m
			if bytesEqual(keyVal.Bytes, key) {
				return val, true, nil
			}
		}
		return tron.Value{}, false, nil
	}

	if len(node) < 12 {
		return tron.Value{}, false, fmt.Errorf("map branch node too small")
	}
	bitmap := binary.LittleEndian.Uint16(node[8:10])
	reserved := binary.LittleEndian.Uint16(node[10:12])
	if reserved != 0 {
		return tron.Value{}, false, fmt.Errorf("map branch reserved must be 0")
	}
	slot := uint8((hash >> (depth * 4)) & 0xF)
	if ((bitmap >> slot) & 1) == 0 {
		return tron.Value{}, false, nil
	}
	mask := uint16((uint32(1) << slot) - 1)
	idx := popcount16(bitmap & mask)
	childPos := 12 + idx*4
	if childPos+4 > len(node) {
		return tron.Value{}, false, fmt.Errorf("child offset truncated")
	}
	child := binary.LittleEndian.Uint32(node[childPos : childPos+4])
	return mapGetBytesHashed(doc, child, key, hash, depth+1)
}

func mapIterValues(doc []byte, off uint32, fn func(tron.Value) error) error {
	h, node, err := tron.NodeSliceAt(doc, off)
	if err != nil {
		return err
	}
	if h.KeyType != tron.KeyMap {
		return fmt.Errorf("node is not a map")
	}
	if h.Kind == tron.NodeLeaf {
		p := 8
		for i := uint32(0); i < h.EntryCount; i++ {
			_, n, err := tron.DecodeValue(node[p:])
			if err != nil {
				return fmt.Errorf("key decode failed: %w", err)
			}
			p += n
			val, m, err := tron.DecodeValue(node[p:])
			if err != nil {
				return fmt.Errorf("value decode failed: %w", err)
			}
			p += m
			if err := fn(val); err != nil {
				return err
			}
		}
		return nil
	}
	if len(node) < 12 {
		return fmt.Errorf("map branch node too small")
	}
	bitmap := binary.LittleEndian.Uint16(node[8:10])
	reserved := binary.LittleEndian.Uint16(node[10:12])
	if reserved != 0 {
		return fmt.Errorf("map branch reserved must be 0")
	}
	count := popcount16(bitmap)
	for i := 0; i < count; i++ {
		childPos := 12 + i*4
		if childPos+4 > len(node) {
			return fmt.Errorf("child offset truncated")
		}
		child := binary.LittleEndian.Uint32(node[childPos : childPos+4])
		if err := mapIterValues(doc, child, fn); err != nil {
			return err
		}
	}
	return nil
}

func mapIterEntries(doc []byte, off uint32, fn func(key []byte, val tron.Value) error) error {
	h, node, err := tron.NodeSliceAt(doc, off)
	if err != nil {
		return err
	}
	if h.KeyType != tron.KeyMap {
		return fmt.Errorf("node is not a map")
	}
	if h.Kind == tron.NodeLeaf {
		p := 8
		for i := uint32(0); i < h.EntryCount; i++ {
			keyVal, n, err := tron.DecodeValue(node[p:])
			if err != nil {
				return fmt.Errorf("key decode failed: %w", err)
			}
			if keyVal.Type != tron.TypeTxt {
				return fmt.Errorf("map key is not txt")
			}
			p += n
			val, m, err := tron.DecodeValue(node[p:])
			if err != nil {
				return fmt.Errorf("value decode failed: %w", err)
			}
			p += m
			if err := fn(keyVal.Bytes, val); err != nil {
				return err
			}
		}
		return nil
	}
	if len(node) < 12 {
		return fmt.Errorf("map branch node too small")
	}
	bitmap := binary.LittleEndian.Uint16(node[8:10])
	reserved := binary.LittleEndian.Uint16(node[10:12])
	if reserved != 0 {
		return fmt.Errorf("map branch reserved must be 0")
	}
	count := popcount16(bitmap)
	for i := 0; i < count; i++ {
		childPos := 12 + i*4
		if childPos+4 > len(node) {
			return fmt.Errorf("child offset truncated")
		}
		child := binary.LittleEndian.Uint32(node[childPos : childPos+4])
		if err := mapIterEntries(doc, child, fn); err != nil {
			return err
		}
	}
	return nil
}

func arrGetRaw(doc []byte, off uint32, index uint32) (tron.Value, bool, error) {
	h, node, err := tron.NodeSliceAt(doc, off)
	if err != nil {
		return tron.Value{}, false, err
	}
	if h.KeyType != tron.KeyArr {
		return tron.Value{}, false, fmt.Errorf("node is not an array")
	}
	if len(node) < 16 {
		return tron.Value{}, false, fmt.Errorf("array node too small")
	}
	shift := node[8]
	reserved := node[9]
	if reserved != 0 {
		return tron.Value{}, false, fmt.Errorf("array reserved must be 0")
	}
	bitmap := binary.LittleEndian.Uint16(node[10:12])
	if h.Kind == tron.NodeLeaf {
		if shift != 0 {
			return tron.Value{}, false, fmt.Errorf("array leaf shift must be 0")
		}
		slot := uint8(index & 0xF)
		if ((bitmap >> slot) & 1) == 0 {
			return tron.Value{}, false, nil
		}
		mask := uint16((uint32(1) << slot) - 1)
		idx := popcount16(bitmap & mask)
		p := 16
		for i := uint32(0); i < h.EntryCount; i++ {
			val, n, err := tron.DecodeValue(node[p:])
			if err != nil {
				return tron.Value{}, false, fmt.Errorf("value decode failed: %w", err)
			}
			if int(i) == idx {
				return val, true, nil
			}
			p += n
		}
		return tron.Value{}, false, fmt.Errorf("array leaf index missing")
	}

	slot := uint8((index >> shift) & 0xF)
	if ((bitmap >> slot) & 1) == 0 {
		return tron.Value{}, false, nil
	}
	mask := uint16((uint32(1) << slot) - 1)
	idx := popcount16(bitmap & mask)
	childPos := 16 + idx*4
	if childPos+4 > len(node) {
		return tron.Value{}, false, fmt.Errorf("child offset truncated")
	}
	child := binary.LittleEndian.Uint32(node[childPos : childPos+4])
	return arrGetRaw(doc, child, index)
}

func arrayLength(doc []byte, off uint32) (uint32, error) {
	return tron.ArrayRootLength(doc, off)
}

func arrIterValues(doc []byte, off uint32, fn func(tron.Value) error) error {
	h, node, err := tron.NodeSliceAt(doc, off)
	if err != nil {
		return err
	}
	if h.KeyType != tron.KeyArr {
		return fmt.Errorf("node is not an array")
	}
	if len(node) < 16 {
		return fmt.Errorf("array node too small")
	}
	shift := node[8]
	reserved := node[9]
	if reserved != 0 {
		return fmt.Errorf("array reserved must be 0")
	}
	bitmap := binary.LittleEndian.Uint16(node[10:12])
	if h.Kind == tron.NodeLeaf {
		if shift != 0 {
			return fmt.Errorf("array leaf shift must be 0")
		}
		p := 16
		for i := uint32(0); i < h.EntryCount; i++ {
			val, n, err := tron.DecodeValue(node[p:])
			if err != nil {
				return fmt.Errorf("value decode failed: %w", err)
			}
			p += n
			if err := fn(val); err != nil {
				return err
			}
		}
		return nil
	}
	count := popcount16(bitmap)
	for i := 0; i < count; i++ {
		childPos := 16 + i*4
		if childPos+4 > len(node) {
			return fmt.Errorf("child offset truncated")
		}
		child := binary.LittleEndian.Uint32(node[childPos : childPos+4])
		if err := arrIterValues(doc, child, fn); err != nil {
			return err
		}
	}
	return nil
}

func arrCollectValues(doc []byte, off uint32, base uint32, values []tron.Value, present []bool) error {
	h, node, err := tron.NodeSliceAt(doc, off)
	if err != nil {
		return err
	}
	if h.KeyType != tron.KeyArr {
		return fmt.Errorf("node is not an array")
	}
	if len(node) < 16 {
		return fmt.Errorf("array node too small")
	}
	shift := node[8]
	reserved := node[9]
	if reserved != 0 {
		return fmt.Errorf("array reserved must be 0")
	}
	bitmap := binary.LittleEndian.Uint16(node[10:12])
	if h.Kind == tron.NodeLeaf {
		if shift != 0 {
			return fmt.Errorf("array leaf shift must be 0")
		}
		p := 16
		idx := 0
		for slot := 0; slot < 16; slot++ {
			if ((bitmap >> uint(slot)) & 1) == 0 {
				continue
			}
			val, n, err := tron.DecodeValue(node[p:])
			if err != nil {
				return fmt.Errorf("value decode failed: %w", err)
			}
			p += n
			index := base + uint32(slot)
			if index >= uint32(len(values)) {
				return fmt.Errorf("array index out of range: %d", index)
			}
			values[index] = val
			present[index] = true
			idx++
		}
		return nil
	}
	childIdx := 0
	for slot := 0; slot < 16; slot++ {
		if ((bitmap >> uint(slot)) & 1) == 0 {
			continue
		}
		childPos := 16 + childIdx*4
		if childPos+4 > len(node) {
			return fmt.Errorf("child offset truncated")
		}
		child := binary.LittleEndian.Uint32(node[childPos : childPos+4])
		childBase := base + (uint32(slot) << shift)
		if err := arrCollectValues(doc, child, childBase, values, present); err != nil {
			return err
		}
		childIdx++
	}
	return nil
}

func popcount16(x uint16) int {
	x = x - ((x >> 1) & 0x5555)
	x = (x & 0x3333) + ((x >> 2) & 0x3333)
	x = (x + (x >> 4)) & 0x0F0F
	x = x + (x >> 8)
	return int(x & 0x1F)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
