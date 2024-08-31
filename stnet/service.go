package stnet

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Service struct {
	*Listener
	Name                string
	imp                 ServiceImp
	messageQ            []chan sessionMessage
	connects            sync.Map //map[uint64]*Connect
	netSignal           *[]chan int
	threadId            int
	ProcessorThreadsNum int //number of threads in server.
}

func parseAddress(address string) (network string, ipport string) {
	network = "tcp"
	ipport = address
	ipport = strings.Replace(ipport, " ", "", -1)
	if strings.Contains(address, "udp:") {
		network = "udp"
		ipport = strings.Replace(ipport, "udp:", "", -1)
	}
	return network, ipport
}

func (service *Service) handlePanic() {
	if err := recover(); err != nil {
		sysLog.Critical("panic error: %v", err)
		buf := make([]byte, 16384)
		buf = buf[:runtime.Stack(buf, true)]
		sysLog.Critical("panic stack: %s", string(buf))
	}
}

func (service *Service) handleMsg(msg sessionMessage) {
	current := msg.current
	if msg.Err != nil {
		service.imp.HandleError(current, msg.Err)
	} else if msg.DtType == Open {
		service.imp.SessionOpen(current.Sess)
	} else if msg.DtType == Close {
		service.imp.SessionClose(current.Sess)
	} else if msg.DtType == ConnClose {
		service.onConnectClose(current.Sess.conn)
	} else if msg.DtType == HeartBeat {
		service.imp.HeartBeatTimeOut(current.Sess)
	} else if msg.DtType == Data {
		service.imp.HandleMessage(current, uint64(msg.MsgID), msg.Msg)
	} else if msg.DtType == System {
	} else {
		sysLog.Error("message type not find;service=%s;msgtype=%d", service.Name, msg.DtType)
	}
}

func (service *Service) messageThread(threadIdx int) int {
	defer service.handlePanic()

	n := 0
	for i := 0; i < 1024; i++ {
		select {
		case msg := <-service.messageQ[threadIdx]:
			service.handleMsg(msg)
			n++
		default:
			return n
		}
	}
	return n
}

type sessionMessage struct {
	current *CurrentContent
	DtType  CMDType
	MsgID   int64
	Msg     interface{}
	Err     error
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
	service.connects.Range(func(k, v interface{}) bool {
		v.(*Connector).Close()
		return true
	})
	for i := 0; i < service.ProcessorThreadsNum; i++ {
		select {
		case service.messageQ[i] <- sessionMessage{nil, System, 0, nil, nil}:
		default:
		}
	}
}

func (service *Service) PushRequest(sess *Session, msgid int64, msg interface{}) error {
	cur := &CurrentContent{0, sess, sess.peer, nil}
	m := sessionMessage{cur, Data, msgid, msg, nil}
	th := service.getProcessor(cur, msgid, msg)
	select {
	case service.messageQ[th] <- m:
	default:
		return fmt.Errorf("service recv queue is full and the message is droped;service=%s;msgid=%d", service.Name, msgid)
	}

	//wakeup logic thread
	select {
	case (*service.netSignal)[th] <- 1:
	default:
	}
	return nil
}

func (service *Service) getProcessor(cur *CurrentContent, msgid int64, msg interface{}) int {
	th := service.imp.HashProcessor(cur, uint64(msgid), msg)
	if th > 0 {
		th = th % service.ProcessorThreadsNum
	} else if th == 0 {
		th = service.threadId
	} else if th == -1 {
		if cur.Sess != nil {
			th = int(cur.Sess.GetID() % uint64(service.ProcessorThreadsNum))
		} else { //session is nil
			th = service.threadId
		}
	}

	cur.GoroutineID = th

	if th < 0 || th >= service.ProcessorThreadsNum {
		th = service.threadId
	}
	return th
}

func (service *Service) ParseMsg(sess *Session, data []byte) int {
	lenParsed, msgid, msg, e := service.imp.Unmarshal(sess, data)
	if lenParsed <= 0 || msgid < 0 {
		return lenParsed
	}
	cur := &CurrentContent{0, sess, sess.peer, nil}
	th := service.getProcessor(cur, msgid, msg)
	select {
	case service.messageQ[th] <- sessionMessage{cur, Data, msgid, msg, e}:
	default:
		sysLog.Error("service recv queue is full and the message is droped;service=%s;msgid=%d;err=%v;", service.Name, msgid, e)
	}

	//wakeup logic thread
	select {
	case (*service.netSignal)[th] <- 1:
	default:
	}
	return lenParsed
}

func (service *Service) sessionEvent(sess *Session, cmd CMDType) {
	th := service.threadId
	cur := &CurrentContent{th, sess, sess.peer, nil}
	if cmd == Data {
		th = service.getProcessor(cur, 0, nil)
	} else if cmd == Open {
		service.handleMsg(sessionMessage{cur, cmd, 0, nil, nil})
		return
	}

	to := time.NewTimer(100 * time.Millisecond)
	select {
	case service.messageQ[th] <- sessionMessage{cur, cmd, 0, nil, nil}:
		//wakeup logic thread
		select {
		case (*service.netSignal)[th] <- 1:
		default:
		}
	case <-to.C:
		sysLog.Error("service recv queue is full and the message is droped;service=%s;msgtype=%d", service.Name, cmd)
	}
	to.Stop()
}

func (service *Service) IterateConnect(callback func(connector *Connector) bool) {
	service.connects.Range(func(k, v interface{}) bool {
		return callback(v.(*Connector))
	})
}

func (service *Service) GetSession(id uint64) *Session {
	v, ok := service.connects.Load(id)
	if ok {
		return v.(*Connector).sess
	}
	return service.Listener.GetSession(id)
}

// NewConnect reconnect at 0 1 4 9 16...times reconnectMSec(100ms);when call send or changeAddr, it will NotifyReconn and reconnect at once;when call Close, reconnect will stop
func (service *Service) NewConnect(address string, userdata interface{}) *Connector {
	conn := NewConnector(address, service, userdata)
	service.connects.Store(conn.GetID(), conn)
	return conn
}

func (service *Service) onConnectClose(c *Connector) {
	service.connects.Delete(c.GetID())
}
