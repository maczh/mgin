// Package safego provides the dependency-free goroutine boundary used by utils
// and packages that sit below utils in the logging dependency graph.
package safego

import (
	"fmt"

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
				reset := string([]byte{27, 91, 48, 109})
				fmt.Printf("[SafeGo] panic recovered:\n\n%s%s\n\n%s", goErr.Error(), goErr.Stack(), reset)
			}
		}()
		fn()
	}()
}
