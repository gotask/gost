package stnet

import (
	"errors"

	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type CMDType int

const (
	Data CMDType = 0 + iota
	Open
	Close
	HeartBeat
	System
)

var (
	ErrSocketClosed   = errors.New("socket closed")
	ErrSocketIsOpen   = errors.New("socket is open")
	ErrSendOverTime   = errors.New("send message over time")
	ErrSendBuffIsFull = errors.New("send buffer is full")
	ErrMsgParseNil    = errors.New("MsgParse is nil")
)

type MsgParse interface {
	//*Session:session which recved message
	//CMDType:event type of msg
	//[]byte:recved data now;
	//int:length of recved data parsed;
	ParseMsg(sess *Session, data []byte) int

	sessionEvent(sess *Session, cmd CMDType)
}

//this will be called when session open
type FuncOnOpen = func(*Session)

//this will be called when session closed
type FuncOnClose = func(*Session)

//message recv buffer size
var (
	MsgBuffSize = 1024
	MinMsgSize  = 64

	//the length of send queue
	WriterListLen = 256
	RecvListLen   = 256
)

//session id
var GlobalSessionID uint64

type rsData struct {
	data []byte
	peer net.Addr
}

type Session struct {
	parser    MsgParse
	id        uint64
	socket    net.Conn
	writer    chan rsData
	hander    chan rsData
	closer    chan int
	wg        *sync.WaitGroup
	onopen    FuncOnOpen
	onclose   FuncOnClose
	isclose   *Closer
	heartbeat uint32
	conn      *Connector
	isUdp     bool
	peer      net.Addr

	UserData interface{}
}

func NewSession(con net.Conn, msgparse MsgParse, onopen FuncOnOpen, onclose FuncOnClose, heartbeat uint32, isudp bool) (*Session, error) {
	if msgparse == nil {
		return nil, ErrMsgParseNil
	}

	if isudp {
		WriterListLen = 10240
		RecvListLen = 10240
	}

	sess := &Session{
		id:        atomic.AddUint64(&GlobalSessionID, 1),
		socket:    con,
		writer:    make(chan rsData, WriterListLen), //It's OK to leave a Go channel open forever and never close it. When the channel is no longer used, it will be garbage collected.
		hander:    make(chan rsData, RecvListLen),
		closer:    make(chan int),
		wg:        &sync.WaitGroup{},
		parser:    msgparse,
		onopen:    onopen,
		onclose:   onclose,
		isclose:   NewCloser(false),
		heartbeat: heartbeat,
		isUdp:     isudp,
	}
	if isudp {
		SysLog.System("udp session start, local addr: %s", sess.socket.LocalAddr())
	} else {
		SysLog.System("tcp session start, local addr: %s, remote addr: %s", sess.socket.LocalAddr(), sess.socket.RemoteAddr())
	}
	asyncDo(sess.dosend, sess.wg)
	asyncDo(sess.dohand, sess.wg)

	go sess.dorecv()
	return sess, nil
}

func newConnSession(msgparse MsgParse, onopen FuncOnOpen, onclose FuncOnClose, c *Connector, isudp bool) (*Session, error) {
	if msgparse == nil {
		return nil, ErrMsgParseNil
	}

	if isudp {
		WriterListLen = 10240
		RecvListLen = 10240
	}

	sess := &Session{
		id:        atomic.AddUint64(&GlobalSessionID, 1),
		writer:    make(chan rsData, WriterListLen), //It's OK to leave a Go channel open forever and never close it. When the channel is no longer used, it will be garbage collected.
		hander:    make(chan rsData, RecvListLen),
		wg:        &sync.WaitGroup{},
		parser:    msgparse,
		onopen:    onopen,
		onclose:   onclose,
		isclose:   NewCloser(true),
		heartbeat: 0,
		conn:      c,
		isUdp:     isudp,
	}
	return sess, nil
}

func (s *Session) RemoteAddr() string {
	return s.peer.Network() + ":" + s.peer.String()
}

func (s *Session) Connector() *Connector {
	return s.conn
}

func (s *Session) handlePanic() {
	if err := recover(); err != nil {
		SysLog.Critical("panic error: %v", err)
		buf := make([]byte, 16384)
		buf = buf[:runtime.Stack(buf, true)]
		SysLog.Critical("panic stack: %s", string(buf))
		//close socket
		s.socket.Close()
	}
}

func (s *Session) restart(con net.Conn) error {
	if !s.isclose.IsClose() {
		return ErrSocketIsOpen
	}
	s.closer = make(chan int)
	s.socket = con
	//writer buffer not should be cleanup
	//s.writer = make(chan rsData, WriterListLen)
	//receive buffer maybe half part,so should be cleanup
	s.hander = make(chan rsData, RecvListLen)

	if s.isUdp {
		SysLog.System("udp session restart, local addr: %s", s.socket.LocalAddr())
	} else {
		SysLog.System("tcp session restart, local addr: %s, remote addr: %s", s.socket.LocalAddr(), s.socket.RemoteAddr())
	}

	asyncDo(s.dosend, s.wg)
	asyncDo(s.dohand, s.wg)
	go s.dorecv()
	return nil
}

func (s *Session) GetID() uint64 {
	return s.id
}

func (s *Session) Peer() net.Addr {
	return s.peer
}

func (s *Session) Send(data []byte, peer net.Addr) error {
	msg := bp.Alloc(len(data))
	copy(msg, data)

	select {
	case <-s.closer:
		return ErrSocketClosed
	case s.writer <- rsData{msg, peer}:
		return nil
	default:
		SysLog.Error("session sending queue is full and the message is droped;sessionid=%d", s.id)
		return ErrSendBuffIsFull
	}
}

func (s *Session) Close() {
	if s.IsClose() {
		return
	}
	s.socket.Close()
}

func (s *Session) IsClose() bool {
	return s.isclose.IsClose()
}

func (s *Session) dosend() {
	var udpConn *net.UDPConn
	if s.isUdp {
		udpConn = s.socket.(*net.UDPConn)
	}

	for {
		select {
		case <-s.closer:
			return
		case buf := <-s.writer:
			if s.isUdp {
				if buf.peer == nil || s.conn != nil {
					udpConn.Write(buf.data)
				} else {
					udpConn.WriteTo(buf.data, buf.peer)
				}
			} else {
				//s.socket.SetWriteDeadline(time.Now().Add(time.Millisecond * 300))
				n := 0
				for n < len(buf.data) {
					n1, err := s.socket.Write(buf.data[n:])
					if err != nil {
						SysLog.Error("session sending error: %s;sessionid=%d", err.Error(), s.id)
						s.socket.Close()
						bp.Free(buf.data)
						return
					}
					n += n1
				}
			}
			bp.Free(buf.data)
		}
	}
}

func (s *Session) dorecv() {
	defer func() {
		//close socket
		s.socket.Close()
		select {
		case <-s.closer:
		default:
			close(s.closer)
		}
		s.wg.Wait()
		s.isclose.Close()
		if s.isUdp {
			SysLog.System("udp session close, local addr: %s", s.socket.LocalAddr())
		} else {
			SysLog.System("tcp session close, local addr: %s, remote addr: %s", s.socket.LocalAddr(), s.socket.RemoteAddr())
		}
		s.parser.sessionEvent(s, Close)
		s.onclose(s)
	}()

	if s.onopen != nil {
		s.onopen(s)
	}
	s.parser.sessionEvent(s, Open)

	var (
		udpConn *net.UDPConn
		peer    net.Addr
		n       int
		err     error
	)

	if s.isUdp {
		udpConn = s.socket.(*net.UDPConn)
	} else {
		peer = s.socket.RemoteAddr()
	}

	msgbuf := bp.Alloc(MsgBuffSize)
	for {
		if s.isUdp {
			n, peer, err = udpConn.ReadFrom(msgbuf)
		} else {
			n, err = s.socket.Read(msgbuf)
		}
		if err != nil || n == 0 {
			SysLog.Error("session recv error: %s,n: %d", err.Error(), n)
			//defer close
			return
		}
		s.hander <- rsData{msgbuf[0:n], peer}
		if s.isUdp {
			msgbuf = bp.Alloc(MsgBuffSize)
			continue
		}

		bufLen := len(msgbuf)
		if MinMsgSize < bufLen && n*2 < bufLen {
			msgbuf = bp.Alloc(bufLen / 2)
		} else if n == bufLen {
			msgbuf = bp.Alloc(bufLen * 2)
		} else {
			msgbuf = bp.Alloc(bufLen)
		}
	}
}

func (s *Session) dohand() {
	defer s.handlePanic()

	wt := time.Second * time.Duration(s.heartbeat)
	ht := time.NewTimer(wt)
	if s.heartbeat == 0 {
		ht.Stop()
	} else {
		defer ht.Stop()
	}
	var tempBuf []byte
	for {
		if s.heartbeat > 0 {
			if !ht.Stop() {
				select {
				case <-ht.C:
				default:
				}
			}
			ht.Reset(wt)
		}
		select {
		case <-s.closer:
			return
		case <-ht.C:
			s.parser.sessionEvent(s, HeartBeat)
		case buf := <-s.hander:
			s.peer = buf.peer
			if tempBuf != nil {
				buf.data = append(tempBuf, buf.data...)
			}
		anthorMsg:
			parseLen := s.parser.ParseMsg(s, buf.data)
			if parseLen >= len(buf.data) {
				tempBuf = nil
				bp.Free(buf.data)
			} else if parseLen > 0 {
				buf.data = buf.data[parseLen:]
				goto anthorMsg
			} else if parseLen == 0 {
				if s.isUdp {
					SysLog.Error("udp need read all data once,length: %d, local addr: %s, remote addr: %s", len(buf.data), s.socket.LocalAddr(), buf.peer.String())
					continue
				}
				tempBuf = buf.data
			}
		}
	}
}

func asyncDo(fn func(), wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		fn()
		wg.Done()
	}()
}
