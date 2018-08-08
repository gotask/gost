package stnet

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

var (
	msgProcessorThreadsNum = 128
)

func newService(name, address string, imp ServiceImp) (*Service, error) {
	if imp == nil {
		return nil, fmt.Errorf("ServiceImp should not be nil")
	}
	msgTh := make([]chan sessionMessage, msgProcessorThreadsNum+1)
	for i := 0; i <= msgProcessorThreadsNum; i++ {
		msgTh[i] = make(chan sessionMessage, 1024)
	}
	svr := &Service{name, nil, imp, msgTh, false, make(map[uint64]*Connect, 0), sync.Mutex{}}

	if address != "" {
		lis, err := NewListener(address, svr)
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
		SysLog.Critical("panic stack: %s", string(debug.Stack()))
	}
}

func (service *Service) messageThread(idx int) int {
	defer service.handlePanic()

	for i := 0; i < 1024; i++ {
		select {
		case msg := <-service.messageQ[idx]:
			if msg.DtType == Data {
				service.imp.HandleMessage(msg.Sess, uint32(msg.MsgID), msg.Msg)
			}
		default:
			return i
		}
	}
	return 1024
}

type Service struct {
	Name     string
	listen   *Listener
	imp      ServiceImp
	messageQ []chan sessionMessage
	isClose  bool
	connects map[uint64]*Connect
	mutex    sync.Mutex
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
	for i := 0; i < 1024; i++ {
		select {
		case msg := <-service.messageQ[0]:
			if msg.Err != nil {
				service.imp.HandleError(msg.Sess, msg.Err)
			} else if msg.DtType == Open {
				service.imp.SessionOpen(msg.Sess)
			} else if msg.DtType == Close {
				service.imp.SessionClose(msg.Sess)
			} else if msg.DtType == Data {
				service.imp.HandleMessage(msg.Sess, uint32(msg.MsgID), msg.Msg)
			} else {
				SysLog.Error("message type not find;service=%s;msgtype=%d", service.Name, msg.DtType)
			}
		default:
			return
		}
	}
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
	for i := 0; i < msgProcessorThreadsNum; i++ {
		select {
		case service.messageQ[i+1] <- sessionMessage{nil, System, 0, nil, nil}:
		default:
		}
	}
}
func (service *Service) ParseMsg(sess *Session, data []byte) int {
	lenParsed, th, msgid, msg, e := service.imp.Unmarshal(sess, data)
	if lenParsed <= 0 || msgid < 0 {
		return lenParsed
	}
	if e != nil || th < 0 || th > msgProcessorThreadsNum {
		th = 0
	}
	to := time.NewTimer(time.Second)
	select {
	case service.messageQ[th] <- sessionMessage{sess, Data, msgid, msg, e}:
	case <-to.C:
		SysLog.Error("service recv queue is full and the message is droped;service=%s;msgid=%d;err=%v;", service.Name, msgid, e)
	}
	to.Stop()
	return lenParsed
}
func (service *Service) SessionEvent(sess *Session, cmd CMDType) {
	to := time.NewTimer(time.Second)
	select {
	case service.messageQ[0] <- sessionMessage{sess, cmd, 0, nil, nil}:
	case <-to.C:
		SysLog.Error("service recv queue is full and the message is droped;service=%s;msgtype=%d", service.Name, cmd)
	}
	to.Stop()
}

func newConnect(service *Service, name, address string, reconnectmsec int) (*Connect, error) {
	if service == nil {
		return nil, fmt.Errorf("service should not be nil")
	}
	conn := &Connect{service, NewConnector(address, reconnectmsec, service), name}
	service.mutex.Lock()
	service.connects[conn.GetID()] = conn
	service.mutex.Unlock()
	return conn, nil
}

type Connect struct {
	*Service
	*Connector
	Name string
}

func (ct *Connect) Close() {
	ct.destroy()
	ct.mutex.Lock()
	if _, ok := ct.connects[ct.GetID()]; ok {
		delete(ct.connects, ct.GetID())
	}
	ct.mutex.Unlock()
}
func (ct *Connect) destroy() {
	ct.Connector.Close()
}
