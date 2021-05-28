package stnet

import (
	"github.com/gotask/gost/stlog"
)

var (
	SysLog *stlog.Logger
)

func init() {
	SysLog = stlog.NewLogger()
	SysLog.SetFileLevel(stlog.SYSTEM, "net_system.log", 1024*1024*1024, 0, 1)
	SysLog.SetTermLevel(stlog.CLOSE)
}
