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
	services     map[int][]*Service
	nullservices map[int][]*NullService
	connects     map[int][]*Connect
	wg           sync.WaitGroup
	isclose      uint32
}

func NewServer(name string, loopmsec uint32) *Server {
	svr := &Server{}
	svr.name = name
	svr.loopmsec = loopmsec
	svr.services = make(map[int][]*Service)
	svr.nullservices = make(map[int][]*NullService)
	svr.connects = make(map[int][]*Connect)
	return svr
}

func (svr *Server) AddNullService(name string, imp NullServiceImp, threadId int) *NullService {
	s := &NullService{name, imp}
	svr.nullservices[threadId] = append(svr.nullservices[threadId], s)
	return s
}

func (svr *Server) AddService(name, address string, imp ServiceImp, threadId int) (*Service, error) {
	s, e := newService(name, address, imp)
	if e != nil {
		return nil, e
	}
	svr.services[threadId] = append(svr.services[threadId], s)
	return s, e
}

func (svr *Server) AddRpcService(name, address string, rpcFuncStruct interface{}, threadId int) (*Service, error) {
	rpcImp := &RPCServerImp{rpcFuncStruct}
	s, e := newService(name, address, rpcImp)
	if e != nil {
		return nil, e
	}
	svr.services[threadId] = append(svr.services[threadId], s)
	return s, e
}

func (svr *Server) AddConnect(name, address string, reconnectmsec int, imp ConnectImp, threadId int) (*Connect, error) {
	c, e := newConnect(name, address, reconnectmsec, imp)
	if e != nil {
		return nil, e
	}
	svr.connects[threadId] = append(svr.connects[threadId], c)
	return c, e
}

func (svr *Server) AddConnectNoStart(name, address string, reconnectmsec int, imp ConnectImp, threadId int) (*Connect, error) {
	c, e := newConnectNoStart(name, address, reconnectmsec, imp)
	if e != nil {
		return nil, e
	}
	svr.connects[threadId] = append(svr.connects[threadId], c)
	return c, e
}

func (svr *Server) AddRpcClient(name, servicename, address string, threadId int) (*RPC, error) {
	r, e := newRPC(name, servicename, address)
	if e != nil {
		return nil, e
	}
	svr.connects[threadId] = append(svr.connects[threadId], r.Connect)
	svr.AddNullService(name, r.rpcimp, threadId)
	return r, e
}

func (svr *Server) Start() error {
	for _, v := range svr.services {
		for _, s := range v {
			if !s.imp.Init() {
				return fmt.Errorf(s.Name + " init failed!")
			}
			s.imp.RegisterSMessage(s)
		}
	}

	for _, v := range svr.nullservices {
		for _, s := range v {
			if !s.imp.Init() {
				return fmt.Errorf(s.Name + " init failed!")
			}
		}
	}

	for _, v := range svr.connects {
		for _, c := range v {
			c.imp.RegisterCMessage(c)
		}
	}

	keyUsed := make(map[int]int)
	for k, v := range svr.services {
		keyUsed[k] = 1
		ct, _ := svr.connects[k]
		ns, _ := svr.nullservices[k]
		go func(ss []*Service, sn []*NullService, cc []*Connect) {
			svr.wg.Add(1)
			for svr.isclose == 0 {
				for _, s := range ss {
					s.loop()
					s.imp.Loop()
				}
				for _, s := range sn {
					s.imp.Loop()
				}
				for _, c := range cc {
					c.loop()
				}
				time.Sleep(time.Duration(svr.loopmsec) * time.Millisecond)
			}
			svr.wg.Done()
		}(v, ns, ct)
	}

	for k, v := range svr.nullservices {
		if _, ok := keyUsed[k]; ok {
			continue
		}
		go func(ns []*NullService) {
			svr.wg.Add(1)
			for svr.isclose == 0 {
				for _, s := range ns {
					s.imp.Loop()
				}
				time.Sleep(time.Duration(svr.loopmsec) * time.Millisecond)
			}
			svr.wg.Done()
		}(v)
	}

	for k, v := range svr.connects {
		if _, ok := keyUsed[k]; ok {
			continue
		}
		go func(cc []*Connect) {
			svr.wg.Add(1)
			for svr.isclose == 0 {
				for _, c := range cc {
					c.loop()
				}
				time.Sleep(time.Duration(svr.loopmsec) * time.Millisecond)
			}
			svr.wg.Done()
		}(v)
	}
	return nil
}

func (svr *Server) Stop() {
	if !atomic.CompareAndSwapUint32(&svr.isclose, 0, 1) {
		return
	}
	svr.wg.Wait()
	for _, v := range svr.services {
		for _, s := range v {
			s.imp.Destroy()
			s.destroy()
		}
	}
	for _, v := range svr.nullservices {
		for _, s := range v {
			s.imp.Destroy()
		}
	}
	for _, v := range svr.connects {
		for _, c := range v {
			c.destroy()
		}
	}
}
