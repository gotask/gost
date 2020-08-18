package stutil

import (
	"reflect"
)

func ReflectStructField(st interface{}, filedname string) interface{} {
	v := reflect.ValueOf(st)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	fv := v.FieldByName(filedname)
	if fv.IsValid() {
		return fv.Interface()
	}
	return nil
}

func ReflectSliceIndex(sl interface{}, index int) interface{} {
	v := reflect.ValueOf(sl)
	if v.Kind() != reflect.Slice {
		return nil
	}
	l := v.Len()
	if index >= l {
		return nil
	}
	return v.Index(index).Interface()
}

func ReflectMapValue(mp interface{}, key interface{}) interface{} {
	v := reflect.ValueOf(mp)
	if v.Kind() != reflect.Map {
		return nil
	}
	fv := v.MapIndex(reflect.ValueOf(key))
	if fv.IsValid() {
		return fv.Interface()
	}
	return nil
}
