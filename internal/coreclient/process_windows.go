//go:build windows

package coreclient

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type shellExecuteInfo struct {
	Size       uint32
	Mask       uint32
	Window     windows.Handle
	Verb       *uint16
	File       *uint16
	Parameters *uint16
	Directory  *uint16
	Show       int32
	Instance   windows.Handle
	IDList     uintptr
	Class      *uint16
	ClassKey   windows.Handle
	HotKey     uint32
	Icon       windows.Handle
	Process    windows.Handle
}

func startChild(path, address string, stderr io.Writer, elevated bool) (*child, error) {
	if !elevated || windows.GetCurrentProcessToken().IsElevated() {
		return commandChild(exec.Command(path, address), stderr)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	params, _ := windows.UTF16PtrFromString(windows.EscapeArg(address))
	info := shellExecuteInfo{Mask: 0x40 | 0x100, Verb: verb, File: file, Parameters: params, Show: 0}
	info.Size = uint32(unsafe.Sizeof(info))
	proc := windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")
	ok, _, e := proc.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return nil, fmt.Errorf("authorize TUN core: %w", e)
	}
	var mu sync.Mutex
	handle := info.Process
	c := &child{done: make(chan struct{})}
	c.kill = func() error {
		mu.Lock()
		defer mu.Unlock()
		if handle == 0 {
			return nil
		}
		return windows.TerminateProcess(handle, 1)
	}
	go func() {
		_, _ = windows.WaitForSingleObject(info.Process, windows.INFINITE)
		mu.Lock()
		_ = windows.CloseHandle(handle)
		handle = 0
		mu.Unlock()
		close(c.done)
	}()
	return c, nil
}
