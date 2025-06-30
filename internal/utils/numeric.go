package utils

import (
	"strconv"
)

// numberic types
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// convert to T or default
func ToNumOr[T Numeric](input string, defval T) T {
	var zero T
	switch any(zero).(type) {
	case int:
		if v, err := strconv.Atoi(input); err == nil {
			return T(v)
		}
		return defval
	case int8:
		if v, err := strconv.ParseInt(input, 10, 8); err == nil {
			return T(v)
		}
		return defval
	case int16:
		if v, err := strconv.ParseInt(input, 10, 16); err == nil {
			return T(v)
		}
		return defval
	case int32:
		if v, err := strconv.ParseInt(input, 10, 32); err == nil {
			return T(v)
		}
		return defval
	case int64:
		if v, err := strconv.ParseInt(input, 10, 64); err == nil {
			return T(v)
		}
		return defval
	case uint:
		if v, err := strconv.ParseUint(input, 10, 0); err == nil {
			return T(v)
		}
		return defval
	case uint8:
		if v, err := strconv.ParseUint(input, 10, 8); err == nil {
			return T(v)
		}
		return defval
	case uint16:
		if v, err := strconv.ParseUint(input, 10, 16); err == nil {
			return T(v)
		}
		return defval
	case uint32:
		if v, err := strconv.ParseUint(input, 10, 32); err == nil {
			return T(v)
		}
		return defval
	case uint64:
		if v, err := strconv.ParseUint(input, 10, 64); err == nil {
			return T(v)
		}
		return defval
	case float32:
		if v, err := strconv.ParseFloat(input, 32); err == nil {
			return T(v)
		}
		return defval
	case float64:
		if v, err := strconv.ParseFloat(input, 64); err == nil {
			return T(v)
		}
		return defval
	default:
		return defval
	}
}
