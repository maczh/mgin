// Package main 演示内存缓存与磁盘缓存。
// 运行：go run ./cache
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/maczh/mgin/v2/pkg/cache"
)

func main() {
	mem := cache.OnMemCache("demo")
	mem.Set("user:1", "tom", 2*time.Second)
	if v, ok := mem.Value("user:1"); ok {
		fmt.Println("mem hit:", v)
	}
	time.Sleep(2100 * time.Millisecond)
	fmt.Println("mem after ttl exists:", mem.IsExist("user:1"))

	dir, _ := os.MkdirTemp("", "mgin-cache")
	defer os.RemoveAll(dir)

	disk := cache.OnDiskCache(filepath.Join(dir, "demo.db"))
	disk.Set("order:9", map[string]any{"amount": 100}, time.Minute)
	if v, ok := disk.Value("order:9"); ok {
		fmt.Println("disk hit:", v)
	}
	disk.Delete("order:9")
	fmt.Println("disk after delete exists:", disk.IsExist("order:9"))

	cache.CloseCache()
}
