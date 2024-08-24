package stnet

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"sync"
	"time"
)

const (
	RpcErrNoRemoteFunc    = -1
	RpcErrCallTimeout     = -2
	RpcErrFuncParamErr    = -3
	RotErrNoRemoteService = -4
)

type ReqProto struct {
	ReqCmdSeq uint32
	ReqData   []byte
	IsOneWay  bool
	FuncName  string
}

type RspProto struct {
	RspCmdSeq uint32
	RspCode   int32
	RspData   []byte
	FuncName  string
}

type PushProto struct {
	SrcName string
	PushSeq uint32
	CmdSeq  uint32
	CmdData []byte
}

type RpcService interface {
	Loop()
	HandleError(current *CurrentContent, err error)
	//HashProcessor if processorID == 0, it only uses main thread of the service.
	//if processorID < 0, it will use hash of session id.
	HashProcessor(current *CurrentContent) (processorID int)
}

type RpcServiceEx interface {
	Loop()
	HandleError(current *CurrentContent, err error)
	//HashProcessor if processorID == 0, it only uses main thread of the service.
	//if processorID < 0, it will use hash of session id.
	HashProcessor(current *CurrentContent) (processorID int)

	HandlePush(current *CurrentContent, msg *PushProto)
}

type SimpRpcService struct {
	imp RpcService
}

func (r *SimpRpcService) Loop() {
	r.imp.Loop()
}
func (r *SimpRpcService) HandleError(current *CurrentContent, err error) {
	r.imp.HandleError(current, err)
}
func (r *SimpRpcService) HashProcessor(c *CurrentContent) (processorID int) {
	return r.imp.HashProcessor(c)
}
func (r *SimpRpcService) HandlePush(current *CurrentContent, msg *PushProto) {
	sysLog.Error("HandlePush is null")
}

type NullRpcService struct {
}

func (r *NullRpcService) Loop() {
}
func (r *NullRpcService) HandleError(current *CurrentContent, err error) {
	sysLog.Error("rpc error: %s", err.Error())
}
func (r *NullRpcService) HashProcessor(*CurrentContent) (processorID int) {
	return 0
}
func (r *NullRpcService) HandlePush(current *CurrentContent, msg *PushProto) {
	sysLog.Error("HandlePush is null")
}

type RpcFuncException func(rspCode int32)

type rpcRequest struct {
	req       ReqProto
	callback  interface{}
	exception RpcFuncException
	timeout   int64
	sess      *Session

	signal chan *RspProto
}

type ServiceRpc struct {
	ServiceBase
	base    *Service
	imp     RpcService
	impEx   RpcServiceEx
	methods map[string]reflect.Method

	rpcRequests    map[uint32]*rpcRequest
	rpcReqSequence uint32
	rpcMutex       sync.Mutex

	rpcTimeOutMs int64
}

func newServiceRpc(imp RpcService, rpcTimeOutMilli int64) *ServiceRpc {
	return newRpcService(imp, &SimpRpcService{imp}, rpcTimeOutMilli)
}

func newServiceRpcEx(imp RpcServiceEx, rpcTimeOutMilli int64) *ServiceRpc {
	return newRpcService(imp, imp, rpcTimeOutMilli)
}

func newRpcService(imp RpcService, impEx RpcServiceEx, rpcTimeOutMilli int64) *ServiceRpc {
	if rpcTimeOutMilli <= 10 {
		rpcTimeOutMilli = 3000 //default 3s
	}

	svr := &ServiceRpc{}
	svr.imp = imp
	svr.impEx = impEx
	svr.rpcRequests = make(map[uint32]*rpcRequest)
	svr.methods = make(map[string]reflect.Method)
	svr.rpcTimeOutMs = rpcTimeOutMilli

	t := reflect.TypeOf(imp)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		svr.methods[m.Name] = m
	}

	delete(svr.methods, "Loop")
	delete(svr.methods, "HandleError")
	delete(svr.methods, "HashProcessor")
	delete(svr.methods, "HandlePush")

	return svr
}

// rpc_call syncORasync remotesession udppeer remotefunction functionparams callback exception
// rpc_call exception function: func(int32){}
func (service *ServiceRpc) rpc_call(isSync bool, isRouter bool, sess *Session, peer net.Addr, funcName string, params ...interface{}) error {
	rpcRequestMetricAdd()

	var rpcReq rpcRequest
	rpcReq.timeout = time.Now().UnixMilli() + service.rpcTimeOutMs
	rpcReq.req.FuncName = funcName

	//have callback function and exception function
	if len(params) > 0 && (params[len(params)-1] == nil || reflect.TypeOf(params[len(params)-1]).Kind() == reflect.Func) {
		if len(params) < 2 {
			return errors.New("no callback function or exception function")
		}
		if params[len(params)-1] != nil {
			if ex, ok := params[len(params)-1].(func(int32)); ok {
				rpcReq.exception = ex
			} else {
				return errors.New("invalid exception function")
			}
		}
		if params[len(params)-2] != nil {
			param2Type := reflect.TypeOf(params[len(params)-2])
			if param2Type.Kind() != reflect.Func {
				return errors.New("invalid callback function")
			}
		}
		rpcReq.callback = params[len(params)-2]

		params = params[0 : len(params)-2]
	}

	var err error
	spb := Spb{}
	for i, v := range params {
		err = RpcMarshal(&spb, uint32(i+1), v)
		if err != nil {
			return fmt.Errorf("wrong params in RpcCall:%s(%d) %s", funcName, i, err.Error())
		}
	}
	rpcReq.req.ReqData = spb.buf
	if rpcReq.callback == nil && rpcReq.exception == nil {
		rpcReq.req.IsOneWay = true
	}

	service.rpcMutex.Lock()
	service.rpcReqSequence++
	rpcReq.req.ReqCmdSeq = service.rpcReqSequence
	rpcReq.sess = sess
	if isSync {
		rpcReq.signal = make(chan *RspProto, 1)
	}
	service.rpcRequests[rpcReq.req.ReqCmdSeq] = &rpcReq
	service.rpcMutex.Unlock()

	if isRouter {
		err = service.sendRouterRpcReq(sess, peer, &rpcReq.req)
	} else {
		err = service.sendRpcReq(sess, peer, &rpcReq.req)
	}
	if err != nil {
		return err
	}

	if !isSync {
		return nil
	}

	to := time.NewTimer(time.Duration(service.rpcTimeOutMs) * time.Millisecond)
	select {
	case rsp := <-rpcReq.signal:
		service.handleRpcRsp(rsp)
	case <-to.C:
		isOk := false
		service.rpcMutex.Lock()
		_, isOk = service.rpcRequests[rpcReq.req.ReqCmdSeq]
		if isOk {
			delete(service.rpcRequests, rpcReq.req.ReqCmdSeq)
		}
		service.rpcMutex.Unlock()

		if isOk {
			rpcFailedMetricAdd()
			rpcOverTimeMetricAdd()
			if rpcReq.exception != nil {
				rpcReq.exception(RpcErrCallTimeout)
			}
		}
	}
	to.Stop()

	return nil
}

// RpcCall remoteSess remoteFunc(string) funcParams callback(could nil) exception(could nil, func(rspCode int32))
// example rpc.RpcCall(c.Session(), "Add", 1, 2, func(result int) {}, func(exception int32) {})
// example rpc.RpcCall(c.Session(), "Ping")
func (service *ServiceRpc) RpcCall(sess *Session, funcName string, params ...interface{}) error {
	return service.rpc_call(false, false, sess, nil, funcName, params...)
}
func (service *ServiceRpc) RpcCall_Sync(sess *Session, funcName string, params ...interface{}) error {
	return service.rpc_call(true, false, sess, nil, funcName, params...)
}
func (service *ServiceRpc) UdpRpcCall(sess *Session, peer net.Addr, funcName string, params ...interface{}) error {
	return service.rpc_call(false, false, sess, peer, funcName, params...)
}
func (service *ServiceRpc) UdpRpcCall_Sync(sess *Session, peer net.Addr, funcName string, params ...interface{}) error {
	return service.rpc_call(true, false, sess, peer, funcName, params...)
}

func (service *ServiceRpc) Init() bool {
	return true
}

func (service *ServiceRpc) Loop() {
	now := time.Now().UnixMilli()
	timeouts := make([]*rpcRequest, 0)
	service.rpcMutex.Lock()
	for k, v := range service.rpcRequests {
		if v.timeout < now && v.signal == nil {
			if v.exception != nil {
				timeouts = append(timeouts, v)
			}
			delete(service.rpcRequests, k)
			rpcFailedMetricAdd()
			rpcOverTimeMetricAdd()
		}
	}
	service.rpcMutex.Unlock()

	for _, v := range timeouts {
		v.exception(RpcErrCallTimeout)
	}

	service.imp.Loop()
}

func (service *ServiceRpc) handleRouterRpcReq(current *CurrentContent, req *RouterMsgReq) {
	r := &ReqProto{}
	e := Unmarshal(req.RawData, r, 0)
	if e != nil {
		rpcRemoteReqMetricAdd()
		rpcInvalidReqMetricAdd()

		sysLog.Error("%s->%s RouterMsgReq.RawData unmarshal failed: %s", req.SrcAddr, req.DstService, e.Error())
		return
	}

	rsp := service.handleRpcReq(current, r)
	sendRouterRsp(current, req, rsp, 0x7)
}

func (service *ServiceRpc) handleRpcReq(current *CurrentContent, req *ReqProto) *RspProto {
	rpcRemoteReqMetricAdd()

	var rsp RspProto
	rsp.RspCmdSeq = req.ReqCmdSeq
	rsp.FuncName = req.FuncName

	m, ok := service.methods[req.FuncName]
	if !ok {
		rpcInvalidReqMetricAdd()
		rsp.RspCode = RpcErrNoRemoteFunc
		sysLog.Error("no rpc function: %s", req.FuncName)
		return &rsp
	}

	spb := Spb{req.ReqData, 0}

	var e error
	funcT := m.Type
	funcVals := make([]reflect.Value, funcT.NumIn())
	funcVals[0] = reflect.ValueOf(service.imp)
	for i := 1; i < funcT.NumIn(); i++ {
		t := funcT.In(i)
		val := newValByType(t)
		e = RpcUnmarshal(&spb, uint32(i), val.Interface())
		if e != nil {
			rpcInvalidReqMetricAdd()
			rsp.RspCode = RpcErrFuncParamErr
			sysLog.Error("function %s param unpack failed: %s", req.FuncName, e.Error())
			return &rsp
		}
		if t.Kind() == reflect.Ptr {
			funcVals[i] = val
		} else {
			funcVals[i] = val.Elem()
		}
	}
	funcV := m.Func
	returns := funcV.Call(funcVals)

	if req.IsOneWay {
		return nil
	}

	spbSend := Spb{}
	for i, v := range returns {
		e = RpcMarshal(&spbSend, uint32(i+1), v.Interface())
		if e != nil {
			rsp.RspCode = RpcErrFuncParamErr
			sysLog.Error("function %s param pack failed: %s", req.FuncName, e.Error())
			return &rsp
		}
	}
	rsp.RspData = spbSend.buf
	return &rsp
}

func (service *ServiceRpc) handleRpcRsp(rsp *RspProto) {
	service.rpcMutex.Lock()
	v, ok := service.rpcRequests[rsp.RspCmdSeq]
	if !ok {
		service.rpcMutex.Unlock()
		sysLog.Error("recv rpc rsp but req not found,%d %d func: %s", rsp.RspCmdSeq, rsp.RspCode, rsp.FuncName)
		return
	}
	delete(service.rpcRequests, rsp.RspCmdSeq)
	service.rpcMutex.Unlock()

	if rsp.RspCode != 0 {
		rpcFailedMetricAdd()

		if v.exception != nil {
			v.exception(rsp.RspCode)
		}
	} else {
		spb := Spb{rsp.RspData, 0}

		if v.callback != nil {
			var e error
			funcT := reflect.TypeOf(v.callback)
			funcVals := make([]reflect.Value, funcT.NumIn())
			for i := 0; i < funcT.NumIn(); i++ {
				t := funcT.In(i)
				val := newValByType(t)
				e = RpcUnmarshal(&spb, uint32(i+1), val.Interface())
				if e != nil {
					rpcFailedMetricAdd()
					if v.exception != nil {
						v.exception(RpcErrFuncParamErr)
					}
					sysLog.Error("recv rpc rsp but unpack failed, func:%s,%s", rsp.FuncName, e.Error())
					return
				}
				if t.Kind() == reflect.Ptr {
					funcVals[i] = val
				} else {
					funcVals[i] = val.Elem()
				}
			}
			funcV := reflect.ValueOf(v.callback)
			funcV.Call(funcVals)
		}
		rpcSuccessMetricAdd()
	}
}

const (
	TyMsgRpcReq       = 0
	TyMsgRpcRsp       = 1
	TyMsgRouterRpcReq = 2
	TyMsgRouterRpcRsp = 3
	TyMsgPush         = 5
	TyMsgRouterPush   = 6
)

func (service *ServiceRpc) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {
	if msgID == TyMsgRpcReq { //rpc req
		rsp := service.handleRpcReq(current, msg.(*ReqProto))
		if rsp != nil {
			service.sendRpcRsp(current, rsp)
		}
	} else if msgID == TyMsgRpcRsp { //rpc rsp
		service.handleRpcRsp(msg.(*RspProto))
	} else if msgID == TyMsgPush { //push
		rpcRecvPushMetricAdd()
		service.impEx.HandlePush(current, msg.(*PushProto))
	} else if msgID == TyMsgRouterRpcReq { //router rpc req
		service.handleRouterRpcReq(current, msg.(*RouterMsgReq))
	} else if msgID == TyMsgRouterRpcRsp { //router rpc rsp
		service.handleRpcRsp(msg.(*RspProto))
	} else if msgID == TyMsgRouterPush { //push
		rpcRecvPushMetricAdd()
		service.impEx.HandlePush(current, msg.(*PushProto))
	} else {
		sysLog.Error("invalid msgid %d", msgID)
	}
}

func (service *ServiceRpc) HandleError(current *CurrentContent, err error) {
	service.imp.HandleError(current, err)
}

func (service *ServiceRpc) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := msgLen(data)
	if msgLen < 4 || msgLen >= uint32(MaxMsgSize) {
		return len(data), 0, nil, fmt.Errorf("message length is invalid: %d", msgLen)
	}

	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}

	flag := data[0]
	if flag&0x2 == 0 { //push
		if flag&0x1 == 0 {
			if flag&0x4 == 0 {
				req := &PushProto{}
				e := Unmarshal(data[4:msgLen], req, 0)
				if e != nil {
					return int(msgLen), 0, nil, e
				}
				return int(msgLen), TyMsgPush, req, nil
			} else { //router
				req := &RouterMsgPush{}
				e := Unmarshal(data[4:msgLen], req, 0)
				if e != nil {
					return int(msgLen), 0, nil, e
				}

				r := &PushProto{}
				e = Unmarshal(req.RawData, r, 0)
				if e != nil {
					sysLog.Error("%s@%s PushProto.RawData unmarshal failed: %s", req.SrcService, req.SrcAddr, e.Error())
					return int(msgLen), 0, nil, e
				}
				return int(msgLen), TyMsgRouterPush, r, nil
			}
		}
	} else { //rpc
		if flag&0x1 == 0 { //req
			if flag&0x4 == 0 {
				req := &ReqProto{}
				e := Unmarshal(data[4:msgLen], req, 0)
				if e != nil {
					return int(msgLen), 0, nil, e
				}
				return int(msgLen), TyMsgRpcReq, req, nil
			} else { //router
				req := &RouterMsgReq{}
				e := Unmarshal(data[4:msgLen], req, 0)
				if e != nil {
					return int(msgLen), 0, nil, e
				}
				return int(msgLen), TyMsgRouterRpcReq, req, nil
			}
		} else { //rsp
			if flag&0x4 == 0 {
				rsp := &RspProto{}
				e := Unmarshal(data[4:msgLen], rsp, 0)
				if e != nil {
					return int(msgLen), 0, nil, e
				}

				rpcResponseMetricAdd()

				service.rpcMutex.Lock()
				v, ok := service.rpcRequests[rsp.RspCmdSeq]
				if ok && v.signal != nil { //sync call
					v.signal <- rsp
					service.rpcMutex.Unlock()
					return int(msgLen), -1, nil, nil
				}
				service.rpcMutex.Unlock()
				//async call
				return int(msgLen), TyMsgRpcRsp, rsp, nil
			} else { //router
				rsp := &RouterMsgRsp{}
				e := Unmarshal(data[4:msgLen], rsp, 0)
				if e != nil {
					return int(msgLen), 0, nil, e
				}

				rpcResponseMetricAdd()

				r := &RspProto{}
				e = Unmarshal(rsp.RawData, r, 0)
				if e != nil {
					sysLog.Error("%s->%s RouterMsgRsp.RawData unmarshal failed: %s", rsp.FromAddr, rsp.FromService, e.Error())
					return int(msgLen), 0, nil, e
				}

				service.rpcMutex.Lock()
				v, ok := service.rpcRequests[r.RspCmdSeq]
				if ok && v.signal != nil { //sync call
					v.signal <- r
					service.rpcMutex.Unlock()
					return int(msgLen), -1, nil, nil
				}
				service.rpcMutex.Unlock()
				//async call
				return int(msgLen), TyMsgRouterRpcRsp, r, nil
			}
		}
	}

	return int(msgLen), 0, nil, fmt.Errorf("message is invalid,flag: %d", flag)
}

func (service *ServiceRpc) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return service.imp.HashProcessor(current)
}

func (service *ServiceRpc) Push(ps *PushProto) error {
	buf, e := encodeProtocol(ps, 0)
	if e != nil {
		return e
	}
	service.base.IterateSession(func(sess *Session) bool {
		rpcSendPushMetricAdd()
		sess.Send(buf, nil)
		return true
	})
	return nil
}

func (service *ServiceRpc) sendRpcReq(sess *Session, peer net.Addr, req *ReqProto) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	buf, e := encodeProtocol(req, 0)
	if e != nil {
		return e
	}
	buf[0] |= 0x2
	return sess.Send(buf, peer)
}

func (service *ServiceRpc) sendRpcRsp(current *CurrentContent, rsp *RspProto) error {
	if rsp == nil {
		return fmt.Errorf("response is nil")
	}
	buf, e := encodeProtocol(rsp, 0)
	if e != nil {
		return e
	}
	buf[0] |= 0x3
	return current.Sess.Send(buf, current.Peer)
}

func (service *ServiceRpc) sendRouterRpcReq(sess *Session, peer net.Addr, req *ReqProto) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	i := strings.Index(req.FuncName, "@")
	if i <= 0 {
		return fmt.Errorf("RouterRpc request is nil,FuncName=%s", req.FuncName)
	}
	serviceName := req.FuncName[i+1:]
	req.FuncName = req.FuncName[0:i]

	var r RouterMsgReq
	r.DstService = serviceName
	d, e := Marshal(req, 0)
	if e != nil {
		return e
	}
	r.RawData = d

	d1, e1 := Marshal(&r, 0)
	if e1 != nil {
		return e1
	}

	var cmd JsonProto
	cmd.CmdId = RouterMsgTypeReq
	cmd.CmdData = d1

	buf, e2 := encodeProtocol(&cmd, 0)
	if e2 != nil {
		return e2
	}
	buf[0] = 0x6

	return sess.Send(buf, peer)
}

func sendRouterRsp(current *CurrentContent, req *RouterMsgReq, rsp *RspProto, flag byte) {
	if rsp == nil { //isoneway
		return
	}
	d, _ := Marshal(rsp, 0)

	var mr RouterMsgRsp
	mr.FromAddr = req.RunAddr
	mr.FromService = req.RunService
	mr.ToSessID = req.SrcSessID
	mr.ToAddr = req.SrcAddr
	mr.RawData = d

	d1, _ := Marshal(&mr, 0)

	var cmd JsonProto
	cmd.CmdId = RouterMsgTypeRsp
	cmd.CmdData = d1

	buf, _ := encodeProtocol(&cmd, 0)
	buf[0] = flag
	current.Sess.Send(buf, current.Peer)
}

// b[0] mark msg's type: 1stBit marks req(0) or rsp（1);2ndBit marks rpc(1) or not(0);
// b[0] 3rdBit marks router(1) or not(0);
func msgLen(b []byte) uint32 {
	return uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 //| uint32(b[0])<<24
}

func encodeProtocol(msg interface{}, encode int) ([]byte, error) {
	data, e := Marshal(msg, encode)
	if e != nil {
		return nil, e
	}
	msglen := len(data) + 4
	spbMsg := Spb{}
	spbMsg.packByte(byte(msglen >> 24))
	spbMsg.packByte(byte(msglen >> 16))
	spbMsg.packByte(byte(msglen >> 8))
	spbMsg.packByte(byte(msglen))
	spbMsg.packData(data)
	flag := spbMsg.buf[0]
	if flag > 0 {
		return nil, fmt.Errorf("msg is too long: %d,max size is 16m", msglen)
	}
	return spbMsg.buf, nil
}

type RpcClientService struct {
	base       *Service
	rpc        *ServiceRpc
	mapDomains map[string][]string
	mapConns   map[string]*Connector
	mapLock    sync.RWMutex
}

func newRpcClient(addr map[string]string, svr *Service, rpc *ServiceRpc) *RpcClientService {
	r := &RpcClientService{}
	r.base = svr
	r.rpc = rpc
	r.mapDomains = make(map[string][]string)

	ips := make(map[string]string)
	for k, v := range addr {
		_, e := netip.ParseAddrPort(v)
		if e != nil { //is domain
			ss := strings.Split(v, ":")
			if len(ss) != 2 {
				panic("error ip format: " + v)
			}
			ads, er := net.LookupHost(ss[0])
			if er != nil {
				panic("domain lookup failed: " + er.Error())
			}
			if len(ads) == 0 {
				panic("domain lookup null: " + v)
			}
			r.mapDomains[k+":"+v] = ads
			ips[k] = ads[randInt()%len(ads)] + ":" + ss[1]
		} else {
			ips[k] = v
		}
	}

	r.mapLock.Lock()
	r.mapConns = make(map[string]*Connector)
	for k, v := range ips {
		r.mapConns[k] = r.base.NewConnect(v, nil)
	}
	r.mapLock.Unlock()

	if len(r.mapDomains) > 0 {
		go func() {
			time.Sleep(time.Minute * 5)
			r.resolveDomain()
		}()
	}

	return r
}

func (r *RpcClientService) resolveDomain() {
	for k, v := range r.mapDomains {
		ss := strings.Split(k, ":")
		sn := ss[0]
		do := ss[1]
		po := ss[2]

		ads, er := net.LookupHost(do)
		if er != nil || len(ads) == 0 {
			continue
		}

		var addIPs []string
		var delIPs []string
		for _, ol := range v {
			f := false
			for _, ne := range ads {
				if ol == ne {
					f = true
					break
				}
			}
			if !f {
				delIPs = append(delIPs, ol)
			}
		}
		for _, ne := range ads {
			f := false
			for _, ol := range v {
				if ol == ne {
					f = true
					break
				}
			}
			if !f {
				addIPs = append(addIPs, ne)
			}
		}
		if len(addIPs)+len(delIPs) == 0 { //no change
			continue
		}

		r.mapDomains[k] = ads
		if len(addIPs) > 0 {
			newAddr := ads[randInt()%len(ads)] + ":" + po
			r.mapLock.RLock()
			if c, ok := r.mapConns[sn]; ok {
				c.ChangeAddr(newAddr)
			}
			r.mapLock.RUnlock()
		} else if len(delIPs) > 0 {
			r.mapLock.RLock()
			if c, ok := r.mapConns[sn]; ok {
				oldAddr := c.Addr()
				for _, d := range delIPs {
					if d == oldAddr {
						newAddr := ads[randInt()%len(ads)] + ":" + po
						c.ChangeAddr(newAddr)
						break
					}
				}
			}
			r.mapLock.RUnlock()
		}
	}
}

func (r *RpcClientService) getSession(name string) *Session {
	r.mapLock.RLock()
	if c, ok := r.mapConns[name]; ok {
		r.mapLock.RUnlock()
		return c.Session()
	}
	r.mapLock.RUnlock()
	return nil
}
func (r *RpcClientService) RpcCall(service string, funcName string, params ...interface{}) error {
	sess := r.getSession(service)
	if sess == nil {
		return fmt.Errorf("%s is not found.", service)
	}
	return r.rpc.RpcCall(sess, funcName, params...)
}
func (r *RpcClientService) RpcCallSync(service string, funcName string, params ...interface{}) error {
	sess := r.getSession(service)
	if sess == nil {
		return fmt.Errorf("%s is not found.", service)
	}
	return r.rpc.RpcCall_Sync(sess, funcName, params...)
}
func (r *RpcClientService) CloseConnector(serviceName string) {
	r.mapLock.RLock()
	if c, ok := r.mapConns[serviceName]; ok {
		c.Close()
	}
	r.mapLock.RUnlock()
}
