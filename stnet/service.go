package stnet

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

type Service struct {
	Name      string
	listen    *Listener
	imp       ServiceImp
	messageQ  []chan sessionMessage
	isClose   bool
	connects  map[uint64]*Connect
	mutex     sync.Mutex
	netSignal *[]chan int
	threadId  int
}

func newService(name, address string, heartbeat uint32, imp ServiceImp, netSignal *[]chan int, threadId int) (*Service, error) {
	if imp == nil || netSignal == nil {
		return nil, fmt.Errorf("ServiceImp should not be nil")
	}
	msgTh := make([]chan sessionMessage, ProcessorThreadsNum)
	for i := 0; i < ProcessorThreadsNum; i++ {
		msgTh[i] = make(chan sessionMessage, 1024)
	}
	svr := &Service{name, nil, imp, msgTh, false, make(map[uint64]*Connect, 0), sync.Mutex{}, netSignal, threadId}

	if address != "" {
		lis, err := NewListener(address, svr, heartbeat)
		if err != nil {
			return nil, err
		}
		svr.listen = lis
	}

	return svr, nil
}

func (service *Service) handlePanic() {
	if err := recover(); err != nil {
		SysLog.Critical("panic error: %v", err)
		buf := make([]byte, 16384)
		buf = buf[:runtime.Stack(buf, true)]
		SysLog.Critical("panic stack: %s", string(buf))
	}
}

func (service *Service) messageThread(idx int) int {
	defer service.handlePanic()

	n := 0
	for i := 0; i < 1024; i++ {
		select {
		case msg := <-service.messageQ[idx]:
			if msg.Err != nil {
				service.imp.HandleError(msg.Sess, msg.Err)
			} else if msg.DtType == Open {
				service.imp.SessionOpen(msg.Sess)
			} else if msg.DtType == Close {
				service.imp.SessionClose(msg.Sess)
			} else if msg.DtType == HeartBeat {
				service.imp.HeartBeatTimeOut(msg.Sess)
			} else if msg.DtType == Data {
				service.imp.HandleMessage(msg.Sess, uint32(msg.MsgID), msg.Msg)
			} else if msg.DtType == System {
			} else {
				SysLog.Error("message type not find;service=%s;msgtype=%d", service.Name, msg.DtType)
			}
			n++
		default:
			return n
		}
	}
	return n
}

type sessionMessage struct {
	Sess   *Session
	DtType CMDType
	MsgID  int32
	Msg    interface{}
	Err    error
}

func (service *Service) Imp() ServiceImp {
	return service.imp
}

func (service *Service) loop() {
	defer service.handlePanic()

	service.imp.Loop()
}
func (service *Service) destroy() {
	service.isClose = true
	if service.listen != nil {
		service.listen.Close()
	}
	service.mutex.Lock()
	for _, v := range service.connects {
		v.destroy()
	}
	service.mutex.Unlock()
	for i := 0; i < ProcessorThreadsNum; i++ {
		select {
		case service.messageQ[i] <- sessionMessage{nil, System, 0, nil, nil}:
		default:
		}
	}
}
func (service *Service) ParseMsg(sess *Session, data []byte) int {
	lenParsed, msgid, msg, e := service.imp.Unmarshal(sess, data)
	if lenParsed <= 0 || msgid < 0 {
		return lenParsed
	}
	th := service.imp.HashProcessor(sess, msgid, msg)
	if e != nil || th < 0 {
		th = service.threadId
	} else if th >= ProcessorThreadsNum {
		th = th % ProcessorThreadsNum
	}
	to := time.NewTimer(time.Second)
	select {
	case service.messageQ[th] <- sessionMessage{sess, Data, msgid, msg, e}:
	case <-to.C:
		SysLog.Error("service recv queue is full and the message is droped;service=%s;msgid=%d;err=%v;", service.Name, msgid, e)
	}
	to.Stop()

	//wakeup logic thread
	select {
	case (*service.netSignal)[th] <- 1:
	default:
	}
	return lenParsed
}
func (service *Service) SessionEvent(sess *Session, cmd CMDType) {
	to := time.NewTimer(time.Second)
	select {
	case service.messageQ[service.threadId] <- sessionMessage{sess, cmd, 0, nil, nil}:
	case <-to.C:
		SysLog.Error("service recv queue is full and the message is droped;service=%s;msgtype=%d", service.Name, cmd)
	}
	to.Stop()
}

func newConnect(service *Service, name, address string, reconnectmsec int, userdata interface{}) *Connect {
	conn := &Connect{NewConnector(address, reconnectmsec, service, userdata), name, service}
	service.mutex.Lock()
	service.connects[conn.GetID()] = conn
	service.mutex.Unlock()
	return conn
}

type Connect struct {
	*Connector
	Name   string
	Master *Service
}

func (ct *Connect) Imp() ServiceImp {
	return ct.Master.imp
}

func (ct *Connect) Close() {
	ct.destroy()
	ct.Master.mutex.Lock()
	if _, ok := ct.Master.connects[ct.GetID()]; ok {
		delete(ct.Master.connects, ct.GetID())
	}
	ct.Master.mutex.Unlock()
}
func (ct *Connect) destroy() {
	ct.Connector.Close()
}
