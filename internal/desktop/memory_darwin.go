//go:build darwin

package desktop

import (
	"os"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var memoryOnce sync.Once
var procPIDInfo func(int32, int32, uint64, unsafe.Pointer, int32) int32

func ResidentMemory() uint64 {
	memoryOnce.Do(func() {
		lib, e := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_LAZY|purego.RTLD_LOCAL)
		if e != nil {
			return
		}
		address, e := purego.Dlsym(lib, "proc_pidinfo")
		if e != nil {
			return
		}
		purego.RegisterFunc(&procPIDInfo, address)
	})
	if procPIDInfo == nil {
		return 0
	}
	var info struct {
		Virtual, Resident, User, System, ThreadsUser, ThreadsSystem uint64
		Details                                                     [12]int32
	}
	if procPIDInfo(int32(os.Getpid()), 4, 0, unsafe.Pointer(&info), int32(unsafe.Sizeof(info))) <= 0 {
		return 0
	}
	return info.Resident
}
