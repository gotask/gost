// pprof.go
package stutil

import (
	"os"
	"runtime/pprof"
	"sync"
)

var (
	pprofFiles sync.Map
)

//go tool pprof -svg cpu.pprof > cpu.svg
func PProfStartCPU(p string) error {
	if p == "" {
		p = "cpu.pprof"
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	pprofFiles.Store(p, f)
	if err := pprof.StartCPUProfile(f); err != nil {
		return err
	}
	return nil
}

func PProfStopCPU(p string) {
	pprof.StopCPUProfile()
	if f, ok := pprofFiles.Load(p); ok {
		f.(*os.File).Close()
		pprofFiles.Delete(p)
	}
}

func PProfSaveMem(p string) error {
	if p == "" {
		p = "mem.pprof"
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return err
	}
	return nil
}

// goroutine block
func PProfSaveBlock(p string) error {
	if p == "" {
		p = "block.pprof"
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := pprof.Lookup("block").WriteTo(f, 0); err != nil {
		return err
	}
	return nil
}
