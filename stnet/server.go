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

// NewServer threadnum is the number of the server's running thread.
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

func (svr *Server) newService(name, address string, heartbeat uint32, imp ServiceImp, netSignal *[]chan int, threadId int) *Service {
	if imp == nil || netSignal == nil {
		panic(fmt.Errorf("ServiceImp should not be nil"))
	}
	msgTh := make([]chan sessionMessage, svr.ProcessorThreadsNum)
	for i := 0; i < svr.ProcessorThreadsNum; i++ {
		msgTh[i] = make(chan sessionMessage, 10240)
	}
	sve := &Service{
		Name:                name,
		imp:                 imp,
		messageQ:            msgTh,
		netSignal:           netSignal,
		threadId:            threadId,
		ProcessorThreadsNum: svr.ProcessorThreadsNum,
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
			panic(err)
		}
		sve.Listener = lis
	}

	return sve
}

// AddService must be called before server started.
// address could be null,then you get a service without listen; address could be udp,example udp:127.0.0.1:6060,default use tcp(127.0.0.1:6060)
// when heartbeat (second)=0,heartbeat will be close.
// threadId should be between 1-ProcessorThreadsNum.
// call Service.NewConnect start a connector
func (svr *Server) AddService(name, address string, heartbeat uint32, imp ServiceImp, threadId int) *Service {
	if threadId < 0 || threadId > svr.ProcessorThreadsNum {
		sysLog.Error("threadId should be 1-%d,now=%d", svr.ProcessorThreadsNum, threadId)
		if threadId <= 0 {
			threadId = 1
		}
	}
	threadId = threadId % svr.ProcessorThreadsNum
	s := svr.newService(name, address, heartbeat, imp, &svr.netSignal, threadId)
	svr.services[threadId] = append(svr.services[threadId], s)
	if name != "" {
		svr.nameServices[name] = s
	}
	return s
}

func (svr *Server) AddLoopService(name string, imp LoopService, threadId int) {
	svr.AddService(name, "", 0, &ServiceLoop{ServiceBase{}, imp}, threadId)
}

func (svr *Server) AddEchoService(name, address string, heartbeat uint32, threadId int) {
	svr.AddService(name, address, heartbeat, &ServiceEcho{}, threadId)
}

func (svr *Server) AddHttpService(name, address string, heartbeat uint32, imp HttpService, h *HttpHandler, threadId int) {
	svr.AddService(name, address, heartbeat, &ServiceHttp{ServiceBase{}, imp, h}, threadId)
}

func (svr *Server) AddJsonService(name, address string, heartbeat uint32, imp JsonService, threadId int) JsonServiceInterface {
	return &JsonServiceImp{svr.AddService(name, address, heartbeat, &ServiceJson{ServiceBase{}, imp}, threadId)}
}

func (svr *Server) AddRpcService(name, address string, imp RpcService, threadId int) RpcServerInterface {
	if imp == nil {
		panic(fmt.Errorf("rpm imp cannot be null"))
	}
	r := newServiceRpc(imp, 0)
	r.base = svr.AddService(name, address, 0, r, threadId)
	return r
}

// AddRpcClient remoteServiceAddr: serviceName->ip:port
func (svr *Server) AddRpcClient(remoteServiceAddr map[string]string, rpcTimeOutMilli int64, threadId int) RpcClientInterface {
	r := newServiceRpc(&NullRpcService{}, rpcTimeOutMilli)
	s := svr.AddService("", "", 0, r, threadId)
	r.base = s
	return newRpcClient(remoteServiceAddr, s, r)
}

func (svr *Server) AddRpcClientEx(remoteAddr map[string]string, rpcTimeOutMilli int64, threadId int, imp RpcServiceEx) RpcClientInterface {
	r := newServiceRpcEx(imp, rpcTimeOutMilli)
	s := svr.AddService("", "", 0, r, threadId)
	r.base = s
	return newRpcClient(remoteAddr, s, r)
}

func (svr *Server) AddRouterService(address string, imp ServiceRouter, threadId int) RouterServerInterface {
	rs := newRouterService(imp)
	//heartbeat 5mins
	s := svr.AddService("", address, 300, rs, threadId)
	rs.base = s
	return rs
}

// AddRouterClient routerAddr: router's address;
func (svr *Server) AddRouterClient(routerAddr []string, appid, token string, rpcTimeOutMilli int64, threadId int) RouterClientInterface {
	r := newServiceRpc(&NullRpcService{}, rpcTimeOutMilli)
	rc := newRouterClient(r, &RouterRegister{appid, token, nil}, routerAddr)
	s := svr.AddService("", "", 0, rc, threadId)
	r.base = s
	rc.base = s
	return rc
}

func (svr *Server) AddRouterClientEx(routerAddr []string, appid, token string, rpcTimeOutMilli int64, threadId int, hp func(msg *PushProto)) RouterClientInterface {
	r := newServiceRpcEx(&RouterRpcServiceEx{hp}, rpcTimeOutMilli)
	rc := newRouterClient(r, &RouterRegister{appid, token, nil}, routerAddr)
	s := svr.AddService("", "", 0, rc, threadId)
	r.base = s
	rc.base = s
	return rc
}

// RegisterToRouter localServiceAddr: local's service [name]ip:port,register only once;
func (svr *Server) RegisterToRouter(routerAddr []string, localServiceAddr map[string]string, appid, token string, rpcTimeOutMilli int64, threadId int) RouterClientInterface {
	r := newServiceRpc(&NullRpcService{}, rpcTimeOutMilli)
	rc := newRouterClient(r, &RouterRegister{appid, token, localServiceAddr}, routerAddr)
	s := svr.AddService("", "", 0, rc, threadId)
	r.base = s
	rc.base = s
	return rc
}

// AddTcpProxyService tcp proxy
func (svr *Server) AddTcpProxyService(address string, heartbeat uint32, threadId int, proxyaddr []string, proxyweight []int) {
	if len(proxyaddr) > 1 && len(proxyaddr) != len(proxyweight) {
		panic(fmt.Errorf("error proxy param"))
	}
	c := svr.AddService("", "", 0, &ServiceProxyC{}, threadId)
	s := &ServiceProxyS{}
	s.remote = c
	addr := make([]string, len(proxyaddr))
	copy(addr, proxyaddr)
	s.remoteIP = addr
	if len(addr) > 1 {
		weight := make([]int, len(proxyweight))
		copy(weight, proxyweight)
		for i := 1; i < len(weight); i++ {
			weight[i] += weight[i-1]
		}
		s.weight = weight
	}
	svr.AddService("", address, heartbeat, s, threadId)
}

// PushRequest push message into handle thread;id of thread is the result of ServiceImp.HashProcessor
func (svr *Server) PushRequest(servicename string, sess *Session, msgid int64, msg interface{}) error {
	if servicename == "" {
		return fmt.Errorf("servicename is null")
	}
	if s, ok := svr.nameServices[servicename]; ok {
		return s.PushRequest(sess, msgid, msg)
	}
	return fmt.Errorf("no service named %s", servicename)
}

// SetLogLvl open or close system log.
func (svr *Server) SetLogLvl(lvl Level) {
	sysLog.SetLevel(lvl)
	sysLog.SetTermLevel(CLOSE)
}

type CurrentContent struct {
	GoroutineID int //thead id
	Sess        *Session
	Peer        net.Addr
	UserDefine  interface{}
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
				n := 0
				for _, s := range all {
					n += s.messageThread(threadIdx)
				}
				if n > 0 {
					continue
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
			sysLog.System("%d thread quit.", threadIdx)
			svr.wg.Done()
		}(k, v, allServices)
	}

	for i := 0; i < svr.ProcessorThreadsNum; i++ {
		if _, ok := svr.services[i]; ok {
			continue
		}
		svr.wg.Add(1)
		go func(idx int, ss []*Service) {
			for !svr.isClose.IsClose() {
				nmsg := 0
				for _, s := range ss {
					nmsg += s.messageThread(idx)
				}
				if nmsg == 0 {
					//wait for new message
					<-svr.netSignal[idx]
				}
			}
			sysLog.System("%d thread quit.", idx)
			svr.wg.Done()
		}(i, allServices)
	}
	sysLog.Debug("server start~~~~~~")
	return nil
}

func (svr *Server) Stop() {
	//stop network
	for _, v := range svr.services {
		for _, s := range v {
			s.destroy()
		}
	}
	sysLog.System("network stop~~~~~~")

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
	sysLog.System("logic stop~~~~~~")

	for _, v := range svr.services {
		for _, s := range v {
			s.imp.Destroy()
		}
	}
	logMetric()
	sysLog.Debug("server closed~~~~~~")
	logClose()
}

func (svr *Server) StopWithSignal() {
	sysWaitSignal()
	svr.Stop()
}
