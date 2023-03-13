package stnet

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Server struct {
	loopmsec uint32
	//threadid->Services
	services  map[int][]*Service
	wg        sync.WaitGroup
	isClose   *Closer
	netSignal []chan int

	nameServices map[string]*Service

	ProcessorThreadsNum int //number of threads in server.
}

func NewServer(loopmsec uint32, threadnum int) *Server {
	if threadnum <= 0 {
		threadnum = 1
	}

	if loopmsec == 0 {
		loopmsec = 1
	}
	svr := &Server{}
	svr.ProcessorThreadsNum = threadnum
	svr.loopmsec = loopmsec
	svr.services = make(map[int][]*Service)
	svr.isClose = NewCloser(false)
	svr.nameServices = make(map[string]*Service)

	svr.netSignal = make([]chan int, svr.ProcessorThreadsNum)
	for i := 0; i < svr.ProcessorThreadsNum; i++ {
		svr.netSignal[i] = make(chan int, 1)
	}
	return svr
}

func (svr *Server) newService(name, address string, heartbeat uint32, imp ServiceImp, netSignal *[]chan int, threadId int) (*Service, error) {
	if imp == nil || netSignal == nil {
		return nil, fmt.Errorf("ServiceImp should not be nil")
	}
	msgTh := make([]chan sessionMessage, svr.ProcessorThreadsNum)
	for i := 0; i < svr.ProcessorThreadsNum; i++ {
		msgTh[i] = make(chan sessionMessage, 10240)
	}
	sve := &Service{
		Name:      name,
		imp:       imp,
		messageQ:  msgTh,
		netSignal: netSignal,
		threadId:  threadId,
		svr:       svr,
	}

	if address != "" {
		var (
			lis *Listener
			err error
		)
		network, ipport := parseAddress(address)
		if network == "udp" {
			lis, err = NewUdpListener(ipport, sve, heartbeat)
		} else {
			lis, err = NewListener(ipport, sve, heartbeat)
		}
		if err != nil {
			return nil, err
		}
		sve.Listener = lis
	}

	return sve, nil
}

// AddService must be called before server started.
// address could be null,then you get a service without listen.
// when heartbeat(second)=0,heartbeat will be close.
// call service.NewConnect start a connector
func (svr *Server) AddService(name, address string, heartbeat uint32, imp ServiceImp, threadId int) (*Service, error) {
	threadId = threadId % svr.ProcessorThreadsNum
	s, e := svr.newService(name, address, heartbeat, imp, &svr.netSignal, threadId)
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

// AddJsonService use SendJsonCmd to send message
func (svr *Server) AddJsonService(name, address string, heartbeat uint32, imp JsonService, threadId int) (*Service, error) {
	return svr.AddService(name, address, heartbeat, &ServiceJson{ServiceBase{}, imp}, threadId)
}

// AddRpcService imp:	NewServiceRpc
func (svr *Server) AddRpcService(name, address string, heartbeat uint32, imp *ServiceRpc, threadId int) (*Service, error) {
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
	addr := make([]string, len(proxyaddr))
	copy(addr, proxyaddr)
	s.remoteip = addr
	if len(addr) > 1 {
		weight := make([]int, len(proxyweight))
		copy(weight, proxyweight)
		for i := 1; i < len(weight); i++ {
			weight[i] += weight[i-1]
		}
		s.weight = weight
	}
	_, e = svr.AddService("", address, heartbeat, s, threadId)
	return e
}

/*
func (svr *Server) PushRequest(servicename string, msgid int64, msg interface{}) error {
	if servicename == "" {
		return fmt.Errorf("servicename is null")
	}
	if s, ok := svr.nameServices[servicename]; ok {
		return s.PushRequest(msgid, msg)
	}
	return fmt.Errorf("no service named %s", servicename)
}*/

type CurrentContent struct {
	GoroutineID int
	Sess        *Session
	UserDefined interface{}
	Peer        net.Addr
}

func (svr *Server) Start() error {
	logOpen()

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
		svr.wg.Add(1)
		go func(threadIdx int, ms []*Service, all []*Service) {
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

	for i := 0; i < svr.ProcessorThreadsNum; i++ {
		if _, ok := svr.services[i]; ok {
			continue
		}
		svr.wg.Add(1)
		go func(idx int, ss []*Service) {
			current := &CurrentContent{GoroutineID: idx}
			for !svr.isClose.IsClose() {
				nmsg := 0
				for _, s := range ss {
					nmsg += s.messageThread(current)
				}
				if nmsg == 0 {
					//wait for new message
					<-svr.netSignal[idx]
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
	SysLog.System("network stop~~~~~~")

	//stop logic work
	svr.isClose.Close()
	//wakeup logic thread
	for i := 0; i < svr.ProcessorThreadsNum; i++ {
		select {
		case svr.netSignal[i] <- 1:
		default:
		}
	}
	svr.wg.Wait()
	SysLog.System("logic stop~~~~~~")

	for _, v := range svr.services {
		for _, s := range v {
			s.imp.Destroy()
		}
	}
	SysLog.Debug("server closed~~~~~~")
	logClose()
}
