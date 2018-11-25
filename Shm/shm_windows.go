// shm_windows.go
package Shm

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/gotask/gost/stutil"
)

func (sh *shm) Init(key, size uint32) error {
	name := strconv.FormatUint(uint64(key), 16)

	var fHandle syscall.Handle
	if stutil.FileIsExist(name) {
		if stutil.FileSize(name) != int64(size) {
			return fmt.Errorf("FileIsExist But FileSize NE")
		}
	} else {
		c := make([]byte, size, size)
		e := stutil.FileCreateAndWrite(name, string(c))
		if e != nil {
			return e
		}
	}
	f, e := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if e != nil {
		fHandle = 0
	} else {
		fHandle = syscall.Handle(uintptr(f.Fd()))
		defer f.Close()
	}

	b, err := syscall.UTF16FromString(name)
	if err != nil {
		return os.NewSyscallError("UTF16FromString", err)
	}
	if len(b) == 0 {
		return fmt.Errorf("name cannot be null.")
	}
	h, errno := syscall.CreateFileMapping(fHandle, nil, syscall.PAGE_READWRITE, 0, size, &b[0])
	//fmt.Println(syscall.GetLastError())
	if h == 0 || errno != nil {
		return os.NewSyscallError("CreateFileMapping", errno)
	}
	addr, errno := syscall.MapViewOfFile(h, syscall.FILE_MAP_WRITE, 0, 0, uintptr(size))
	if addr == 0 || errno != nil {
		return os.NewSyscallError("MapViewOfFile", errno)
	}
	/*errno = syscall.VirtualLock(addr, uintptr(size))
	if errno != nil {
		return os.NewSyscallError("VirtualLock", errno)
	}*/

	sh.data = make([]byte, 0, 0)
	sh.size = size
	sh.key = key
	sh.name = name
	sh.h = uintptr(h)
	dh := (*reflect.SliceHeader)(unsafe.Pointer(&sh.data))
	dh.Data = addr
	dh.Len = int(size)
	dh.Cap = dh.Len

	return nil
}

func (sh *shm) Detach() error {
	dh := (*reflect.SliceHeader)(unsafe.Pointer(&sh.data))
	err := syscall.UnmapViewOfFile(dh.Data)
	if err != nil {
		return os.NewSyscallError("UnmapViewOfFile", err)
	}
	err = syscall.CloseHandle(syscall.Handle(sh.h))
	if err != nil {
		return os.NewSyscallError("CloseHandle", err)
	}
	sh.reset()
	return nil
}

func (sh *shm) Delete() error {
	return stutil.FileDelete(sh.name)
}
