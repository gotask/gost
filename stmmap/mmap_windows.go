// mmap_windows.go
package stmmap

import (
	_ "fmt"
	"os"
	"reflect"
	"syscall"
	"unsafe"
)

func (mp *mmap) Init(fd int, offset int64, length int) error {
	var fHandle syscall.Handle
	fHandle = syscall.Handle(uintptr(fd))

	end := int64(length) + offset
	h, errno := syscall.CreateFileMapping(fHandle, nil, syscall.PAGE_READWRITE, uint32(uint64(end)>>32), uint32(uint64(end)&0xffffffff), nil)
	if h == 0 || errno != nil {
		//fmt.Println(syscall.GetLastError())
		return os.NewSyscallError("CreateFileMapping", errno)
	}
	addr, errno := syscall.MapViewOfFile(h, syscall.FILE_MAP_WRITE, uint32(uint64(offset)>>32), uint32(offset&0xffffffff), uintptr(length))
	if addr == 0 || errno != nil {
		//fmt.Println(syscall.GetLastError())
		return os.NewSyscallError("MapViewOfFile", errno)
	}
	/*errno = syscall.VirtualLock(addr, uintptr(length))
	if errno != nil {
		return os.NewSyscallError("VirtualLock", errno)
	}*/

	mp.data = make([]byte, 0, 0)
	mp.size = uint32(length)
	mp.h = uintptr(h)
	dh := (*reflect.SliceHeader)(unsafe.Pointer(&mp.data))
	dh.Data = addr
	dh.Len = int(length)
	dh.Cap = dh.Len

	return nil
}

func (mp *mmap) Unmap() error {
	dh := (*reflect.SliceHeader)(unsafe.Pointer(&mp.data))
	err := syscall.UnmapViewOfFile(dh.Data)
	if err != nil {
		return os.NewSyscallError("UnmapViewOfFile", err)
	}
	err = syscall.CloseHandle(syscall.Handle(mp.h))
	if err != nil {
		return os.NewSyscallError("CloseHandle", err)
	}
	mp.reset()
	return nil
}

func (mp *mmap) Flush() error {
	dh := (*reflect.SliceHeader)(unsafe.Pointer(&mp.data))
	err := syscall.FlushViewOfFile(dh.Data, uintptr(mp.size))
	if err != nil {
		return os.NewSyscallError("FlushFileBuffers", err)
	}
	return nil
}

// Lock locks all the mapped memory to RAM, preventing the pages from swapping out
func (mp *mmap) Lock() error {
	dh := (*reflect.SliceHeader)(unsafe.Pointer(&mp.data))
	err := syscall.VirtualLock(dh.Data, uintptr(mp.size))
	if err != nil {
		return os.NewSyscallError("VirtualLock", err)
	}
	return nil
}

// Unlock unlocks the mapped memory from RAM, enabling swapping out of RAM if required
func (mp *mmap) Unlock() error {
	dh := (*reflect.SliceHeader)(unsafe.Pointer(&mp.data))
	err := syscall.VirtualUnlock(dh.Data, uintptr(mp.size))
	if err != nil {
		return os.NewSyscallError("VirtualUnlock", err)
	}
	return nil
}
