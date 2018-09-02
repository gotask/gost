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
}

func NewConnector(address string, reconnectmsec int, msgparse MsgParse, onconnected FuncOnOpen) *Connector {
	if msgparse == nil {
		panic(ErrMsgParseNil)
	}

	conn := &Connector{
		sessCloseSignal: make(chan int, 1),
		address:         address,
		reconnectMSec:   reconnectmsec,
		wg:              &sync.WaitGroup{},
	}

	conn.Session, _ = newConnSession(msgparse, onconnected, func(*Session) {
		conn.sessCloseSignal <- 1
	})

	go conn.connect()

	return conn
}

func (conn *Connector) connect() {
	conn.wg.Add(1)
	for !conn.closeflag {
		cn, err := net.Dial("tcp", conn.address)
		if err != nil {
			SysLog.Error("connect failed;addr=%s;error=%s", conn.address, err.Error())
			if conn.reconnectMSec <= 0 || conn.closeflag {
				break
			}
			time.Sleep(time.Duration(conn.reconnectMSec) * time.Millisecond)
			continue
		}

		conn.Session.restart(cn)

		<-conn.sessCloseSignal
		if conn.reconnectMSec <= 0 || conn.closeflag {
			break
		}
		time.Sleep(time.Duration(conn.reconnectMSec) * time.Millisecond)
	}
	atomic.CompareAndSwapUint32(&conn.isclose, 0, 1)
	conn.wg.Done()
}

func (cnt *Connector) IsConnected() bool {
	return !cnt.Session.IsClose()
}

func (c *Connector) Close() {
	if c.IsClose() {
		return
	}
	c.closeflag = true
	c.Session.Close()
	c.wg.Wait()
}

func (c *Connector) IsClose() bool {
	return atomic.LoadUint32(&c.isclose) > 0
}
