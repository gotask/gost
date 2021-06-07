// ssproxy project main.go
package main

import "github.com/gotask/gost/stutil"

func main() {
	stutil.SysDaemon()

	defer LOG.Close()

	e := Init()
	if e != nil {
		LOG.Error(e.Error())
		return
	}

	e = proxy.Start()
	if e != nil {
		LOG.Error(e.Error())
		return
	}

	stutil.SysWaitSignal()

	proxy.Stop()
}
