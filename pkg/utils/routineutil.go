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
	if end := bytes.IndexByte(b, ' '); end >= 0 {
		b = b[:end]
	}
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
