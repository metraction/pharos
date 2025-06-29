package utils

import "time"

// parse string, return time.Duration or default
func DurationOr(input string, defval time.Duration) time.Duration {
	dt, err := time.ParseDuration(input)
	if err != nil {
		return defval
	}
	return dt
}

// parse string, return time.Time or defauls
func DateStrOr(input string, defval time.Time) time.Time {
	if t, err := time.Parse("2006-01-02", input); err == nil {
		return t
	}
	return defval
}
