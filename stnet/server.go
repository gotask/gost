package stnet

import (
	"fmt"
	"sync"
	"time"
)

//number of threads in server.
var (
	ProcessorThreadsNum = 32
)

type Server struct {
	name     string
	loopmsec uint32
	//threadid->Services
	services  map[int][]*Service
	wg        sync.WaitGroup
	isClose   bool
	netSignal []chan int

	nameServices map[string]*Service
}

func NewServer(name string, loopmsec uint32) *Server {
	if loopmsec == 0 {
		loopmsec = 1
	}
	svr := &Server{}
	svr.name = name
	svr.loopmsec = loopmsec
	svr.services = make(map[int][]*Service)
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
func (svr *Server) AddService(name, address string, heartbeat uint32, imp ServiceImp, threadId int) (*Service, error) {
	threadId = threadId % ProcessorThreadsNum
	s, e := newService(name, address, heartbeat, imp, &svr.netSignal, threadId)
	if e != nil {
		return nil, e
	}
	svr.services[threadId] = append(svr.services[threadId], s)
	svr.nameServices[name] = s
	return s, e
}

//must be called before server started.
func (svr *Server) AddConnect(name, address string, imp ServiceImp, threadId int) (*Connect, error) {
	cs, e := svr.AddService(name, "", 0, imp, threadId)
	if e != nil {
		return nil, e
	}
	return newConnect(cs, name, address, nil), nil
}

//can be called when server is running
//this connect session uses service's ServiceImp to handle message.
func (svr *Server) NewConnect(service *Service, name, address string, userdata interface{}) *Connect {
	return newConnect(service, name, address, userdata)
}

func (svr *Server) PushRequest(servicename string, msgid int32, msg interface{}) error {
	if s, ok := svr.nameServices[servicename]; ok {
		return s.PushRequest(msgid, msg)
	}
	return fmt.Errorf("no service named %s", servicename)
}

type CurrentContent struct {
	GoroutineID int
	Sess        *Session
	UserDefined interface{}
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
			for !svr.isClose {
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

			current := &CurrentContent{GoroutineID: idx}
			for !svr.isClose {
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
	svr.isClose = true
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
