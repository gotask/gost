package stnet

import (
	"github.com/gotask/gost/stlog"
)

var (
	SysLog *stlog.Logger
)

func init() {
	SysLog = stlog.NewLogger()
	SysLog.SetFileLevel(stlog.INFO, "net_system.log", 1024*1024*1024, 1, 30) //one file 1G, 30 files max one day
}
