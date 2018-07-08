package stnet

import (
	"github.com/gotask/gost/stlog"
)

var (
	SysLog *stlog.Logger
)

func init() {
	SysLog = stlog.NewLogger()
	SysLog.SetFileLevel(stlog.DEBUG, "st_system.log", 1024*1024*100, 1, 30) //one file 100M, 30 files max one day
}
