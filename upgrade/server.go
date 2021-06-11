package upgrade

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gotask/gost/stnet"
)

var (
	s  *stnet.Server
	sg chan string = make(chan string, 1)
)

func StartServer(address string) error {
	s = stnet.NewServer("upgradeserver", 100, 8)
	s.AddRpcService("upgrade", address, 0, stnet.NewServiceRpc(&UpgradeServer{}), 0)
	e := s.Start()
	if e != nil {
		return e
	}

	versionMoniter()

	return nil
}

type OnClose func()

func WaitStopSignal(oc OnClose) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	signal.Ignore(syscall.SIGPIPE)

	var cmd string
	select {
	case <-c:
	case cmd = <-sg:
	}

	if oc != nil {
		oc()
	}

	stop(cmd)
}

func stop(cmd string) {
	if s != nil {
		s.Stop()
	}
	if c != nil {
		c.Stop()
	}
	if cmd != "" {
		LOG.Info("exe %s", cmd)
	}
	LOG.Close()
	if cmd != "" {
		exe(cmd)
	}
}
