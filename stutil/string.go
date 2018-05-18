package stutil

import (
	"strconv"
	"strings"
)

func StringToInt(a string) int64 {
	i, _ := strconv.ParseInt(a, 0, 64)
	return i
}

func StringToUint(a string) uint64 {
	i, _ := strconv.ParseUint(a, 0, 64)
	return i
}

func StringToFloat(a string) float64 {
	f, _ := strconv.ParseFloat(a, 64)
	return f
}

func StringToIntList(s, sep string) []int64 {
	sl := strings.Split(s, sep)
	if sl == nil {
		return nil
	}
	il := make([]int64, len(sl))
	for i, v := range sl {
		il[i] = StringToInt(v)
	}
	return il
}
func StringToUintList(s, sep string) []uint64 {
	sl := strings.Split(s, sep)
	if sl == nil {
		return nil
	}
	il := make([]uint64, len(sl))
	for i, v := range sl {
		il[i] = StringToUint(v)
	}
	return il
}
func StringToFloatList(s, sep string) []float64 {
	sl := strings.Split(s, sep)
	if sl == nil {
		return nil
	}
	fl := make([]float64, len(sl))
	for i, v := range sl {
		fl[i] = StringToFloat(v)
	}
	return fl
}
