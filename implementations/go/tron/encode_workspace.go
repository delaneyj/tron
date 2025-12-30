package tron

type encodeWorkspace struct {
	mapEntries      [][]mapEntry
	arrayEntries    [][]arrayEntry
	values          [][]Value
	uint32s         [][]uint32
	mapNodes        []*mapNode
	arrayNodes      []*arrayNode
	mapNodeSlices   [][]*mapNode
	arrayNodeSlices [][]*arrayNode
}

func newEncodeWorkspace() *encodeWorkspace {
	return &encodeWorkspace{}
}

func getMapEntrySliceWithWorkspace(n int, workspace *encodeWorkspace) []mapEntry {
	if workspace == nil {
		return getMapEntrySlice(n)
	}
	return workspace.getMapEntrySlice(n)
}

func putMapEntrySliceWithWorkspace(s []mapEntry, workspace *encodeWorkspace) {
	if workspace == nil {
		putMapEntrySlice(s)
		return
	}
	workspace.putMapEntrySlice(s)
}

func getArrayEntrySliceWithWorkspace(n int, workspace *encodeWorkspace) []arrayEntry {
	if workspace == nil {
		return getArrayEntrySlice(n)
	}
	return workspace.getArrayEntrySlice(n)
}

func putArrayEntrySliceWithWorkspace(s []arrayEntry, workspace *encodeWorkspace) {
	if workspace == nil {
		putArrayEntrySlice(s)
		return
	}
	workspace.putArrayEntrySlice(s)
}

func getValueSliceWithWorkspace(n int, workspace *encodeWorkspace) []Value {
	if workspace == nil {
		return getValueSlice(n)
	}
	return workspace.getValueSlice(n)
}

func putValueSliceWithWorkspace(s []Value, workspace *encodeWorkspace) {
	if workspace == nil {
		putValueSlice(s)
		return
	}
	workspace.putValueSlice(s)
}

func getUint32SliceWithWorkspace(n int, workspace *encodeWorkspace) []uint32 {
	if workspace == nil {
		return getUint32Slice(n)
	}
	return workspace.getUint32Slice(n)
}

func putUint32SliceWithWorkspace(s []uint32, workspace *encodeWorkspace) {
	if workspace == nil {
		putUint32Slice(s)
		return
	}
	workspace.putUint32Slice(s)
}

func getMapNodeWithWorkspace(workspace *encodeWorkspace) *mapNode {
	if workspace == nil {
		return getMapNode()
	}
	return workspace.getMapNode()
}

func putMapNodeWithWorkspace(n *mapNode, workspace *encodeWorkspace) {
	if workspace == nil {
		putMapNode(n)
		return
	}
	workspace.putMapNode(n)
}

func getArrayNodeWithWorkspace(workspace *encodeWorkspace) *arrayNode {
	if workspace == nil {
		return getArrayNode()
	}
	return workspace.getArrayNode()
}

func putArrayNodeWithWorkspace(n *arrayNode, workspace *encodeWorkspace) {
	if workspace == nil {
		putArrayNode(n)
		return
	}
	workspace.putArrayNode(n)
}

func getMapNodeSliceWithWorkspace(n int, workspace *encodeWorkspace) []*mapNode {
	if workspace == nil {
		return getMapNodeSlice(n)
	}
	return workspace.getMapNodeSlice(n)
}

func putMapNodeSliceWithWorkspace(s []*mapNode, workspace *encodeWorkspace) {
	if workspace == nil {
		putMapNodeSlice(s)
		return
	}
	workspace.putMapNodeSlice(s)
}

func getArrayNodeSliceWithWorkspace(n int, workspace *encodeWorkspace) []*arrayNode {
	if workspace == nil {
		return getArrayNodeSlice(n)
	}
	return workspace.getArrayNodeSlice(n)
}

func putArrayNodeSliceWithWorkspace(s []*arrayNode, workspace *encodeWorkspace) {
	if workspace == nil {
		putArrayNodeSlice(s)
		return
	}
	workspace.putArrayNodeSlice(s)
}

func (w *encodeWorkspace) getMapEntrySlice(n int) []mapEntry {
	if n <= 0 {
		return nil
	}
	if last := len(w.mapEntries) - 1; last >= 0 {
		s := w.mapEntries[last]
		w.mapEntries = w.mapEntries[:last]
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]mapEntry, n)
}

func (w *encodeWorkspace) putMapEntrySlice(s []mapEntry) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = mapEntry{}
	}
	w.mapEntries = append(w.mapEntries, s[:0])
}

func (w *encodeWorkspace) getArrayEntrySlice(n int) []arrayEntry {
	if n <= 0 {
		return nil
	}
	if last := len(w.arrayEntries) - 1; last >= 0 {
		s := w.arrayEntries[last]
		w.arrayEntries = w.arrayEntries[:last]
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]arrayEntry, n)
}

func (w *encodeWorkspace) putArrayEntrySlice(s []arrayEntry) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = arrayEntry{}
	}
	w.arrayEntries = append(w.arrayEntries, s[:0])
}

func (w *encodeWorkspace) getValueSlice(n int) []Value {
	if n <= 0 {
		return nil
	}
	if last := len(w.values) - 1; last >= 0 {
		s := w.values[last]
		w.values = w.values[:last]
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]Value, n)
}

func (w *encodeWorkspace) putValueSlice(s []Value) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = Value{}
	}
	w.values = append(w.values, s[:0])
}

func (w *encodeWorkspace) getUint32Slice(n int) []uint32 {
	if n <= 0 {
		return nil
	}
	if last := len(w.uint32s) - 1; last >= 0 {
		s := w.uint32s[last]
		w.uint32s = w.uint32s[:last]
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]uint32, n)
}

func (w *encodeWorkspace) putUint32Slice(s []uint32) {
	if s == nil {
		return
	}
	w.uint32s = append(w.uint32s, s[:0])
}

func (w *encodeWorkspace) getMapNode() *mapNode {
	if last := len(w.mapNodes) - 1; last >= 0 {
		n := w.mapNodes[last]
		w.mapNodes = w.mapNodes[:last]
		return n
	}
	return &mapNode{}
}

func (w *encodeWorkspace) putMapNode(n *mapNode) {
	if n == nil {
		return
	}
	*n = mapNode{}
	w.mapNodes = append(w.mapNodes, n)
}

func (w *encodeWorkspace) getArrayNode() *arrayNode {
	if last := len(w.arrayNodes) - 1; last >= 0 {
		n := w.arrayNodes[last]
		w.arrayNodes = w.arrayNodes[:last]
		return n
	}
	return &arrayNode{}
}

func (w *encodeWorkspace) putArrayNode(n *arrayNode) {
	if n == nil {
		return
	}
	*n = arrayNode{}
	w.arrayNodes = append(w.arrayNodes, n)
}

func (w *encodeWorkspace) getMapNodeSlice(n int) []*mapNode {
	if n <= 0 {
		return nil
	}
	if last := len(w.mapNodeSlices) - 1; last >= 0 {
		s := w.mapNodeSlices[last]
		w.mapNodeSlices = w.mapNodeSlices[:last]
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]*mapNode, n)
}

func (w *encodeWorkspace) putMapNodeSlice(s []*mapNode) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = nil
	}
	w.mapNodeSlices = append(w.mapNodeSlices, s[:0])
}

func (w *encodeWorkspace) getArrayNodeSlice(n int) []*arrayNode {
	if n <= 0 {
		return nil
	}
	if last := len(w.arrayNodeSlices) - 1; last >= 0 {
		s := w.arrayNodeSlices[last]
		w.arrayNodeSlices = w.arrayNodeSlices[:last]
		if cap(s) >= n {
			return s[:n]
		}
	}
	return make([]*arrayNode, n)
}

func (w *encodeWorkspace) putArrayNodeSlice(s []*arrayNode) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = nil
	}
	w.arrayNodeSlices = append(w.arrayNodeSlices, s[:0])
}
