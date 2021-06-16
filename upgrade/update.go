package upgrade

import (
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/gotask/gost/stutil"
)

func exe(run string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", run)
		//cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	} else {
		cmd = exec.Command("/bin/sh", "-c", run)
	}
	cmd.Stderr = nil //os.Stderr
	cmd.Stdin = nil  //os.Stdin
	cmd.Stdout = nil //os.Stdout
	e := cmd.Start()
	if e != nil {
		return e
	}
	e = cmd.Process.Release()
	if e != nil {
		return e
	}
	return nil
}

func Update() {
	defer LOG.Close()
	if len(os.Args) < 2 {
		LOG.Error("update error: param need name of bin")
		return
	}
	bin := os.Args[1]
	if !stutil.FileIsExist(bin + ".new") {
		LOG.Error("update error: cannot find file %s", bin+".new")
		return
	}

	time.Sleep(time.Second) //wait for close

	e := stutil.FileDelete(bin)
	if e != nil {
		LOG.Error("update error: cannot del file %s, %s", bin, e.Error())
		return
	}
	e = stutil.FileMove(bin+".new", bin)
	if e != nil {
		LOG.Error("update error: cannot move file %s, %s", bin, e.Error())
		return
	}
	if runtime.GOOS != "windows" {
		os.Chmod(bin, 0777)
	}

	var cmd string
	for i, s := range os.Args {
		if i == 0 {
			continue
		}
		cmd += " " + s
	}
	e = exe(cmd[1:])
	if e != nil {
		LOG.Error("update error: exe failed %s, %s", cmd, e.Error())
		return
	}
	LOG.Info("update success: %s", cmd)
}
