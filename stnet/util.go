package stnet

import (
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

func randInt() int {
	return rand.Int()
}
func randUInt32() uint32 {
	return rand.Uint32()
}

func sysWaitSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	signal.Ignore(syscall.SIGPIPE)

	select {
	case <-c:
	}
}
