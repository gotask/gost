// shm_unix.go
// +build darwin dragonfly freebsd linux openbsd solaris netbsd

package Shm

import (
	"os"
	"reflect"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	IPC_CREAT  = 00001000 /* create if key is nonexistent */
	IPC_EXCL   = 00002000 /* fail if key exists */
	IPC_NOWAIT = 00004000 /* return error on wait */

	IPC_RMID = 0 /* remove resource */
	IPC_SET  = 1 /* set ipc_perm options */
	IPC_STAT = 2 /* get ipc_perm options */
	IPC_INFO = 3 /* see ipcs */

)

func (sh *shm) Init(key, size uint32) error {
	shmid, _, sysErr := syscall.RawSyscall(syscall.SYS_SHMGET, uintptr(key), uintptr(size), uintptr(0666))
	if sysErr != 0 {
		shmid, _, sysErr = syscall.RawSyscall(syscall.SYS_SHMGET, uintptr(key), uintptr(size), uintptr(0666|IPC_CREAT|IPC_EXCL))
		if sysErr != 0 {
			return os.NewSyscallError("SYS_SHMGET", sysErr)
		}
	}

	shmAddr, _, sysErr := syscall.RawSyscall(syscall.SYS_SHMAT, shmid, 0, 0)
	if sysErr != 0 {
		return os.NewSyscallError("SYS_SHMAT", sysErr)
	}

	sh.data = make([]byte, 0, 0)
	sh.size = size
	sh.key = key
	sh.name = strconv.FormatUint(uint64(key), 16)
	sh.h = shmid
	dh := (*reflect.SliceHeader)(unsafe.Pointer(&sh.data))
	dh.Data = shmAddr
	dh.Len = int(size)
	dh.Cap = dh.Len

	return nil
}

func (sh *shm) Detach() error {
	dh := (*reflect.SliceHeader)(unsafe.Pointer(&sh.data))
	_, _, sysErr := syscall.RawSyscall(syscall.SYS_SHMDT, dh.Data, 0, 0)
	if sysErr != 0 {
		return os.NewSyscallError("SYS_SHMDT", sysErr)
	}
	sh.reset()
	return nil
}

func (sh *shm) Delete() error {
	_, _, sysErr := syscall.RawSyscall(syscall.SYS_SHMCTL, sh.h, IPC_RMID, 0)
	if sysErr != 0 {
		return os.NewSyscallError("SYS_SHMCTL", sysErr)
	}
	return nil
}
