package stnet

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Connector struct {
	sess            *Session
	address         string
	reconnectMSec   int //Millisecond
	isclose         uint32
	closeflag       bool
	sessCloseSignal chan int
	reconnSignal    chan int
	wg              *sync.WaitGroup
	mutex           *sync.Mutex
}

func NewConnector(address string, msgparse MsgParse, userdata interface{}) *Connector {
	if msgparse == nil {
		panic(ErrMsgParseNil)
	}

	conn := &Connector{
		sessCloseSignal: make(chan int, 1),
		reconnSignal:    make(chan int, 1),
		address:         address,
		reconnectMSec:   5000,
		wg:              &sync.WaitGroup{},
		mutex:           &sync.Mutex{},
	}

	conn.sess, _ = newConnSession(msgparse, nil, func(*Session) {
		conn.sessCloseSignal <- 1
	}, userdata)

	go conn.connect()

	return conn
}

func (conn *Connector) connect() {
	conn.wg.Add(1)
	reconnNum := 0
	for !conn.closeflag {
		reconnNum++
		cn, err := net.Dial("tcp", conn.address)
		if err != nil {
			conn.sess.parser.sessionEvent(conn.sess, Close)
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
		conn.sess.restart(cn)
		conn.mutex.Unlock()

		<-conn.sessCloseSignal
		if conn.reconnectMSec < 0 || conn.closeflag {
			break
		}
		if reconnNum <= 3 { //reconnect 3 times
			time.Sleep(time.Duration(reconnNum*reconnNum*conn.reconnectMSec) * time.Millisecond)
		} else {
			<-conn.reconnSignal
			reconnNum = 0
		}
	}
	atomic.CompareAndSwapUint32(&conn.isclose, 0, 1)
	conn.wg.Done()
}

func (c *Connector) ChangeAddr(addr string) {
	c.address = addr
	c.sess.Close() //close socket,wait for reconnecting
	c.NotifyReconn()
}
func (c *Connector) Addr() string {
	return c.address
}
func (c *Connector) IsConnected() bool {
	return !c.sess.IsClose()
}

func (c *Connector) Close() {
	if c.IsClose() {
		return
	}
	c.mutex.Lock()
	c.closeflag = true
	c.sess.Close()
	c.mutex.Unlock()
	c.NotifyReconn()
	c.wg.Wait()
	SysLog.System("connection close, remote addr: %s", c.address)
}

func (c *Connector) IsClose() bool {
	return atomic.LoadUint32(&c.isclose) > 0
}

func (c *Connector) GetID() uint64 {
	return c.sess.GetID()
}

func (c *Connector) UserData() interface{} {
	return c.sess.UserData
}

func (c *Connector) SetUserData(data interface{}) {
	c.sess.UserData = data
}

func (c *Connector) Send(data []byte) error {
	c.NotifyReconn()
	return c.sess.Send(data)
}

func (c *Connector) AsyncSend(data []byte) error {
	c.NotifyReconn()
	return c.sess.AsyncSend(data)
}

func (c *Connector) NotifyReconn() {
	select {
	case c.reconnSignal <- 1:
	default:
	}
}
