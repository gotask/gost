package stutil

import (
	"os"
	"os/signal"
	"syscall"
)

func SysWaitSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	signal.Ignore(syscall.SIGPIPE)

	select {
	case <-c:
	}
}
