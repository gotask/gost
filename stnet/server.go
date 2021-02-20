package stnet

import (
	"fmt"
	"net"
	"sync"
	"time"
)

//number of threads in server.
var (
	ProcessorThreadsNum = 128
)

type Server struct {
	name     string
	loopmsec uint32
	//threadid->Services
	services  map[int][]*Service
	wg        sync.WaitGroup
	isClose   *Closer
	netSignal []chan int

	nameServices map[string]*Service
}

func NewServer(name string, loopmsec uint32) *Server {
	//init log
	NewSysLog()

	if loopmsec == 0 {
		loopmsec = 1
	}
	svr := &Server{}
	svr.name = name
	svr.loopmsec = loopmsec
	svr.services = make(map[int][]*Service)
	svr.isClose = NewCloser(false)
	svr.nameServices = make(map[string]*Service)

	svr.netSignal = make([]chan int, ProcessorThreadsNum)
	for i := 0; i < ProcessorThreadsNum; i++ {
		svr.netSignal[i] = make(chan int, 1)
	}
	return svr
}

//must be called before server started.
//address could be null,then you get a service without listen.
//when heartbeat(second)=0,heartbeat will be close.
//call service.NewConnect start a connector
func (svr *Server) AddService(name, address string, heartbeat uint32, imp ServiceImp, threadId int) (*Service, error) {
	threadId = threadId % ProcessorThreadsNum
	s, e := newService(name, address, heartbeat, imp, &svr.netSignal, threadId)
	if e != nil {
		return nil, e
	}
	svr.services[threadId] = append(svr.services[threadId], s)
	if name != "" {
		svr.nameServices[name] = s
	}
	return s, e
}

func (svr *Server) AddLoopService(name string, imp LoopService, threadId int) (*Service, error) {
	return svr.AddService(name, "", 0, &ServiceLoop{ServiceBase{}, imp}, threadId)
}

func (svr *Server) AddEchoService(name, address string, heartbeat uint32, threadId int) (*Service, error) {
	return svr.AddService(name, address, heartbeat, &ServiceEcho{}, threadId)
}

func (svr *Server) AddHttpService(name, address string, heartbeat uint32, imp HttpService, threadId int) (*Service, error) {
	return svr.AddService(name, address, heartbeat, &ServiceHttp{ServiceBase{}, imp}, threadId)
}

func (svr *Server) AddSpbRpcService(name, address string, heartbeat uint32, imp *ServiceRpc, threadId int) (*Service, error) {
	return svr.AddService(name, address, heartbeat, imp, threadId)
}

func (svr *Server) AddTcpProxyService(address string, heartbeat uint32, threadId int, proxyaddr []string, proxyweight []int) error {
	if len(proxyaddr) > 1 && len(proxyaddr) != len(proxyweight) {
		return fmt.Errorf("error proxy param")
	}
	c, e := svr.AddService("", "", 0, &ServiceProxyC{}, threadId)
	if e != nil {
		return e
	}
	s := &ServiceProxyS{}
	s.remote = c
	addr := make([]string, len(proxyaddr), len(proxyaddr))
	copy(addr, proxyaddr)
	s.remoteip = addr
	if len(addr) > 1 {
		weight := make([]int, len(proxyweight), len(proxyweight))
		copy(weight, proxyweight)
		for i := 1; i < len(weight); i++ {
			weight[i] += weight[i-1]
		}
		s.weight = weight
	}
	_, e = svr.AddService("", address, heartbeat, s, threadId)
	return e
}

func (svr *Server) PushRequest(servicename string, msgid int64, msg interface{}) error {
	if servicename == "" {
		return fmt.Errorf("servicename is null")
	}
	if s, ok := svr.nameServices[servicename]; ok {
		return s.PushRequest(msgid, msg)
	}
	return fmt.Errorf("no service named %s", servicename)
}

type CurrentContent struct {
	GoroutineID int
	Sess        *Session
	UserDefined interface{}
	Peer        net.Addr
}

func (svr *Server) Start() error {
	allServices := make([]*Service, 0)
	for _, v := range svr.services {
		allServices = append(allServices, v...)
		for _, s := range v {
			if !s.imp.Init() {
				return fmt.Errorf(s.Name + " init failed!")
			}
		}
	}

	for k, v := range svr.services {
		go func(threadIdx int, ms []*Service, all []*Service) {
			svr.wg.Add(1)

			current := &CurrentContent{GoroutineID: threadIdx}
			lastLoopTime := time.Now()
			needD := time.Duration(svr.loopmsec) * time.Millisecond
			for !svr.isClose.IsClose() {
				now := time.Now()

				if now.Sub(lastLoopTime) >= needD {
					lastLoopTime = now
					for _, s := range ms {
						s.loop() //service loop
					}
				}

				//processing message of messageQ[threadIdx]
				for _, s := range all {
					s.messageThread(current)
				}

				subD := now.Sub(lastLoopTime)
				if subD < needD {
					to := time.NewTimer(needD - subD)
					select { //wait for new message
					case <-svr.netSignal[threadIdx]:
					case <-to.C:
					}
					to.Stop()
				}
			}
			SysLog.System("%d thread quit.", threadIdx)
			svr.wg.Done()
		}(k, v, allServices)
	}

	for i := 0; i < ProcessorThreadsNum; i++ {
		if _, ok := svr.services[i]; ok {
			continue
		}
		go func(idx int, ss []*Service) {
			svr.wg.Add(1)

			current := &CurrentContent{GoroutineID: idx}
			for !svr.isClose.IsClose() {
				nmsg := 0
				for _, s := range ss {
					nmsg += s.messageThread(current)
				}
				if nmsg == 0 {
					select { //wait for new message
					case <-svr.netSignal[idx]:
					}
				}
			}
			SysLog.System("%d thread quit.", idx)
			svr.wg.Done()
		}(i, allServices)
	}
	SysLog.Debug("server start~~~~~~")
	return nil
}

func (svr *Server) Stop() {
	//stop network
	for _, v := range svr.services {
		for _, s := range v {
			s.destroy()
		}
	}

	//stop logic work
	svr.isClose.Close()
	//wakeup logic thread
	for i := 0; i < ProcessorThreadsNum; i++ {
		select {
		case svr.netSignal[i] <- 1:
		default:
		}
	}

	svr.wg.Wait()
	for _, v := range svr.services {
		for _, s := range v {
			s.imp.Destroy()
		}
	}
	SysLog.Debug("server closed~~~~~~")
	SysLog.Close()
}
