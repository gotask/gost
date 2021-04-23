// app_windows.go
package stutil

import "os/exec"

func SysDaemon() {
}

func SysLock() {
}

func Exe(cmddir, cmdstr string) string {
	cmd := exec.Command("cmd", "/C", cmdstr)
	cmd.Dir = cmddir
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		return string(out)
	}
	if err != nil {
		return err.Error()
	}

	return ""
}
