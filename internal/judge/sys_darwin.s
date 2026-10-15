// Trampolines for the libproc calls in sys_darwin.go. The same text
// assembles for arm64 and amd64.

#include "textflag.h"

TEXT libc_proc_pid_rusage_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_proc_pid_rusage(SB)
GLOBL	·libc_proc_pid_rusage_trampoline_addr(SB), RODATA, $8
DATA	·libc_proc_pid_rusage_trampoline_addr(SB)/8, $libc_proc_pid_rusage_trampoline<>(SB)

TEXT libc_proc_listpids_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_proc_listpids(SB)
GLOBL	·libc_proc_listpids_trampoline_addr(SB), RODATA, $8
DATA	·libc_proc_listpids_trampoline_addr(SB)/8, $libc_proc_listpids_trampoline<>(SB)

TEXT libc_proc_pidpath_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_proc_pidpath(SB)
GLOBL	·libc_proc_pidpath_trampoline_addr(SB), RODATA, $8
DATA	·libc_proc_pidpath_trampoline_addr(SB)/8, $libc_proc_pidpath_trampoline<>(SB)
