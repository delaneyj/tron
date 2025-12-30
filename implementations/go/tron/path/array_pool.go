package path

import (
	"sync"

	tron "tron"
)

var (
	valueSlicePool = sync.Pool{
		New: func() any {
			return make([]tron.Value, 0, 16)
		},
	}
	boolSlicePool = sync.Pool{
		New: func() any {
			return make([]bool, 0, 16)
		},
	}
)

func getValueSlice(n int) []tron.Value {
	if n <= 0 {
		return nil
	}
	s := valueSlicePool.Get().([]tron.Value)
	if cap(s) < n {
		return make([]tron.Value, n)
	}
	return s[:n]
}

func putValueSlice(s []tron.Value) {
	if s == nil {
		return
	}
	for i := range s {
		s[i] = tron.Value{}
	}
	s = s[:0]
	valueSlicePool.Put(s)
}

func getBoolSlice(n int) []bool {
	if n <= 0 {
		return nil
	}
	s := boolSlicePool.Get().([]bool)
	if cap(s) < n {
		return make([]bool, n)
	}
	return s[:n]
}

func putBoolSlice(s []bool) {
	if s == nil {
		return
	}
	clear(s)
	s = s[:0]
	boolSlicePool.Put(s)
}
