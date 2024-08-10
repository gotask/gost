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
	Name     string
	imp      ServiceImp
	messageQ []chan sessionMessage
	connects sync.Map //map[uint64]*Connect
	//connectMutex sync.Mutex
	netSignal *[]chan int
	threadId  int
	svr       *Server
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

func (service *Service) handleMsg(current *CurrentContent, msg sessionMessage) {
	current.Sess = msg.Sess
	current.Peer = msg.peer
	if msg.Err != nil {
		service.imp.HandleError(current, msg.Err)
	} else if msg.DtType == Open {
		service.imp.SessionOpen(msg.Sess)
	} else if msg.DtType == Close {
		service.imp.SessionClose(msg.Sess)
	} else if msg.DtType == ConnClose {
		service.onConnectClose(msg.Sess.conn)
	} else if msg.DtType == HeartBeat {
		service.imp.HeartBeatTimeOut(msg.Sess)
	} else if msg.DtType == Data {
		service.imp.HandleMessage(current, uint64(msg.MsgID), msg.Msg)
	} else if msg.DtType == System {
	} else {
		sysLog.Error("message type not find;service=%s;msgtype=%d", service.Name, msg.DtType)
	}
}

func (service *Service) messageThread(current *CurrentContent) int {
	defer service.handlePanic()

	n := 0
	for i := 0; i < 1024; i++ {
		select {
		case msg := <-service.messageQ[current.GoroutineID]:
			service.handleMsg(current, msg)
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
	service.connects.Range(func(k, v interface{}) bool {
		v.(*Connector).Close()
		return true
	})
	for i := 0; i < service.svr.ProcessorThreadsNum; i++ {
		select {
		case service.messageQ[i] <- sessionMessage{nil, System, 0, nil, nil, nil}:
		default:
		}
	}
}

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
}

func (service *Service) getProcessor(sess *Session, msgid int64, msg interface{}) int {
	cur := &CurrentContent{}
	if sess != nil {
		cur = &CurrentContent{0, sess, sess.UserData, sess.peer}
	}
	th := service.imp.HashProcessor(cur, uint64(msgid), msg)
	if th > 0 {
		th = th % service.svr.ProcessorThreadsNum
	} else if th == 0 {
		th = service.threadId
	} else if sess != nil {
		t := sess.GetID() % uint64(service.svr.ProcessorThreadsNum)
		th = int(t)
	} else { //session is nil
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
	if cmd == Data {
		th = service.getProcessor(sess, 0, nil)
	} else if cmd == Open {
		service.handleMsg(&CurrentContent{th, sess, nil, nil}, sessionMessage{sess, cmd, 0, nil, nil, sess.peer})
		return
	}

	to := time.NewTimer(100 * time.Millisecond)
	select {
	case service.messageQ[th] <- sessionMessage{sess, cmd, 0, nil, nil, sess.peer}:
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
