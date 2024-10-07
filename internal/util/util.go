package util

import (
	"reflect"
)

func IsSlice(value any) bool {
	return reflect.TypeOf(value).Kind() == reflect.Slice
}

func ExistsInSlice(value any, checkList []any) bool {
	exists := false
	for i := 0; i < len(checkList); i++ {
		if checkList[i] == value {
			exists = true
			break
		}
	}
	return exists
}
