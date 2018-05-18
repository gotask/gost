package stnet

import (
	"errors"
	"fmt"
	"reflect"
	"time"
)

const (
	SDPSERVERSUCCESS      = 0
	SDPSERVERUNKNOWNERR   = -999990
	SDPSERVERNOFUNCERR    = -999991
	SDPSERVERNOSERVICEERR = -999992
	SDPSERVERQUEUETIMEOUT = -999993
	SDPASYNCCALLTIMEOUT   = -999994
	SDPINVOKETIMEOUT      = -999995
	SDPPROXYCONNECTERR    = -999996
	SDPSERVEROVERLOAD     = -999997
	SDPADAPTERNULL        = -999998
	SDPRPCFUNCPARAMSEERR  = -999999
)

var (
	ErrRpcTimeOut     = errors.New("rpc timeout")
	ErrNoCallbackFunc = errors.New("no callback function or exception function")
	ErrNoRpcFunc      = errors.New("no rpc function")
	ErrRpcRspTimeOut  = errors.New("receive rsp but timeout")
)

type RequestPacket struct {
	IsOneWay    bool
	RequestId   uint32
	ServiceName string
	FuncName    string
	ReqPayload  string
	Timeout     uint32
	Context     map[string]string
}

type ResponsePacket struct {
	MfwRet     int32
	RequestId  uint32
	RspPayload string
	Context    map[string]string
}

type RPC struct {
	*Connect
	rpcimp      *RPCImp
	ServiceName string
	ReqSequence uint32
}

type ExceptionHander = func(int32)

type rpcRequest struct {
	req       RequestPacket
	callback  interface{}
	exception ExceptionHander
	timeout   int64
}

func (rpc *RPC) SyncCallWithCallbackAndException(funcName string, params ...interface{}) error { //the last two params should be callback function and exception function
	if len(params) < 2 || reflect.TypeOf(params[len(params)-2]).Kind() != reflect.Func {
		return ErrNoCallbackFunc
	}
	if _, ok := params[len(params)-1].(ExceptionHander); !ok {
		return ErrNoCallbackFunc
	}

	rpcReq := rpcRequest{}
	rpcReq.req.FuncName = funcName
	rpcReq.callback = params[len(params)-2]
	rpcReq.exception = params[len(params)-1].(ExceptionHander)
	params = params[0 : len(params)-2]
	return rpc.synccall(rpcReq, params...)
}

func (rpc *RPC) SyncCallWithCallback(funcName string, params ...interface{}) error {
	if len(params) == 0 || reflect.TypeOf(params[len(params)-1]).Kind() != reflect.Func {
		return ErrNoCallbackFunc
	}

	rpcReq := rpcRequest{}
	rpcReq.req.FuncName = funcName
	rpcReq.callback = params[len(params)-1]
	rpcReq.exception = nil
	params = params[0 : len(params)-1]
	return rpc.synccall(rpcReq, params...)
}

func (rpc *RPC) SyncCallWithException(funcName string, params ...interface{}) error {
	if len(params) == 0 {
		return ErrNoCallbackFunc
	}
	if _, ok := params[len(params)-1].(ExceptionHander); !ok {
		return ErrNoCallbackFunc
	}

	rpcReq := rpcRequest{}
	rpcReq.req.FuncName = funcName
	rpcReq.callback = nil
	rpcReq.exception = params[len(params)-1].(ExceptionHander)
	params = params[0 : len(params)-1]
	return rpc.synccall(rpcReq, params...)
}

func (rpc *RPC) SyncCall(funcName string, params ...interface{}) error {
	rpcReq := rpcRequest{}
	rpcReq.req.FuncName = funcName
	rpcReq.callback = nil
	rpcReq.exception = nil
	return rpc.synccall(rpcReq, params...)
}

func (rpc *RPC) synccall(rpcReq rpcRequest, params ...interface{}) error {
	rpcReq.timeout = time.Now().Unix() + 5
	rpcReq.req.ServiceName = rpc.ServiceName
	rpcReq.req.RequestId = rpc.ReqSequence

	sdp := Sdp{}
	for i, v := range params {
		err := sdp.pack(uint32(i+1), v, true, true)
		if err != nil {
			return fmt.Errorf("wrong params in synccall:%s", err.Error())
		}
	}
	rpcReq.req.ReqPayload = string(sdp.buf)

	rpc.imp.(*RPCImp).pushRequest(rpcReq)
	rpc.ReqSequence++

	return rpc.Send(PackSdpProtocol(Encode(rpcReq.req)))
}

func newRPC(name, servicename, address string) (*RPC, error) {
	rpcimp := &RPCImp{}
	ct, err := newConnect(name, address, 100, rpcimp)
	if err != nil {
		return nil, err
	}
	return &RPC{ct, rpcimp, servicename, 1}, nil
}

type RPCImp struct {
	requests map[uint32]rpcRequest
}

func (rpc *RPCImp) pushRequest(req rpcRequest) bool {
	rpc.requests[req.req.RequestId] = req
	return true
}

func (rpc *RPCImp) Init() bool {
	rpc.requests = make(map[uint32]rpcRequest)
	return true
}
func (rpc *RPCImp) Loop() {
	now := time.Now().Unix()
	for k, v := range rpc.requests {
		if v.timeout < now {
			if v.exception != nil {
				v.exception(SDPASYNCCALLTIMEOUT)
			}
			delete(rpc.requests, k)
		}
	}
}
func (rpc *RPCImp) Destroy() {

}

func (rpc *RPCImp) RegisterCMessage(ct *Connect) {
	ct.RegisterMessage(0, rpc.HandleCallBack)
}

func (rpc *RPCImp) HandleCallBack(s *Session, i interface{}) {
	rsp := i.(*ResponsePacket)
	v, ok := rpc.requests[rsp.RequestId]
	if !ok {
		rpc.HandleError(s, ErrRpcRspTimeOut)
		return
	}

	delete(rpc.requests, rsp.RequestId)

	if rsp.MfwRet != 0 {
		if v.exception != nil {
			v.exception(rsp.MfwRet)
		}
	} else {
		sdp := Sdp{[]byte(rsp.RspPayload), 0}

		if v.callback != nil {
			funcT := reflect.TypeOf(v.callback)
			funcVals := make([]reflect.Value, funcT.NumIn())
			for i := 0; i < funcT.NumIn(); i++ {
				t := funcT.In(i)
				val := newValByType(t)
				e := sdp.unpack(val, true)
				if e != nil {
					if v.exception != nil {
						v.exception(SDPRPCFUNCPARAMSEERR)
					}
					rpc.HandleError(s, e)
					return
				}
				funcVals[i] = val
			}
			funcV := reflect.ValueOf(v.callback)
			funcV.Call(funcVals)
		}
	}

}

func (rpc *RPCImp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := SdpLen(data)
	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}
	rsp := &ResponsePacket{}
	e := Decode(rsp, data[4:msgLen])
	if e != nil {
		return int(msgLen), 0, nil, e
	}
	return int(msgLen), 0, rsp, nil
}
func (rpc *RPCImp) Connected(sess *Session) {

}
func (rpc *RPCImp) DisConnected(sess *Session) {

}
func (rpc *RPCImp) HandleError(sess *Session, err error) {
	fmt.Println(err.Error())
}

type RPCServerImp struct {
	rpcFuncs interface{}
}

func (rpc *RPCServerImp) Init() bool {
	return true
}
func (rpc *RPCServerImp) Loop() {
}
func (rpc *RPCServerImp) Destroy() {

}
func (rpc *RPCServerImp) RegisterSMessage(s *Service) {
	s.RegisterMessage(0, rpc.HandleRpcRequest)
}
func (rpc *RPCServerImp) HandleRpcRequest(s *Session, i interface{}) {
	req := i.(*RequestPacket)

	m, ok := reflect.TypeOf(rpc.rpcFuncs).MethodByName(req.FuncName)
	if !ok {
		rpc.SendResponse(s, req, SDPSERVERNOFUNCERR, "")
		rpc.HandleError(s, fmt.Errorf("no rpc funciton:%s", req.FuncName))
		return
	}

	sdp := Sdp{[]byte(req.ReqPayload), 0}

	funcT := m.Type
	funcVals := make([]reflect.Value, funcT.NumIn())
	funcVals[0] = reflect.ValueOf(rpc.rpcFuncs)
	for i := 1; i < funcT.NumIn(); i++ {
		t := funcT.In(i)
		val := newValByType(t)
		e := sdp.unpack(val, true)
		if e != nil {
			rpc.SendResponse(s, req, SDPRPCFUNCPARAMSEERR, "")
			rpc.HandleError(s, e)
			return
		}
		funcVals[i] = val
	}
	funcV := m.Func
	returns := funcV.Call(funcVals)

	sdpSend := Sdp{}
	for i, v := range returns {
		e := sdpSend.pack(uint32(i+1), v.Interface(), true, true)
		if e != nil {
			rpc.SendResponse(s, req, SDPRPCFUNCPARAMSEERR, "")
			rpc.HandleError(s, e)
			return
		}
	}

	rpc.SendResponse(s, req, 0, string(sdpSend.buf))
}

func (rpc *RPCServerImp) SendResponse(s *Session, req *RequestPacket, ret int32, msg string) error {
	rsp := ResponsePacket{}
	rsp.MfwRet = ret
	rsp.RequestId = req.RequestId
	rsp.Context = req.Context
	rsp.RspPayload = msg
	return s.Send(PackSdpProtocol(Encode(rsp)))
}

func (rpc *RPCServerImp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := SdpLen(data)
	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}
	req := &RequestPacket{}
	e := Decode(req, data[4:msgLen])
	if e != nil {
		return int(msgLen), 0, nil, e
	}
	return int(msgLen), 0, req, nil
}
func (rpc *RPCServerImp) SessionOpen(sess *Session) {
}
func (rpc *RPCServerImp) SessionClose(sess *Session) {
}
func (rpc *RPCServerImp) HandleError(sess *Session, err error) {
	fmt.Println(err.Error())
}
