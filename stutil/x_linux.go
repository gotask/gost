// x_linux.go
package stutil

// #include <unistd.h>
import "C"
import (
	"fmt"
	"os"
	"syscall"
)

func SysDaemon() error {
	if os.Getenv("GO_DAEMON2") != "" {
		syscall.Umask(0)
		return nil
	}
	if os.Getenv("GO_DAEMON1") == "" {
		os.Setenv("GO_DAEMON1", "1")
	} else {
		os.Setenv("GO_DAEMON2", "2")
	}

	files := make([]*os.File, 3, 6)
	nullDev, e := os.OpenFile("/dev/null", 0, 0)
	if e != nil {
		return e
	}
	files[0], files[1], files[2] = nullDev, nullDev, nullDev
	//files[0], files[1], files[2] = os.Stdin, os.Stdout, os.Stderr

	dir, _ := os.Getwd()
	sysattrs := syscall.SysProcAttr{Setsid: true}
	attrs := os.ProcAttr{Dir: dir, Env: os.Environ(), Files: files, Sys: &sysattrs}

	proc, err := os.StartProcess(os.Args[0], os.Args, &attrs)
	if err != nil {
		return err
	}
	proc.Release()
	os.Exit(0)
	return nil
}

func SysLock() {
	lockfile := os.Args[0] + ".lock"
	fp, err := os.OpenFile(lockfile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: can not open lock file: %s\n", lockfile)
		os.Exit(1)
	}

	err = syscall.Flock(int(fp.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s is running\n", os.Args[0])
		os.Exit(1)
	}
}
