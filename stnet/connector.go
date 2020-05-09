package stnet

import (
	"net"
	"sync"
	"time"
)

type Connector struct {
	sess            *Session
	address         string
	reconnectMSec   int //Millisecond
	reconnCount     int
	closer          chan int
	sessCloseSignal chan int
	reconnSignal    chan int
	wg              *sync.WaitGroup
}

//reconnect at 0s 1s 4s 9s 16s...;when call send changeAddr NotifyReconn, reconnect at once
func NewConnector(address string, msgparse MsgParse, userdata interface{}) *Connector {
	if msgparse == nil {
		panic(ErrMsgParseNil)
	}

	conn := &Connector{
		sessCloseSignal: make(chan int, 1),
		reconnSignal:    make(chan int, 1),
		closer:          make(chan int, 1),
		address:         address,
		reconnectMSec:   1000,
		wg:              &sync.WaitGroup{},
	}

	conn.sess, _ = newConnSession(msgparse, nil, func(*Session) {
		conn.sessCloseSignal <- 1
	}, conn)
	conn.sess.UserData = userdata

	go conn.connect()

	return conn
}

func (conn *Connector) connect() {
	conn.wg.Add(1)
	defer conn.wg.Done()
	for !conn.IsClose() {
		if conn.reconnCount > 0 {
			to := time.NewTimer(time.Duration(conn.reconnCount*conn.reconnCount*conn.reconnectMSec) * time.Millisecond)
			select {
			case <-conn.closer:
				to.Stop()
				return
			case <-conn.reconnSignal:
				to.Stop()
			case <-to.C:
				to.Stop()
			}
		}
		conn.reconnCount++

		cn, err := net.Dial("tcp", conn.address)
		if err != nil {
			conn.sess.parser.sessionEvent(conn.sess, Close)
			SysLog.Error("connect failed;addr=%s;error=%s", conn.address, err.Error())
			if conn.reconnectMSec < 0 || conn.IsClose() {
				break
			}
			continue
		}

		//maybe already be closed
		if conn.IsClose() {
			break
		}
		conn.sess.restart(cn)
		conn.reconnCount = 0
		<-conn.sessCloseSignal
		if conn.reconnectMSec < 0 || conn.IsClose() {
			break
		}
	}
}

func (c *Connector) ChangeAddr(addr string) {
	c.address = addr
	c.sess.Close() //close socket,wait for reconnecting
	c.NotifyReconn()
}

func (c *Connector) Addr() string {
	return c.address
}

func (c *Connector) ReconnCount() int {
	return c.reconnCount
}

func (c *Connector) IsConnected() bool {
	if c.IsClose() {
		return false
	}
	return !c.sess.IsClose()
}

func (c *Connector) Close() {
	if c.IsClose() {
		return
	}
	close(c.closer)
	c.wg.Wait()
	SysLog.System("connection close, remote addr: %s", c.address)
}

func (c *Connector) IsClose() bool {
	select {
	case <-c.closer:
		return true
	default:
		return false
	}
	return false
}

func (c *Connector) GetID() uint64 {
	return c.sess.GetID()
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

func (c *Connector) Session() *Session {
	return c.sess
}
