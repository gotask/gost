package stnet

import (
	"fmt"
	"sync"
	"time"
)

var (
	msgThreadsNum = 64
)

func newService(name, address string, imp ServiceImp) (*Service, error) {
	if imp == nil {
		return nil, fmt.Errorf("ServiceImp should not be nil")
	}
	msgTh := make([]chan sessionMessage, msgThreadsNum+1)
	for i := 0; i <= msgThreadsNum; i++ {
		msgTh[i] = make(chan sessionMessage, 1024)
	}
	svr := &Service{name, nil, imp, msgTh, make(map[uint32]FuncHandleMessage), sync.WaitGroup{}, false, make(map[uint64]*Connect, 0), sync.Mutex{}}

	if address != "" {
		lis, err := NewListener(address, svr)
		if err != nil {
			return nil, err
		}
		svr.listen = lis
	}

	for i := 0; i < msgThreadsNum; i++ {
		go func(idx int) {
			svr.wg.Add(1)
			for !svr.isClose {
				select {
				case msg := <-svr.messageQ[idx]:
					if msg.DtType == Data {
						if handler, ok := svr.messageHandlers[msg.MsgID]; ok {
							handler(msg.Sess, msg.Msg)
						} else {
							SysLog.Error("message handler not find;msgid=%d;sessionid=%d", msg.MsgID, msg.Sess.GetID())
						}
					}
				}
			}
			svr.wg.Done()
		}(i + 1)
	}
	return svr, nil
}

func (service *Service) RegisterMessage(msgID uint32, handler FuncHandleMessage) {
	if handler == nil {
		return
	}
	service.messageHandlers[msgID] = handler
}

type Service struct {
	Name            string
	listen          *Listener
	imp             ServiceImp
	messageQ        []chan sessionMessage
	messageHandlers map[uint32]FuncHandleMessage
	wg              sync.WaitGroup
	isClose         bool
	connects        map[uint64]*Connect
	mutex           sync.Mutex
}

type sessionMessage struct {
	Sess   *Session
	DtType CMDType
	MsgID  uint32
	Msg    interface{}
	Err    error
}

func (service *Service) Imp() ServiceImp {
	return service.imp
}

func (service *Service) loop() {
	for i := 0; i < 100; i++ {
		select {
		case msg := <-service.messageQ[0]:
			if msg.Err != nil {
				service.imp.HandleError(msg.Sess, msg.Err)
			} else if msg.DtType == Open {
				service.imp.SessionOpen(msg.Sess)
			} else if msg.DtType == Close {
				service.imp.SessionClose(msg.Sess)
			} else if msg.DtType == Data {
				if handler, ok := service.messageHandlers[msg.MsgID]; ok {
					handler(msg.Sess, msg.Msg)
				} else {
					SysLog.Error("message handler not find;service=%s;msgid=%d;sessionid=%d", service.Name, msg.MsgID, msg.Sess.GetID())
				}
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
	for _, v := range service.connects {
		v.destroy()
	}
	for i := 0; i < msgThreadsNum; i++ {
		select {
		case service.messageQ[i+1] <- sessionMessage{nil, System, 0, nil, nil}:
		default:
		}
	}
	service.wg.Wait()
}
func (service *Service) ParseMsg(sess *Session, data []byte) int {
	lenParsed, msgid, msg, e := service.imp.Unmarshal(sess, data)
	if lenParsed == 0 {
		return 0
	}
	th := service.imp.HashHandleThread(sess)
	if e != nil || th < 0 || th > msgThreadsNum {
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
