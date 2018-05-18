package stnet

import (
	"errors"

	"net"
	"sync"
	"sync/atomic"
	"time"
)

type CMDType int

const (
	Data CMDType = 0 + iota
	Open
	Close
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

	SessionEvent(sess *Session, cmd CMDType)
}

//this will be called when session closed
type FuncOnClose func(*Session)

//message recv buffer size
const (
	MsgBuffSize = 1024
	MinMsgSize  = 64

	//the length of send queue
	WriterListLen = 256
	RecvListLen   = 256
)

//session id
var GlobalSessionID uint64

type Session struct {
	MsgParse

	id      uint64
	socket  net.Conn
	writer  chan []byte
	hander  chan []byte
	closer  chan int
	wg      *sync.WaitGroup
	onclose FuncOnClose
	isclose uint32

	UserData interface{}
}

func NewSession(con net.Conn, msgparse MsgParse, onclose FuncOnClose) (*Session, error) {
	if msgparse == nil {
		return nil, ErrMsgParseNil
	}
	sess := &Session{
		id:       atomic.AddUint64(&GlobalSessionID, 1),
		socket:   con,
		writer:   make(chan []byte, WriterListLen), //It's OK to leave a Go channel open forever and never close it. When the channel is no longer used, it will be garbage collected.
		hander:   make(chan []byte, RecvListLen),
		closer:   make(chan int),
		wg:       &sync.WaitGroup{},
		MsgParse: msgparse,
		onclose:  onclose,
	}
	asyncDo(sess.dosend, sess.wg)
	asyncDo(sess.dohand, sess.wg)

	go sess.dorecv()
	return sess, nil
}

func newConnSession(msgparse MsgParse, onclose FuncOnClose, UserData interface{}) (*Session, error) {
	if msgparse == nil {
		return nil, ErrMsgParseNil
	}
	sess := &Session{
		id:       atomic.AddUint64(&GlobalSessionID, 1),
		writer:   make(chan []byte, WriterListLen), //It's OK to leave a Go channel open forever and never close it. When the channel is no longer used, it will be garbage collected.
		hander:   make(chan []byte, RecvListLen),
		closer:   make(chan int),
		wg:       &sync.WaitGroup{},
		MsgParse: msgparse,
		onclose:  onclose,
		UserData: UserData,
	}
	close(sess.closer)
	sess.isclose = 1
	return sess, nil
}

func (s *Session) restart(con net.Conn) error {
	if !atomic.CompareAndSwapUint32(&s.isclose, 1, 0) {
		return ErrSocketIsOpen
	}
	s.closer = make(chan int)
	s.socket = con
	asyncDo(s.dosend, s.wg)
	asyncDo(s.dohand, s.wg)
	go s.dorecv()
	return nil
}

func (s *Session) GetID() uint64 {
	return s.id
}

func (s *Session) Send(data []byte) error {
	msg := bp.Alloc(len(data))
	copy(msg, data)
	for {
		select {
		case <-s.closer:
			return ErrSocketClosed
		case s.writer <- msg:
			return nil
		case <-time.After(100 * time.Millisecond):
			return ErrSendOverTime
		}
	}
}

func (s *Session) AsyncSend(data []byte) error {
	msg := bp.Alloc(len(data))
	copy(msg, data)
	for {
		select {
		case <-s.closer:
			return ErrSocketClosed
		case s.writer <- msg:
			return nil
		default:
			return ErrSendBuffIsFull
		}
	}
}

func (s *Session) Close() {
	s.socket.Close()
}

func (s *Session) IsClose() bool {
	return atomic.LoadUint32(&s.isclose) > 0
}

func (s *Session) dosend() {
	for {
		select {
		case <-s.closer:
			return
		case buf := <-s.writer:
			if _, err := s.socket.Write(buf); err != nil {
				s.socket.Close()
				return
			}
			bp.Free(buf)
		}
	}
}

func (s *Session) dorecv() {
	s.SessionEvent(s, Open)

	msgbuf := bp.Alloc(MsgBuffSize)
	for {
		n, err := s.socket.Read(msgbuf)
		if err != nil {
			s.SessionEvent(s, Close)
			s.socket.Close()
			close(s.closer)
			s.wg.Wait()
			atomic.AddUint32(&s.isclose, 1)
			s.onclose(s)
			return
		}
		s.hander <- msgbuf[0:n]

		bufLen := len(msgbuf)
		if MinMsgSize < bufLen && n*2 < bufLen {
			msgbuf = bp.Alloc(bufLen / 2)
		} else if n == bufLen {
			msgbuf = bp.Alloc(bufLen * 2)
		}
	}
}

func (s *Session) dohand() {
	var tempBuf []byte
	for {
		select {
		case <-s.closer:
			return
		case buf := <-s.hander:
			if tempBuf != nil {
				buf = append(tempBuf, buf...)
			}
		anthorMsg:
			parseLen := s.ParseMsg(s, buf)
			tempBuf = buf[parseLen:]
			if parseLen >= len(buf) {
				tempBuf = nil
				bp.Free(buf)
			} else if parseLen > 0 {
				buf = tempBuf
				goto anthorMsg
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
