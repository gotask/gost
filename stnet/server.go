package stnet

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

//max 128 threads in server.
var (
	ProcessorThreadsNum = 32
)

type Server struct {
	name     string
	loopmsec uint32
	//threadid->Services
	services  map[int][]*Service
	wg        sync.WaitGroup
	isclose   uint32
	netSignal []chan int
}

func NewServer(name string, loopmsec uint32) *Server {
	if loopmsec == 0 {
		loopmsec = 1
	}
	svr := &Server{}
	svr.name = name
	svr.loopmsec = loopmsec
	svr.services = make(map[int][]*Service)

	svr.netSignal = make([]chan int, ProcessorThreadsNum)
	for i := 0; i < ProcessorThreadsNum; i++ {
		svr.netSignal[i] = make(chan int, 1)
	}
	return svr
}

//must be called before server started.
//address could be null,then you get a service without listen.
//when heartbeat=0,heartbeat will be close.
func (svr *Server) AddService(name, address string, heartbeat uint32, imp ServiceImp, threadId int) (*Service, error) {
	threadId = threadId % ProcessorThreadsNum
	s, e := newService(name, address, heartbeat, imp, &svr.netSignal, threadId)
	if e != nil {
		return nil, e
	}
	svr.services[threadId] = append(svr.services[threadId], s)
	return s, e
}

//must be called before server started.
func (svr *Server) AddConnect(name, address string, reconnectmsec int, imp ServiceImp, onconnected FuncOnOpen, threadId int) (*Connect, error) {
	cs, e := svr.AddService(name, "", 0, imp, threadId)
	if e != nil {
		return nil, e
	}
	ct, err := newConnect(cs, name, address, reconnectmsec, onconnected)
	if err != nil {
		return nil, err
	}
	return ct, nil
}

//can be called when server is running
func (svr *Server) NewConnect(service *Service, name, address string, reconnectmsec int, onconnected FuncOnOpen) (*Connect, error) {
	c, e := newConnect(service, name, address, reconnectmsec, onconnected)
	if e != nil {
		return nil, e
	}
	return c, nil
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
			for svr.isclose == 0 {
				nowA := time.Now()
				for _, s := range ms {
					s.loop() //service loop
				}

				//processing message of messageQ[threadIdx]
				for _, s := range all {
					s.messageThread(threadIdx)
				}

				nowB := time.Now()
				subD := nowB.Sub(nowA)
				needD := time.Duration(svr.loopmsec) * time.Millisecond
				if subD < needD {
					to := time.NewTimer(needD - subD)
					select { //wait for new message
					case <-svr.netSignal[threadIdx]:
					case <-to.C:
					}
					to.Stop()
				}
			}
			SysLog.Info("%d thread quit.", threadIdx)
			svr.wg.Done()
		}(k, v, allServices)
	}

	for i := 0; i < ProcessorThreadsNum; i++ {
		if _, ok := svr.services[i]; ok {
			continue
		}
		go func(idx int, ss []*Service) {
			svr.wg.Add(1)
			for svr.isclose == 0 {
				nmsg := 0
				for _, s := range ss {
					nmsg += s.messageThread(idx)
				}
				if nmsg == 0 {
					select { //wait for new message
					case <-svr.netSignal[idx]:
					}
				}
			}
			SysLog.Info("%d thread quit.", idx)
			svr.wg.Done()
		}(i, allServices)
	}

	return nil
}

func (svr *Server) Stop() {
	//stop network
	for _, v := range svr.services {
		for _, s := range v {
			s.destroy()
		}
	}
	time.Sleep(time.Second) //wait a second for processing destroy messages

	//stop logic work
	atomic.CompareAndSwapUint32(&svr.isclose, 0, 1)
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
	SysLog.Info("server closed.")
	SysLog.Close()
}
