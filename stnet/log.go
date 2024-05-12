package stnet

import (
	"sync/atomic"
)

var (
	sysLog *Logger
	user   int32
)

func init() {
	sysLog = NewLogger()
	sysLog.SetFileLevel(SYSTEM, "net_system.log", 1024*1024*1024, 0, 1)
	sysLog.SetTermLevel(CLOSE)
}

func logOpen() {
	atomic.AddInt32(&user, 1)
}

func logClose() {
	u := atomic.AddInt32(&user, -1)
	if u <= 0 {
		sysLog.Close()
	}
}
