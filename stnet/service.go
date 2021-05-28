package stnet

import (
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Service struct {
	*Listener
	Name         string
	imp          ServiceImp
	messageQ     []chan sessionMessage
	connects     map[uint64]*Connect
	connectMutex sync.Mutex
	netSignal    *[]chan int
	threadId     int
}

func parseAddress(address string) (network string, ipport string) {
	network = "tcp"
	ipport = address
	ipport = strings.Replace(ipport, " ", "", -1)
	if strings.Contains(address, "udp") {
		network = "udp"
		ipport = strings.Replace(ipport, "udp", "", -1)
	}
	return network, ipport
}

func newService(name, address string, heartbeat uint32, imp ServiceImp, netSignal *[]chan int, threadId int) (*Service, error) {
	if imp == nil || netSignal == nil {
		return nil, fmt.Errorf("ServiceImp should not be nil")
	}
	msgTh := make([]chan sessionMessage, ProcessorThreadsNum)
	for i := 0; i < ProcessorThreadsNum; i++ {
		msgTh[i] = make(chan sessionMessage, 10240)
	}
	svr := &Service{nil, name, imp, msgTh, make(map[uint64]*Connect, 0), sync.Mutex{}, netSignal, threadId}

	if address != "" {
		var (
			lis *Listener
			err error
		)
		network, ipport := parseAddress(address)
		if network == "udp" {
			lis, err = NewUdpListener(ipport, svr, heartbeat)
		} else {
			lis, err = NewListener(ipport, svr, heartbeat)
		}
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
			current.Peer = msg.peer
			if msg.Err != nil {
				service.imp.HandleError(current, msg.Err)
			} else if msg.DtType == Open {
				service.imp.SessionOpen(msg.Sess)
			} else if msg.DtType == Close {
				service.imp.SessionClose(msg.Sess)
			} else if msg.DtType == HeartBeat {
				service.imp.HeartBeatTimeOut(msg.Sess)
			} else if msg.DtType == Data {
				service.imp.HandleMessage(current, uint64(msg.MsgID), msg.Msg)
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
	MsgID  int64
	Msg    interface{}
	Err    error
	peer   net.Addr
}

func (service *Service) Imp() ServiceImp {
	return service.imp
}

func (service *Service) loop() {
	defer service.handlePanic()

	service.imp.Loop()
}
func (service *Service) destroy() {
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
		case service.messageQ[i] <- sessionMessage{nil, System, 0, nil, nil, nil}:
		default:
		}
	}
}

/*
func (service *Service) PushRequest(sess *Session, msgid int64, msg interface{}) error {
	th := service.getProcessor(sess, msgid, msg)
	m := sessionMessage{sess, Data, msgid, msg, nil, nil}
	if sess != nil {
		m.peer = sess.peer
	}
	select {
	case service.messageQ[th] <- m:
	default:
		return fmt.Errorf("service recv queue is full and the message is droped;service=%s;msgid=%d;", service.Name, msgid)
	}

	//wakeup logic thread
	select {
	case (*service.netSignal)[th] <- 1:
	default:
	}
	return nil
}*/

func (service *Service) getProcessor(sess *Session, msgid int64, msg interface{}) int {
	th := service.imp.HashProcessor(&CurrentContent{0, sess, sess.UserData, sess.peer}, uint64(msgid), msg)
	if th > 0 {
		th = th % ProcessorThreadsNum
	} else {
		th = service.threadId
	}

	return th
}

func (service *Service) ParseMsg(sess *Session, data []byte) int {
	lenParsed, msgid, msg, e := service.imp.Unmarshal(sess, data)
	if lenParsed <= 0 || msgid < 0 {
		return lenParsed
	}
	th := service.getProcessor(sess, msgid, msg)
	select {
	case service.messageQ[th] <- sessionMessage{sess, Data, msgid, msg, e, sess.peer}:
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

func (service *Service) sessionEvent(sess *Session, cmd CMDType) {
	th := service.getProcessor(sess, 0, nil)
	to := time.NewTimer(100 * time.Millisecond)
	select {
	case service.messageQ[th] <- sessionMessage{sess, cmd, 0, nil, nil, sess.peer}:
		//wakeup logic thread
		select {
		case (*service.netSignal)[th] <- 1:
		default:
		}
	case <-to.C:
		SysLog.Error("service recv queue is full and the message is droped;service=%s;msgtype=%d", service.Name, cmd)
	}
	to.Stop()
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

func (service *Service) GetConnect(id uint64) *Connect {
	service.connectMutex.Lock()
	defer service.connectMutex.Unlock()

	v, ok := service.connects[id]
	if ok {
		return v
	}
	return nil
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
