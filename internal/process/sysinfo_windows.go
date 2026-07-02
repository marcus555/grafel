//go:build windows

package process

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var (
	modkernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modkernel32.NewProc("GlobalMemoryStatusEx")
)

// TotalMemoryMB returns total physical memory on Windows via
// kernel32!GlobalMemoryStatusEx. Returns 0 on failure so callers keep their
// existing conservative fallback.
func TotalMemoryMB() int64 {
	var status memoryStatusEx
	status.Length = uint32(unsafe.Sizeof(status))
	r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if r1 == 0 || status.TotalPhys == 0 {
		return 0
	}
	return int64(status.TotalPhys / 1024 / 1024)
}
