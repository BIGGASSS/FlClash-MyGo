//go:build windows

package desktop

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var processMemory = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

func ResidentMemory() uint64 {
	var counters struct {
		Size, PageFaults                                                                                    uint32
		Peak, Working, QuotaPeakPaged, QuotaPaged, QuotaPeakNonPaged, QuotaNonPaged, Pagefile, PeakPagefile uintptr
	}
	counters.Size = uint32(unsafe.Sizeof(counters))
	ok, _, _ := processMemory.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&counters)), uintptr(counters.Size))
	if ok == 0 {
		return 0
	}
	return uint64(counters.Working)
}
