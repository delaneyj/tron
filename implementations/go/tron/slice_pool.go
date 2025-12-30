package tron

import "github.com/delaneyj/toolbelt"

var (
	uint32Pool         = toolbelt.New(func() []uint32 { return make([]uint32, 0, 16) })
	valuePool          = toolbelt.New(func() []Value { return make([]Value, 0, 16) })
	entryPool          = toolbelt.New(func() []MapLeafEntry { return make([]MapLeafEntry, 0, 8) })
	arrayEntryPool     = toolbelt.New(func() []arrayEntry { return make([]arrayEntry, 0, 16) })
	mapEntryPool       = toolbelt.New(func() []mapEntry { return make([]mapEntry, 0, 8) })
	mapNodePool        = toolbelt.New(func() *mapNode { return &mapNode{} })
	arrayNodePool      = toolbelt.New(func() *arrayNode { return &arrayNode{} })
	mapNodeSlicePool   = toolbelt.New(func() []*mapNode { return make([]*mapNode, 0, 8) })
	arrayNodeSlicePool = toolbelt.New(func() []*arrayNode { return make([]*arrayNode, 0, 8) })
)

func getUint32Slice(n int) []uint32 {
	if n <= 0 {
		return nil
	}
	s := uint32Pool.Get()
	if cap(s) < n {
		return make([]uint32, n)
	}
	return s[:n]
}

func putUint32Slice(s []uint32) {
	if s == nil {
		return
	}
	s = s[:0]
	uint32Pool.Put(s)
}

func getValueSlice(n int) []Value {
	if n <= 0 {
		return nil
	}
	s := valuePool.Get()
	if cap(s) < n {
		return make([]Value, n)
	}
	return s[:n]
}

func putValueSlice(s []Value) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = Value{}
	}
	s = s[:0]
	valuePool.Put(s)
}

func getEntrySlice(n int) []MapLeafEntry {
	if n <= 0 {
		return nil
	}
	s := entryPool.Get()
	if cap(s) < n {
		return make([]MapLeafEntry, n)
	}
	return s[:n]
}

func putEntrySlice(s []MapLeafEntry) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = MapLeafEntry{}
	}
	s = s[:0]
	entryPool.Put(s)
}

func getMapEntrySlice(n int) []mapEntry {
	if n <= 0 {
		return nil
	}
	s := mapEntryPool.Get()
	if cap(s) < n {
		return make([]mapEntry, n)
	}
	return s[:n]
}

func putMapEntrySlice(s []mapEntry) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = mapEntry{}
	}
	s = s[:0]
	mapEntryPool.Put(s)
}

func getArrayEntrySlice(n int) []arrayEntry {
	if n <= 0 {
		return nil
	}
	s := arrayEntryPool.Get()
	if cap(s) < n {
		return make([]arrayEntry, n)
	}
	return s[:n]
}

func putArrayEntrySlice(s []arrayEntry) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = arrayEntry{}
	}
	s = s[:0]
	arrayEntryPool.Put(s)
}

func getMapNode() *mapNode {
	return mapNodePool.Get()
}

func putMapNode(n *mapNode) {
	if n == nil {
		return
	}
	*n = mapNode{}
	mapNodePool.Put(n)
}

func getArrayNode() *arrayNode {
	return arrayNodePool.Get()
}

func putArrayNode(n *arrayNode) {
	if n == nil {
		return
	}
	*n = arrayNode{}
	arrayNodePool.Put(n)
}

func getMapNodeSlice(n int) []*mapNode {
	if n <= 0 {
		return nil
	}
	s := mapNodeSlicePool.Get()
	if cap(s) < n {
		return make([]*mapNode, n)
	}
	return s[:n]
}

func putMapNodeSlice(s []*mapNode) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = nil
	}
	s = s[:0]
	mapNodeSlicePool.Put(s)
}

func getArrayNodeSlice(n int) []*arrayNode {
	if n <= 0 {
		return nil
	}
	s := arrayNodeSlicePool.Get()
	if cap(s) < n {
		return make([]*arrayNode, n)
	}
	return s[:n]
}

func putArrayNodeSlice(s []*arrayNode) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = nil
	}
	s = s[:0]
	arrayNodeSlicePool.Put(s)
}
