package stnet

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Connector struct {
	*Session
	address         string
	reconnectMSec   int //Millisecond
	isclose         uint32
	closeflag       bool
	sessCloseSignal chan int
	wg              *sync.WaitGroup
	mutex           *sync.Mutex
}

func NewConnector(address string, reconnectmsec int, msgparse MsgParse, userdata interface{}) *Connector {
	if msgparse == nil {
		panic(ErrMsgParseNil)
	}

	conn := &Connector{
		sessCloseSignal: make(chan int, 1),
		address:         address,
		reconnectMSec:   reconnectmsec,
		wg:              &sync.WaitGroup{},
		mutex:           &sync.Mutex{},
	}

	conn.Session, _ = newConnSession(msgparse, nil, func(*Session) {
		conn.sessCloseSignal <- 1
	}, userdata)

	go conn.connect()

	return conn
}

func (conn *Connector) connect() {
	conn.wg.Add(1)
	for !conn.closeflag {
		cn, err := net.Dial("tcp", conn.address)
		if err != nil {
			conn.parser.sessionEvent(conn.Session, Close)
			SysLog.Error("connect failed;addr=%s;error=%s", conn.address, err.Error())
			if conn.reconnectMSec < 0 || conn.closeflag {
				break
			}
			time.Sleep(time.Duration(conn.reconnectMSec) * time.Millisecond)
			continue
		}

		conn.mutex.Lock() //maybe already be closed
		if conn.closeflag {
			break
		}
		conn.Session.restart(cn)
		conn.mutex.Unlock()

		<-conn.sessCloseSignal
		if conn.reconnectMSec < 0 || conn.closeflag {
			break
		}
		time.Sleep(time.Duration(conn.reconnectMSec) * time.Millisecond)
	}
	atomic.CompareAndSwapUint32(&conn.isclose, 0, 1)
	conn.wg.Done()
}

func (cnt *Connector) ChangeAddr(addr string) {
	cnt.address = addr
	cnt.Session.Close() //close socket,wait for reconnecting
}
func (cnt *Connector) Addr() string {
	return cnt.address
}
func (cnt *Connector) ReconnectMSec() int {
	return cnt.reconnectMSec
}
func (cnt *Connector) IsConnected() bool {
	return !cnt.Session.IsClose()
}

func (c *Connector) Close() {
	if c.IsClose() {
		return
	}
	c.mutex.Lock()
	c.closeflag = true
	c.Session.Close()
	c.mutex.Unlock()
	c.wg.Wait()
	SysLog.Debug("connection close, remote addr: %s", c.address)
}

func (c *Connector) IsClose() bool {
	return atomic.LoadUint32(&c.isclose) > 0
}
