package judge

import (
	"debug/elf"
	"debug/macho"
	"sync"
)

// staticCache maps a binary's path to its staticData. Build paths are
// content-addressed, so a path's answer never changes.
var staticCache sync.Map

// staticData is how much writable static data bin maps at load: its
// globals, initialized or zero-filled. macOS counts an untouched page of a
// huge global array as nothing, so `int a[300000000];` (1.2 GB) runs here
// under a 256 MB limit, while judges that count committed memory
// (Codeforces) or cap the address space (RLIMIT_AS) reject it before it
// starts. 0 when bin is neither Mach-O nor ELF or cannot be read.
func staticData(bin string) int64 {
	if v, ok := staticCache.Load(bin); ok {
		return v.(int64)
	}
	n := machoData(bin)
	if n < 0 {
		n = elfData(bin)
	}
	n = max(n, 0)
	staticCache.Store(bin, n)
	return n
}

// machoData sums the writable segments (__DATA and its kin; __bss and
// __common are zero-fill sections inside __DATA), -1 for a file that is
// not Mach-O.
func machoData(bin string) int64 {
	f, err := macho.Open(bin)
	if err != nil {
		return -1
	}
	defer func() { _ = f.Close() }()
	const vmProtWrite = 2
	var n int64
	for _, l := range f.Loads {
		if s, ok := l.(*macho.Segment); ok && s.Prot&vmProtWrite != 0 {
			n += int64(s.Memsz)
		}
	}
	return n
}

// elfData sums the writable PT_LOAD segments (.data, .bss), -1 for a file
// that is not ELF.
func elfData(bin string) int64 {
	f, err := elf.Open(bin)
	if err != nil {
		return -1
	}
	defer func() { _ = f.Close() }()
	var n int64
	for _, p := range f.Progs {
		if p.Type == elf.PT_LOAD && p.Flags&elf.PF_W != 0 {
			n += int64(p.Memsz)
		}
	}
	return n
}
