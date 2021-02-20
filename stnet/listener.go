package stnet

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Listener struct {
	isclose   *Closer
	address   string
	lst       net.Listener
	heartbeat uint32
	isUdp     bool
	udpConn   *net.UDPConn
	udpCh     chan int

	sessMap      map[uint64]*Session
	sessMapMutex sync.RWMutex
	waitExit     sync.WaitGroup
}

func NewListener(address string, msgparse MsgParse, heartbeat uint32) (*Listener, error) {
	if msgparse == nil {
		return nil, fmt.Errorf("MsgParse should not be nil")
	}

	ls, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	lis := &Listener{
		isclose:   NewCloser(false),
		address:   address,
		lst:       ls,
		heartbeat: heartbeat,
		sessMap:   make(map[uint64]*Session),
	}

	go func() {
		lis.waitExit.Add(1)
		for !lis.isclose.IsClose() {
			conn, err := lis.lst.Accept()
			if err != nil {
				SysLog.Error("accept error: %s", err.Error())
				break
			}

			lis.sessMapMutex.Lock()
			if !lis.isclose.IsClose() {
				lis.waitExit.Add(1)
				sess, _ := NewSession(conn, msgparse, nil, func(con *Session) {
					lis.sessMapMutex.Lock()
					delete(lis.sessMap, con.id)
					lis.waitExit.Done()
					lis.sessMapMutex.Unlock()
				}, heartbeat, false)
				lis.sessMap[sess.id] = sess
			}
			lis.sessMapMutex.Unlock()
		}
		lis.waitExit.Done()
	}()
	return lis, nil
}

func NewUdpListener(address string, msgparse MsgParse, heartbeat uint32) (*Listener, error) {
	if msgparse == nil {
		return nil, fmt.Errorf("MsgParse should not be nil")
	}
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	ls, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	lis := &Listener{
		isclose:   NewCloser(false),
		address:   address,
		isUdp:     true,
		udpConn:   ls,
		udpCh:     make(chan int, 1),
		heartbeat: heartbeat,
		sessMap:   make(map[uint64]*Session),
	}

	go func() {
		lis.waitExit.Add(1)

		for !lis.isclose.IsClose() {
			if err == nil {
				NewSession(lis.udpConn, msgparse, nil, func(con *Session) {
					lis.udpCh <- 1
				}, heartbeat, true)

				<-lis.udpCh
			}

			if !lis.isclose.IsClose() {
				ls, err = net.ListenUDP("udp", addr)
				if err != nil {
					SysLog.Error("udp listen failed: %s %s", address, err.Error())
					time.Sleep(time.Second * 3)
				} else {
					lis.udpConn = ls
				}
			}
		}

		lis.waitExit.Done()
	}()
	return lis, nil
}

func (ls *Listener) Close() {
	if ls.isclose.IsClose() {
		return
	}
	ls.isclose.Close()
	if ls.lst != nil {
		ls.lst.Close()
	}
	if ls.udpConn != nil {
		ls.udpConn.Close()
		ls.udpCh <- 1

		//send udp data to awake udpsocket
		addr := ls.udpConn.LocalAddr().(*net.UDPAddr)
		if tmpconn, err := net.DialUDP("udp", nil, addr); err == nil {
			tmpconn.Write([]byte(""))
			tmpconn.Close()
		}
	}
	ls.IterateSession(func(sess *Session) bool {
		sess.Close()
		return true
	})
	ls.waitExit.Wait()
}

func (ls *Listener) GetSession(id uint64) *Session {
	ls.sessMapMutex.RLock()
	defer ls.sessMapMutex.RUnlock()

	v, ok := ls.sessMap[id]
	if ok {
		return v
	}
	return nil
}

func (ls *Listener) IterateSession(callback func(*Session) bool) {
	ls.sessMapMutex.RLock()
	defer ls.sessMapMutex.RUnlock()

	for _, ses := range ls.sessMap {
		if !callback(ses) {
			break
		}
	}
}
