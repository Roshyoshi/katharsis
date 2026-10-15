package judge

import (
	"encoding/binary"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// osRelease is the kernel release ("25.5.0"); a macOS update can change
// which sanitizer runtimes work (see probe.go).
func osRelease() string {
	rel, _ := syscall.Sysctl("kern.osrelease")
	return rel
}

// usage reads a live process's memory and CPU time from
// proc_pid_rusage(2), which libSystem exports and ps(1) itself uses. Go has
// no wrapper for it, so this calls it the way golang.org/x/sys/unix calls
// libSystem: a trampoline (sys_darwin.s) resolved by cgo_import_dynamic,
// invoked through syscall.syscall. No cgo, and a call costs under a
// microsecond (measured 0.8 µs on an M4), against about 1.3 ms to fork ps.
//
// The memory is the larger of resident size and physical footprint.
// Footprint also counts pages the compressor has squeezed out of RAM, so a
// runaway allocation keeps growing in it even when resident size stalls
// under memory pressure; resident size counts shared library pages
// footprint leaves out.
func usage(pid int) (mem int64, cpu time.Duration, err error) {
	var ri rusageInfoV0
	_, _, errno := syscall_syscall(libc_proc_pid_rusage_trampoline_addr,
		uintptr(pid), rusageInfoV0Flavor, uintptr(unsafe.Pointer(&ri)))
	if errno != 0 {
		return 0, 0, errno
	}
	mem = int64(max(ri.ResidentSize, ri.PhysFootprint))
	return mem, machTime(ri.UserTime + ri.SystemTime), nil
}

// rusageInfoV0 is struct rusage_info_v0 from <sys/resource.h>. The times
// are in mach absolute time units: nanoseconds on Intel, 24 MHz ticks on
// Apple silicon (see machTime).
type rusageInfoV0 struct {
	UUID             [16]byte
	UserTime         uint64
	SystemTime       uint64
	PkgIdleWkups     uint64
	InterruptWkups   uint64
	Pageins          uint64
	WiredSize        uint64
	ResidentSize     uint64
	PhysFootprint    uint64
	ProcStartAbstime uint64
	ProcExitAbstime  uint64
}

const rusageInfoV0Flavor = 0 // RUSAGE_INFO_V0

// machTime converts mach absolute time units to a duration. The unit's
// frequency is the sysctl hw.tbfrequency (24000000 on Apple silicon,
// 1000000000 on Intel), read once.
func machTime(ticks uint64) time.Duration {
	hz := tbFrequency()
	if hz == 0 {
		return 0 // unknown: the CPU check stays off, wait4's figure decides
	}
	sec, rem := ticks/hz, ticks%hz
	return time.Duration(sec)*time.Second + time.Duration(rem*uint64(time.Second)/hz)
}

var tbFrequency = sync.OnceValue(func() uint64 {
	// A quad sysctl comes back as its 8 little-endian bytes, minus the
	// trailing zero byte Sysctl trims from strings.
	s, err := syscall.Sysctl("hw.tbfrequency")
	if err != nil || len(s) == 0 || len(s) > 8 {
		return 0
	}
	var b [8]byte
	copy(b[:], s)
	return binary.LittleEndian.Uint64(b[:])
})

//go:linkname syscall_syscall syscall.syscall
func syscall_syscall(fn, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno)

var libc_proc_pid_rusage_trampoline_addr uintptr

//go:cgo_import_dynamic libc_proc_pid_rusage proc_pid_rusage "/usr/lib/libSystem.B.dylib"
