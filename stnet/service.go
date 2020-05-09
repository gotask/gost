package stnet

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

type Service struct {
	*Listener
	Name         string
	imp          ServiceImp
	messageQ     []chan sessionMessage
	isClose      bool
	connects     map[uint64]*Connect
	connectMutex sync.Mutex
	netSignal    *[]chan int
	threadId     int
}

func newService(name, address string, heartbeat uint32, imp ServiceImp, netSignal *[]chan int, threadId int) (*Service, error) {
	if imp == nil || netSignal == nil {
		return nil, fmt.Errorf("ServiceImp should not be nil")
	}
	msgTh := make([]chan sessionMessage, ProcessorThreadsNum)
	for i := 0; i < ProcessorThreadsNum; i++ {
		msgTh[i] = make(chan sessionMessage, 1024)
	}
	svr := &Service{nil, name, imp, msgTh, false, make(map[uint64]*Connect, 0), sync.Mutex{}, netSignal, threadId}

	if address != "" {
		lis, err := NewListener(address, svr, heartbeat)
		if err != nil {
			return nil, err
		}
		svr.Listener = lis
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

func (service *Service) messageThread(current *CurrentContent) int {
	defer service.handlePanic()

	n := 0
	for i := 0; i < 1024; i++ {
		select {
		case msg := <-service.messageQ[current.GoroutineID]:
			current.Sess = msg.Sess
			if msg.Err != nil {
				service.imp.HandleError(current, msg.Err)
			} else if msg.DtType == Open {
				service.imp.SessionOpen(msg.Sess)
			} else if msg.DtType == Close {
				service.imp.SessionClose(msg.Sess)
			} else if msg.DtType == HeartBeat {
				service.imp.HeartBeatTimeOut(msg.Sess)
			} else if msg.DtType == Data {
				service.imp.HandleMessage(current, uint32(msg.MsgID), msg.Msg)
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
	if service.Listener != nil {
		service.Listener.Close()
	}
	service.connectMutex.Lock()
	for _, v := range service.connects {
		v.destroy()
	}
	service.connectMutex.Unlock()
	for i := 0; i < ProcessorThreadsNum; i++ {
		select {
		case service.messageQ[i] <- sessionMessage{nil, System, 0, nil, nil}:
		default:
		}
	}
}

func (service *Service) PushRequest(msgid int32, msg interface{}) error {
	select {
	case service.messageQ[service.threadId] <- sessionMessage{nil, Data, msgid, msg, nil}:
	default:
		return fmt.Errorf("service recv queue is full and the message is droped;service=%s;msgid=%d;", service.Name, msgid)
	}

	//wakeup logic thread
	select {
	case (*service.netSignal)[service.threadId] <- 1:
	default:
	}
	return nil
}

func (service *Service) ParseMsg(sess *Session, data []byte) int {
	lenParsed, msgid, msg, e := service.imp.Unmarshal(sess, data)
	if lenParsed <= 0 || msgid < 0 {
		return lenParsed
	}
	th := service.threadId
	if e == nil && msg != nil {
		th = service.imp.HashProcessor(sess, msgid, msg)
		if th >= 0 {
			th = th % ProcessorThreadsNum
		} else {
			th = service.threadId
		}
	}
	select {
	case service.messageQ[th] <- sessionMessage{sess, Data, msgid, msg, e}:
	default:
		SysLog.Error("service recv queue is full and the message is droped;service=%s;msgid=%d;err=%v;", service.Name, msgid, e)
	}

	//wakeup logic thread
	select {
	case (*service.netSignal)[th] <- 1:
	default:
	}
	return lenParsed
}

func (service *Service) IterateConnect(callback func(*Connect) bool) {
	service.connectMutex.Lock()
	defer service.connectMutex.Unlock()

	for _, c := range service.connects {
		if !callback(c) {
			break
		}
	}
}

func (service *Service) sessionEvent(sess *Session, cmd CMDType) {
	to := time.NewTimer(100 * time.Millisecond)
	select {
	case service.messageQ[service.threadId] <- sessionMessage{sess, cmd, 0, nil, nil}:
	case <-to.C:
		SysLog.Error("service recv queue is full and the message is droped;service=%s;msgtype=%d", service.Name, cmd)
	}
	to.Stop()
}

func (service *Service) NewConnect(address string, userdata interface{}) *Connect {
	conn := &Connect{NewConnector(address, service, userdata), service}
	service.connectMutex.Lock()
	service.connects[conn.GetID()] = conn
	service.connectMutex.Unlock()
	return conn
}

type Connect struct {
	*Connector
	Master *Service
}

func (ct *Connect) Imp() ServiceImp {
	return ct.Master.imp
}

func (ct *Connect) Close() {
	go ct.destroy()
	ct.Master.connectMutex.Lock()
	if _, ok := ct.Master.connects[ct.GetID()]; ok {
		delete(ct.Master.connects, ct.GetID())
	}
	ct.Master.connectMutex.Unlock()
}
func (ct *Connect) destroy() {
	ct.Connector.Close()
}
