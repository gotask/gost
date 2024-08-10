package stnet

import (
	"sync/atomic"
	"time"
)

var (
	sysLog  *Logger
	logUser int32
)

func init() {
	sysLog = NewLogger()
	sysLog.SetFileLevel(SYSTEM, "net_system.log", 1024*1024*1024, 0, 1)
	sysLog.SetTermLevel(CLOSE)
}

func logOpen() {
	atomic.AddInt32(&logUser, 1)

	go func() {
		for {
			time.Sleep(time.Hour)
			if atomic.LoadInt32(&logUser) > 0 {
				logMetric()
				resetMetricInfo()
			}
		}
	}()
}

func logClose() {
	u := atomic.AddInt32(&logUser, -1)
	if u <= 0 {
		sysLog.Close()
	}
}
