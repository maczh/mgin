// Package safego provides the dependency-free goroutine boundary used by utils
// and packages that sit below utils in the logging dependency graph.
package safego

import (
	"log/slog"

	"github.com/go-errors/errors"
)

// Run starts fn in a goroutine and recovers any panic raised by fn.
func Run(fn func()) {
	if fn == nil {
		return
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				goErr := errors.Wrap(recovered, 2)
				slog.Error("goroutine panic recovered", "panic", goErr.Error(), "stack", goErr.Stack())
			}
		}()
		fn()
	}()
}
