package sql

import "fmt"

type StringSlice struct {
	Data []string
}

func (s *StringSlice) Scan(src any) error {
	arr, ok := src.([]interface{})
	if !ok {
		return fmt.Errorf("unsupported value")
	}

	if len(arr) == 0 {
		return nil
	}

	for _, elem := range arr {
		str, ok := elem.(string)
		if !ok {
			return fmt.Errorf("unsupported value in array")
		}

		s.Data = append(s.Data, str)
	}

	return nil
}

type IntSlice struct {
	Data []int
}

func (s *IntSlice) Scan(src any) error {
	arr, ok := src.([]interface{})
	if !ok {
		return fmt.Errorf("unsupported value")
	}

	if len(arr) == 0 {
		return nil
	}

	for _, elem := range arr {
		switch num := elem.(type) {
		case int:
			s.Data = append(s.Data, num)
		case int64:
			s.Data = append(s.Data, int(num))
		case int32:
			s.Data = append(s.Data, int(num))
		case int16:
			s.Data = append(s.Data, int(num))
		case int8:
			s.Data = append(s.Data, int(num))
		case uint:
			s.Data = append(s.Data, int(num))
		case uint64:
			s.Data = append(s.Data, int(num))
		case uint32:
			s.Data = append(s.Data, int(num))
		case uint16:
			s.Data = append(s.Data, int(num))
		case uint8:
			s.Data = append(s.Data, int(num))
		default:
			return fmt.Errorf("unsupported value in array")
		}
	}

	return nil
}

type FloatSlice struct {
	Data []float64
}

func (s *FloatSlice) Scan(src any) error {
	arr, ok := src.([]interface{})
	if !ok {
		return fmt.Errorf("unsupported value")
	}

	if len(arr) == 0 {
		return nil
	}

	for _, elem := range arr {
		switch num := elem.(type) {
		case int:
			s.Data = append(s.Data, float64(num))
		case int64:
			s.Data = append(s.Data, float64(num))
		case int32:
			s.Data = append(s.Data, float64(num))
		case int16:
			s.Data = append(s.Data, float64(num))
		case int8:
			s.Data = append(s.Data, float64(num))
		case uint:
			s.Data = append(s.Data, float64(num))
		case uint64:
			s.Data = append(s.Data, float64(num))
		case uint32:
			s.Data = append(s.Data, float64(num))
		case uint16:
			s.Data = append(s.Data, float64(num))
		case uint8:
			s.Data = append(s.Data, float64(num))
		case float32:
			s.Data = append(s.Data, float64(num))
		case float64:
			s.Data = append(s.Data, num)
		default:
			return fmt.Errorf("unsupported value in array")
		}
	}

	return nil
}
