package stnet

import (
	"net"
	"sync"
	"time"
)

type Connector struct {
	sess            *Session
	network         string
	address         string
	reconnectMSec   int //Millisecond
	reconnCount     int
	closer          chan int
	closeLock       sync.Mutex
	sessCloseSignal chan int
	reconnSignal    chan int
	wg              *sync.WaitGroup
}

// NewConnector reconnect at 0s 1s 4s 9s 16s...;when call send changeAddr NotifyReconn, reconnect at once
func NewConnector(address string, msgparse MsgParse, userdata interface{}) *Connector {
	if msgparse == nil {
		panic(ErrMsgParseNil)
	}

	network, ipport := parseAddress(address)

	conn := &Connector{
		sessCloseSignal: make(chan int, 1),
		reconnSignal:    make(chan int, 1),
		closer:          make(chan int, 1),
		network:         network,
		address:         ipport,
		reconnectMSec:   100,
		wg:              &sync.WaitGroup{},
	}

	conn.sess, _ = newConnSession(msgparse, nil, func(*Session) {
		conn.sessCloseSignal <- 1
	}, conn, network == "udp")
	conn.sess.UserData = userdata

	go conn.connect()

	return conn
}

func (c *Connector) connect() {
	c.wg.Add(1)
	defer c.wg.Done()
	for !c.IsClose() {
		if c.reconnCount > 0 {
			to := time.NewTimer(time.Duration(c.reconnCount*c.reconnCount*c.reconnectMSec) * time.Millisecond)
			select {
			case <-c.closer:
				to.Stop()
				return
			case <-c.reconnSignal:
				to.Stop()
			case <-to.C:
				to.Stop()
			}
		}
		c.reconnCount++

		cn, err := net.Dial(c.network, c.address)
		if err != nil {
			c.sess.parser.sessionEvent(c.sess, Close)
			SysLog.Error("connect failed;addr=%s;error=%s", c.address, err.Error())
			if c.reconnectMSec <= 0 || c.IsClose() {
				break
			}
			continue
		}

		//maybe already be closed
		if c.IsClose() {
			break
		}
		c.closeLock.Lock()
		if c.IsClose() {
			cn.Close()
			c.closeLock.Unlock()
			break
		}
		c.sess.restart(cn)
		c.closeLock.Unlock()

		c.reconnCount = 0
		<-c.sessCloseSignal
		if c.reconnectMSec <= 0 || c.IsClose() {
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

	c.closeLock.Lock()
	close(c.closer)
	c.sess.Close()
	c.closeLock.Unlock()

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
}

func (c *Connector) GetID() uint64 {
	return c.sess.GetID()
}

func (c *Connector) Send(data []byte) error {
	c.NotifyReconn()
	return c.sess.Send(data, nil)
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
