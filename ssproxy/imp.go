// imp
package main

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/gotask/gost/stencode"
	"github.com/gotask/gost/stnet"
)

var (
	magicNumber     uint64 = 3
	msgMaxLen       uint32 = 1024 * 1024 * 10
	globalThreadIdx        = 0
)

func threadIndex() int {
	globalThreadIdx++
	return globalThreadIdx
}

func AddLCProxy(svr *stnet.Server, lc map[string]proxyWeight) error {
	for k, v := range lc {
		e := svr.AddTcpProxyService(k, 0, threadIndex(), v.address, v.weight)
		if e != nil {
			return e
		}
	}
	return nil
}

func AddLLProxy(svr *stnet.Server, ll map[string]proxyWeight) error {
	for k, v := range ll {
		if len(v.address) != 1 {
			return fmt.Errorf("error proxy param: %s", k)
		}
		llr := &ServiceProxyLLRaw{}
		raw, e := svr.AddService("", k, 0, llr, threadIndex())
		if e != nil {
			return e
		}

		llg := &ServiceProxyLLGpb{}
		gpb, e := svr.AddService("", v.address[0], 0, llg, threadIndex())
		if e != nil {
			return e
		}

		llr.gpb = gpb
		llg.raw = raw
	}
	return nil
}

func AddCCProxy(svr *stnet.Server, cc map[string]proxyWeight) error {
	for k, v := range cc {
		ccr := &ServiceProxyCCRaw{}
		raw, e := svr.AddService("", "", 0, ccr, threadIndex())
		if e != nil {
			return e
		}

		ccg := &ServiceProxyCCGpb{}
		ccg.remoteip = v.address
		ccg.weight = v.weight
		gpb, e := svr.AddService("", "", 0, ccg, threadIndex())
		if e != nil {
			return e
		}
		gpb.NewConnect(k, nil)

		ccr.gpb = gpb
		ccr.proxy = ccg
		ccg.raw = raw
	}
	return nil
}

func SendRawGpb(sess *stnet.Session, cmdid, cmdseq uint64, msg []byte) error {
	r := ProtocolMessage{}
	r.CmdId = cmdid
	r.CmdSeq = cmdseq
	r.CmdData = msg

	buf, e := stencode.Marshal(&r)
	if e != nil {
		return e
	}
	return sess.Send(PackSendProtocol(buf), nil)
}

type ServiceProxyLLRaw struct {
	stnet.ServiceBase
	gpb *stnet.Service
}

func (service *ServiceProxyLLRaw) SessionClose(sess *stnet.Session) {
	service.gpb.IterateSession(func(s *stnet.Session) bool {
		SendRawGpb(s, sess.GetID(), magicNumber, nil)
		return false
	})
}
func (service *ServiceProxyLLRaw) HeartBeatTimeOut(sess *stnet.Session) {
	sess.Close()
}
func (service *ServiceProxyLLRaw) HandleError(current *stnet.CurrentContent, err error) {
	LOG.Error(err.Error())
	current.Sess.Close()
}
func (service *ServiceProxyLLRaw) Unmarshal(sess *stnet.Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	service.gpb.IterateSession(func(s *stnet.Session) bool {
		SendRawGpb(s, sess.GetID(), magicNumber, data)
		return false
	})
	return len(data), -1, nil, nil
}

func (service *ServiceProxyLLRaw) HashProcessor(current *stnet.CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return int(current.Sess.GetID())
}

type ServiceProxyLLGpb struct {
	stnet.ServiceBase
	raw *stnet.Service
}

func (service *ServiceProxyLLGpb) HandleError(current *stnet.CurrentContent, err error) {
	LOG.Error(err.Error())
	current.Sess.Close()
}
func (service *ServiceProxyLLGpb) Unmarshal(sess *stnet.Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := DecodeLength(data)
	if msgLen < 4 || msgLen > msgMaxLen {
		return len(data), 0, nil, fmt.Errorf("invalid length: %d", msgLen)
	}
	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}
	m := &ProtocolMessage{}
	e := stencode.Unmarshal(data[4:msgLen], m)
	if e != nil {
		return len(data), 0, nil, e
	}
	if m.CmdSeq != magicNumber {
		return len(data), 0, nil, fmt.Errorf("invalid magicnumber: %d", m.CmdSeq)
	}
	s := service.raw.GetSession(m.CmdId)
	if s != nil {
		if len(m.CmdData) == 0 {
			s.Close()
		} else {
			s.Send(m.CmdData, nil)
		}
	}
	return int(msgLen), -1, nil, nil
}

type ServiceProxyCCRaw struct {
	stnet.ServiceBase
	gpb   *stnet.Service
	proxy *ServiceProxyCCGpb
}

func (service *ServiceProxyCCRaw) SessionClose(sess *stnet.Session) {
	if sess.UserData != nil {
		service.gpb.IterateConnect(func(c *stnet.Connect) bool {
			SendRawGpb(c.Session(), sess.UserData.(uint64), magicNumber, nil)
			return false
		})
	}
}

func (service *ServiceProxyCCRaw) HandleError(current *stnet.CurrentContent, err error) {
	LOG.Error(err.Error())
	current.Sess.Close()
}

func (service *ServiceProxyCCRaw) Unmarshal(sess *stnet.Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	if sess.UserData != nil {
		service.proxy.sessid.Delete(sess.UserData.(uint64))
		service.gpb.IterateConnect(func(c *stnet.Connect) bool {
			SendRawGpb(c.Session(), sess.UserData.(uint64), magicNumber, data)
			return false
		})
	}
	return len(data), -1, nil, nil
}

func (service *ServiceProxyCCRaw) HashProcessor(current *stnet.CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return int(current.Sess.GetID())
}

type ServiceProxyCCGpb struct {
	stnet.ServiceBase
	raw    *stnet.Service
	sessid sync.Map

	remoteip []string
	weight   []int
}

func (service *ServiceProxyCCGpb) HandleError(current *stnet.CurrentContent, err error) {
	LOG.Error(err.Error())
	current.Sess.Close()
}

func (service *ServiceProxyCCGpb) Unmarshal(sess *stnet.Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := DecodeLength(data)
	if msgLen < 4 || msgLen > msgMaxLen {
		return len(data), 0, nil, fmt.Errorf("invalid length: %d", msgLen)
	}
	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}
	m := &ProtocolMessage{}
	e := stencode.Unmarshal(data[4:msgLen], m)
	if e != nil {
		return len(data), 0, nil, e
	}
	if m.CmdSeq != magicNumber {
		return len(data), 0, nil, fmt.Errorf("invalid magicnumber: %d", m.CmdSeq)
	}

	var co *stnet.Connect
	c, ok := service.sessid.Load(m.CmdId)
	if ok {
		co = c.(*stnet.Connect)
	}
	if co != nil {
		if len(m.CmdData) == 0 {
			service.sessid.Delete(m.CmdId)
			co.Close()
		} else {
			co.Send(m.CmdData)
		}
	} else if len(m.CmdData) > 0 {
		rip := service.remoteip[0]
		ln := len(service.weight)
		if ln > 1 {
			r := rand.Int() % service.weight[ln-1]
			for i := 0; i < ln; i++ {
				if r < service.weight[i] {
					rip = service.remoteip[i]
					break
				}
			}
		}
		c := service.raw.NewConnect(rip, m.CmdId)
		service.sessid.Store(m.CmdId, c)
		c.Send(m.CmdData)
	}
	return int(msgLen), -1, nil, nil
}
