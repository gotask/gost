package stnet

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	name     string
	loopmsec uint32
	//threadid->Services
	services map[int][]*Service
	wg       sync.WaitGroup
	isclose  uint32
}

func NewServer(name string, loopmsec uint32) *Server {
	if loopmsec == 0 {
		loopmsec = 1
	}
	svr := &Server{}
	svr.name = name
	svr.loopmsec = loopmsec
	svr.services = make(map[int][]*Service)
	return svr
}

//must be called before server started.
//address could be null,then you get a service without listen.
func (svr *Server) AddService(name, address string, imp ServiceImp, threadId int) (*Service, error) {
	s, e := newService(name, address, imp)
	if e != nil {
		return nil, e
	}
	svr.services[threadId] = append(svr.services[threadId], s)
	return s, e
}

//must be called before server started.
func (svr *Server) AddConnect(name, address string, reconnectmsec int, imp ServiceImp, threadId int) (*Connect, error) {
	cs, e := svr.AddService(name, "", imp, threadId)
	if e != nil {
		return nil, e
	}
	ct, err := newConnect(cs, name, address, reconnectmsec)
	if err != nil {
		return nil, err
	}
	return ct, nil
}

//can be called when server is running
func (svr *Server) NewConnect(service *Service, name, address string, reconnectmsec int) (*Connect, error) {
	c, e := newConnect(service, name, address, reconnectmsec)
	if e != nil {
		return nil, e
	}
	return c, nil
}

func (svr *Server) Start() error {
	for _, v := range svr.services {
		for _, s := range v {
			if !s.imp.Init() {
				return fmt.Errorf(s.Name + " init failed!")
			}
		}
	}

	for _, v := range svr.services {
		go func(ss []*Service) {
			svr.wg.Add(1)
			for svr.isclose == 0 {
				nowA := time.Now()
				for _, s := range ss {
					s.loop()
				}

				nowB := time.Now()
				subD := nowB.Sub(nowA)
				needD := time.Duration(svr.loopmsec) * time.Millisecond
				if subD < needD {
					time.Sleep(needD - subD)
				}
			}
			svr.wg.Done()
		}(v)

		for i := 0; i < msgProcessorThreadsNum; i++ {
			go func(idx int, ss []*Service) {
				svr.wg.Add(1)
				for svr.isclose == 0 {
					n := 0
					for _, s := range ss {
						n += s.messageThread(idx)
					}
					if n == 0 {
						time.Sleep(time.Duration(svr.loopmsec) * time.Millisecond)
					}
				}
				svr.wg.Done()
			}(i+1, v)
		}
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
	//stop logic work
	atomic.CompareAndSwapUint32(&svr.isclose, 0, 1)
	svr.wg.Wait()
	for _, v := range svr.services {
		for _, s := range v {
			s.imp.Destroy()
		}
	}
}
