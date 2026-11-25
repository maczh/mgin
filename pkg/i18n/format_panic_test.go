package i18n

import (
	"fmt"
	"testing"
)

func TestFormatSupportsNumericTypes(t *testing.T) {
	values := []any{float32(1.25), float64(2.5), int8(1), int16(2), int32(3), int64(4), uint(5), uint8(6), uint16(7), uint32(8), uint64(9), uintptr(10)}
	for _, value := range values {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("Format panicked for %T: %v", value, recovered)
				}
			}()
			if got := Format("{}", value); got == "{}" {
				t.Fatalf("value was not formatted: %T", value)
			}
		})
	}
}
