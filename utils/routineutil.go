package utils

import (
	"bytes"
	"runtime"
	"strconv"
)

func GetGoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	if i := bytes.IndexByte(b, ' '); i >= 0 {
		b = b[:i]
	} else {
		return 0
	}
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
