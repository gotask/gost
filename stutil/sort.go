package stutil

import (
	"reflect"
	"sort"
)

type Pair struct {
	A interface{}
	B interface{}
}

func less(a, b interface{}) bool {
	ta := reflect.TypeOf(a)
	tb := reflect.TypeOf(b)
	if ta != tb {
		return false
	}
	switch ta.Kind() {
	case reflect.Int:
		return a.(int) < b.(int)
	case reflect.Int8:
		return a.(int8) < b.(int8)
	case reflect.Int16:
		return a.(int16) < b.(int16)
	case reflect.Int32:
		return a.(int32) < b.(int32)
	case reflect.Int64:
		return a.(int64) < b.(int64)
	case reflect.Uint:
		return a.(uint) < b.(uint)
	case reflect.Uint8:
		return a.(uint8) < b.(uint8)
	case reflect.Uint16:
		return a.(uint16) < b.(uint16)
	case reflect.Uint32:
		return a.(uint32) < b.(uint32)
	case reflect.Uint64:
		return a.(uint64) < b.(uint64)
	case reflect.Float32:
		return a.(float32) < b.(float32)
	case reflect.Float64:
		return a.(float64) < b.(float64)
	case reflect.String:
		return a.(string) < b.(string)
	}
	return false
}

type mapKeySort []Pair

func (a mapKeySort) Len() int           { return len(a) }
func (a mapKeySort) Less(i, j int) bool { return less(a[i].A, a[j].A) }
func (a mapKeySort) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type mapValSort []Pair

func (a mapValSort) Len() int           { return len(a) }
func (a mapValSort) Less(i, j int) bool { return less(a[i].B, a[j].B) }
func (a mapValSort) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func SortMap(mp interface{}, byKey bool) []Pair {
	var res []Pair
	v := reflect.ValueOf(mp)
	if v.Kind() != reflect.Map {
		return res
	}

	ks := v.MapKeys()
	for _, key := range ks {
		val := v.MapIndex(key)
		res = append(res, Pair{key.Interface(), val.Interface()})
	}

	if byKey {
		sort.Sort(mapKeySort(res))
	} else {
		sort.Sort(mapValSort(res))
	}
	return res
}
