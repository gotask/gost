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

func NewConnector(address string, reconnectmsec int, msgparse MsgParse, UserData interface{}) (*Connector, error) {
	if msgparse == nil {
		return nil, ErrMsgParseNil
	}

	conn := &Connector{
		sessCloseSignal: make(chan int, 1),
		address:         address,
		reconnectMSec:   reconnectmsec,
		wg:              &sync.WaitGroup{},
	}

	conn.Session, _ = newConnSession(msgparse, func(*Session) {
		conn.sessCloseSignal <- 1
	}, UserData)

	go conn.connect()

	return conn, nil
}

func NewConnectorNoStart(address string, reconnectmsec int, msgparse MsgParse, UserData interface{}) (*Connector, error) {
	if msgparse == nil {
		return nil, ErrMsgParseNil
	}

	conn := &Connector{
		sessCloseSignal: make(chan int, 1),
		address:         address,
		reconnectMSec:   reconnectmsec,
		wg:              &sync.WaitGroup{},
	}

	conn.isclose = 1

	conn.Session, _ = newConnSession(msgparse, func(*Session) {
		conn.sessCloseSignal <- 1
	}, UserData)

	return conn, nil
}

func (conn *Connector) connect() {
	conn.wg.Add(1)
	for !conn.closeflag {
		cn, err := net.Dial("tcp", conn.address)
		if err != nil {
			if conn.reconnectMSec <= 0 {
				break
			}
			time.Sleep(time.Duration(conn.reconnectMSec) * time.Millisecond)
			continue
		}

		conn.Session.restart(cn)

		<-conn.sessCloseSignal
		if conn.reconnectMSec <= 0 {
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

func (c *Connector) Start() {
	if atomic.CompareAndSwapUint32(&c.isclose, 1, 0) {
		go c.connect()
	}
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
