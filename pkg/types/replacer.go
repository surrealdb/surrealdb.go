package types

import (
	"reflect"
	"time"
)

func replacerBeforeEncode(value interface{}) interface{} {
	valueType := reflect.TypeOf(value)
	valueKind := valueType.Kind()

	if valueType == reflect.TypeOf(time.Duration(0)) {
		oldVal := value.(time.Duration)
		newValue := CustomDuration.Nanoseconds(oldVal.Nanoseconds())
		return newValue
	}

	if valueKind == reflect.Map {
		oldValue := value.(map[string]interface{})
		newValue := make(map[interface{}]interface{})
		for k, v := range oldValue {
			newKey := replacerBeforeEncode(k)
			newVal := replacerBeforeEncode(v)
			newValue[newKey] = newVal
		}

		return newValue
	}

	return value
}

func replacerAfterDecode(value interface{}) interface{} {
	valueType := reflect.TypeOf(value)
	valueKind := valueType.Kind()

	if valueType == reflect.TypeOf(CustomDuration{}) {
		oldVal := value.(CustomDuration)
		newValue := time.Duration((oldVal.Nanoseconds()))
		return newValue
	}

	if valueKind == reflect.Map {
		oldValue := value.(map[string]interface{})
		newValue := make(map[interface{}]interface{})
		for k, v := range oldValue {
			newKey := replacerAfterDecode(k)
			newVal := replacerAfterDecode(v)
			newValue[newKey] = newVal
		}

		return newValue
	}

	return value
}
