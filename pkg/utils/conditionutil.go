package utils

// IfThen evaluates a condition, if true returns the parameters otherwise nil
func IfThen(condition bool, a any) any {
	if condition {
		return a
	}
	return nil
}

// IfThenElse evaluates a condition, if true returns the first parameter otherwise the second
func IfThenElse(condition bool, a any, b any) any {
	if condition {
		return a
	}
	return b
}

// DefaultIfNil checks if the value is nil, if true returns the default value otherwise the original
func DefaultIfNil(value any, defaultValue any) any {
	if value != nil {
		return value
	}
	return defaultValue
}

// FirstNonNil returns the first non nil parameter
func FirstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
