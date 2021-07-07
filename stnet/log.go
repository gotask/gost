package stnet

import (
	"github.com/gotask/gost/stlog"
	"sync/atomic"
)

var (
	SysLog *stlog.Logger
	user   int32
)

func init() {
	SysLog = stlog.NewLogger()
	SysLog.SetFileLevel(stlog.SYSTEM, "net_system.log", 1024*1024*1024, 0, 1)
	SysLog.SetTermLevel(stlog.CLOSE)
}

func logOpen() {
	atomic.AddInt32(&user, 1)
}

func logClose() {
	u := atomic.AddInt32(&user, -1)
	if u <= 0 {
		SysLog.Close()
	}
}
