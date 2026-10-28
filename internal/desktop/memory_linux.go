//go:build linux

package desktop

import (
	"os"
	"strconv"
	"strings"
)

func ResidentMemory() uint64 {
	b, e := os.ReadFile("/proc/self/statm")
	if e != nil {
		return 0
	}
	parts := strings.Fields(string(b))
	if len(parts) < 2 {
		return 0
	}
	pages, _ := strconv.ParseUint(parts[1], 10, 64)
	return pages * uint64(os.Getpagesize())
}
