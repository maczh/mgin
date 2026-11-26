// Package safego provides a dependency-free panic boundary for background work.
package safego

import (
	"log/slog"

	goerr "github.com/go-errors/errors"
)

// Run starts fn asynchronously and recovers panics at the goroutine boundary.
func Run(fn func()) {
	if fn == nil {
		return
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := goerr.Wrap(recovered, 2)
				slog.Error("goroutine panic recovered", "panic", err.Error(), "stack", err.Stack())
			}
		}()
		fn()
	}()
}
