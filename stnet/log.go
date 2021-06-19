package stnet

import (
	"github.com/gotask/gost/stlog"
)

var (
	SysLog *stlog.Logger
)

func init() {
	SysLog = stlog.NewLogger()
	SysLog.SetFileLevel(stlog.CRITICAL, "net_system.log", 1024*1024*1024, 0, 1)
	SysLog.SetTermLevel(stlog.CLOSE)
}

func LogOpen() {
	SysLog.SetLevel(stlog.SYSTEM)
	SysLog.SetTermLevel(stlog.CLOSE)
}

func LogClose() {
	SysLog.Close()
}
