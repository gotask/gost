// mmap_unix.go
// +build darwin dragonfly freebsd linux openbsd solaris netbsd

package stmmap

import (
	"os"
	"syscall"
	"unsafe"
)

func (mp *mmap) Init(fd int, offset int64, length int) error {
	data, err := syscall.Mmap(fd, offset, length, syscall.PROT_WRITE|syscall.PROT_READ, syscall.MAP_SHARED|syscall.MAP_POPULATE)
	if err != nil {
		return err
	}

	mp.data = data
	mp.size = uint32(length)
	return nil
}

func (mp *mmap) Unmap() error {
	sysErr := syscall.Munmap(mp.data)
	if sysErr != nil {
		return os.NewSyscallError("SYS_Munmap", sysErr)
	}
	mp.reset()
	return nil
}

func (mp *mmap) Flush() error {
	_, _, sysErr := syscall.Syscall(syscall.SYS_MSYNC,
		uintptr(unsafe.Pointer(&mp.data[0])), uintptr(mp.size), uintptr(syscall.MS_SYNC))
	if sysErr != 0 {
		return os.NewSyscallError("SYS_MSYNC", sysErr)
	}

	return nil
}

func (mp *mmap) Lock() error {
	_, _, sysErr := syscall.Syscall(syscall.SYS_MLOCK,
		uintptr(unsafe.Pointer(&mp.data[0])), uintptr(mp.size), 0)
	if sysErr != 0 {
		return os.NewSyscallError("SYS_MLOCK", sysErr)
	}

	return nil
}

func (mp *mmap) Unlock() error {
	_, _, sysErr := syscall.Syscall(syscall.SYS_MUNLOCK,
		uintptr(unsafe.Pointer(&mp.data[0])), uintptr(mp.size), 0)
	if sysErr != 0 {
		return os.NewSyscallError("SYS_MUNLOCK", sysErr)
	}

	return nil
}

func (mp *mmap) Advise(advice int) error {
	_, _, err := syscall.Syscall(syscall.SYS_MADVISE,
		uintptr(unsafe.Pointer(&mp.data[0])), uintptr(mp.size), uintptr(advice))
	if err != 0 {
		return err
	}

	return nil
}
