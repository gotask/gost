package stnet

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const (
	RouterMsgTypeReg  = 1 //register
	RouterMsgTypeReq  = 2 //request
	RouterMsgTypeRsp  = 3 //response
	RouterMsgTypePing = 4 //ping
	RouterMsgTypePush = 5 //push
)

type RouterPing struct {
	Token string
}
type RouterRegister struct {
	AppID       string //one appid -> only one reg info
	Token       string
	ServiceAddr map[string]string //key: service name;val: service address(ip:port)
}
type RouterMsgReq struct {
	SrcAddr    string //req ip
	SrcSessID  uint64 //req session id
	DstService string //req dst service
	RunService string //req run service
	RunAddr    string //req run ip
	RawData    []byte
}
type RouterMsgRsp struct {
	FromAddr    string // rsp ip
	FromService string //rsp service
	ToAddr      string //req ip
	ToSessID    uint64 //req session id
	RawData     []byte
}
type RouterMsgPush struct {
	SrcAddr    string //ip
	SrcService string //service
	RawData    []byte
}

type routerSessPData struct {
	IsValid bool
	RegData RouterRegister
}
type routerConnPData struct {
	ServiceName string
	Addr        string
}
type ServiceRouter interface {
	Init() bool
	IsValidToken(appid string, token string) bool
	Loop()
}

type RouterService struct {
	base        *Service
	imp         ServiceRouter
	ignoreToken bool

	mapServiceByName map[string][]string   //name->ips [HasPrefix match]
	mapServiceByAddr map[string]*Connector //ip->sess
	mapAppSess       map[string]*Session   //appid->sess
	mapLock          sync.RWMutex
	mapAppLock       sync.RWMutex
}

func newRouterService(imp ServiceRouter) *RouterService {
	var rs RouterService
	rs.imp = imp

	return &rs
}

func (service *RouterService) Init() bool {
	service.ignoreToken = service.imp.IsValidToken("", "")

	service.mapServiceByName = make(map[string][]string)
	service.mapServiceByAddr = make(map[string]*Connector)
	service.mapAppSess = make(map[string]*Session)
	return service.imp.Init()
}
func (service *RouterService) Loop() {
	service.imp.Loop()
}
func (service *RouterService) Destroy() {

}
func (service *RouterService) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {

}
func (service *RouterService) SessionOpen(sess *Session) {
	if !sess.isConn {
		routerSessionOpenMetricAdd()
		sess.UserData = &routerSessPData{IsValid: false}
	} else {
		routerConnectOpenMetricAdd()
	}
}
func (service *RouterService) SessionClose(sess *Session) {
	if !sess.isConn {
		routerSessionCloseMetricAdd()
		if sess.UserData.(*routerSessPData).IsValid {
			service.sessionUnregister(sess)
			service.mapAppLock.Lock()
			delete(service.mapAppSess, sess.UserData.(*routerSessPData).RegData.AppID)
			service.mapAppLock.Unlock()
		}
	} else {
		routerConnectCloseMetricAdd()
	}
}
func (service *RouterService) sessionUnregister(sess *Session) {
	if sess.UserData.(*routerSessPData).IsValid { //unregister
		service.mapLock.Lock()
		reg := sess.UserData.(*routerSessPData).RegData
		for k, v := range reg.ServiceAddr {
			ads := service.mapServiceByName[k]
			if len(ads) <= 1 {
				delete(service.mapServiceByName, k)
			} else {
				for i, ad := range ads {
					if ad == v {
						ads = append(ads[0:i], ads[i+1:]...)
						break
					}
				}
				service.mapServiceByName[k] = ads
			}
			if c, ok := service.mapServiceByAddr[v]; ok {
				c.Close()
				delete(service.mapServiceByAddr, v)
			}
			sysLog.System("service unregister: %s@%s", k, v)

			routerUnregisterMetricAdd()
		}
		sess.UserData.(*routerSessPData).RegData.ServiceAddr = nil
		service.mapLock.Unlock()
	}
}
func (service *RouterService) HeartBeatTimeOut(sess *Session) {
	sess.Close()
}
func (service *RouterService) HandleError(current *CurrentContent, err error) {
	routerInvalidMsgMetricAdd()

	sysLog.Error(err.Error())
	current.Sess.Close()
}
func (service *RouterService) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
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

	if sess.isConn && flag&0x2 == 0 { //push
		routerRecvPushMetricAdd()

		req := &RouterMsgPush{}
		req.SrcService = sess.UserData.(*routerConnPData).ServiceName
		req.SrcAddr = sess.UserData.(*routerConnPData).Addr
		req.RawData = data[4:msgLen]

		buf, _ := encodeProtocol(&req, 0)
		buf[0] = 0x4
		service.base.IterateSession(func(s *Session) bool {
			routerSendPushMetricAdd()
			s.Send(buf, nil)
			return true
		})
		return int(msgLen), -1, nil, nil
	}

	cmd := JsonProto{}
	e := Unmarshal(data[4:msgLen], &cmd, 0)
	if e != nil {
		return int(msgLen), 0, nil, e
	}
	if cmd.CmdId == RouterMsgTypePing {
		return int(msgLen), -1, nil, nil
	} else if cmd.CmdId == RouterMsgTypeReg {
		var reg RouterRegister
		e := Unmarshal(cmd.CmdData, &reg, 0)
		if e != nil {
			return int(msgLen), 0, nil, e
		}
		if !service.imp.IsValidToken(reg.AppID, reg.Token) {
			return int(msgLen), 0, nil, fmt.Errorf("register token is invalid: %s", reg.Token)
		}
		if sess.UserData.(*routerSessPData).IsValid {
			sysLog.Error("server had registered: %s, sessid=%d", sess.socket.RemoteAddr().String(), sess.GetID())
			return int(msgLen), -1, nil, nil
		}

		if len(reg.ServiceAddr) > 0 {
			service.mapAppLock.Lock()
			if s, ok := service.mapAppSess[reg.AppID]; ok {
				sysLog.Error("app repeated registered: %s", reg.AppID)
				service.sessionUnregister(s)
				delete(service.mapAppSess, reg.AppID)
				s.Close()
			}
			service.mapAppLock.Unlock()
		}

		service.mapLock.Lock()
		for k, v := range reg.ServiceAddr {
			if _, ok := service.mapServiceByAddr[v]; ok {
				service.mapLock.Unlock()
				return int(msgLen), 0, nil, fmt.Errorf("service address had registered: %s@%s@%s", k, v, reg.AppID)
			}
		}
		for k, v := range reg.ServiceAddr {
			service.mapServiceByName[k] = append(service.mapServiceByName[k], v)
			service.mapServiceByAddr[v] = service.base.NewConnect(v, &routerConnPData{k, v})
			sysLog.System("service register: %s@%s", k, v)

			routerRegisterMetricAdd()
		}
		service.mapLock.Unlock()

		sess.UserData.(*routerSessPData).IsValid = true
		sess.UserData.(*routerSessPData).RegData = reg

		service.mapAppLock.Lock()
		service.mapAppSess[reg.AppID] = sess
		service.mapAppLock.Unlock()

		sysLog.System("server register: %s@%s, sessid=%d", reg.AppID, sess.RemoteAddr(), sess.GetID())

		return int(msgLen), -1, nil, nil
	} else {
		if cmd.CmdId == RouterMsgTypeReq { //from listen
			routerRequestMetricAdd()
			if sess.isConn {
				return int(msgLen), 0, nil, fmt.Errorf("req session is from connect: %s", sess.socket.RemoteAddr().String())
			}

			if !service.ignoreToken && !sess.UserData.(*routerSessPData).IsValid {
				sysLog.Error("session is not registered: %s", sess.socket.RemoteAddr().String())
				return int(msgLen), -1, nil, nil
			}
			var req RouterMsgReq
			e := Unmarshal(cmd.CmdData, &req, 0)
			if e != nil {
				return int(msgLen), 0, nil, e
			}
			req.SrcAddr = sess.socket.RemoteAddr().String()
			req.SrcSessID = sess.GetID()
			addr := make([]string, 0)
			service.mapLock.RLock()
			for k, v := range service.mapServiceByName {
				if strings.HasPrefix(k, req.DstService) {
					addr = append(addr, v...)
				}
			}
			if len(addr) == 0 {
				if _, ok := service.mapServiceByAddr[req.DstService]; ok { //service name=ip
					addr = append(addr, req.DstService)
				} else { // not found service
					service.mapLock.RUnlock()

					var rsp RouterMsgRsp
					rsp.FromAddr = req.RunAddr
					rsp.FromService = req.RunService
					var r RspProto
					r.RspCode = RotErrNoRemoteService
					if flag&0x2 == 1 { //rpc
						re := &ReqProto{}
						e := Unmarshal(req.RawData, re, 0)
						if e != nil {
							return int(msgLen), 0, nil, e
						}
						r.RspCmdSeq = re.ReqCmdSeq
						r.FuncName = re.FuncName
					}
					d, _ := Marshal(&r, 0)
					rsp.RawData = d
					buf, _ := encodeProtocol(&rsp, 0)
					buf[0] = flag | 0x5
					sess.Send(buf, sess.peer)

					sysLog.Error("service is not found: %s,src: %s", req.DstService, req.SrcAddr)
					return int(msgLen), -1, nil, nil
				}
			}
			ad := addr[randInt()%len(addr)]
			c := service.mapServiceByAddr[ad]
			service.mapLock.RUnlock()

			req.RunService = c.sess.UserData.(*routerConnPData).ServiceName
			req.RunAddr = c.sess.UserData.(*routerConnPData).Addr

			buf, _ := encodeProtocol(&req, 0)
			buf[0] = flag | 0x4
			c.Send(buf)
			return int(msgLen), -1, nil, nil
		} else if cmd.CmdId == RouterMsgTypeRsp { //from connect's response
			routerResponseMetricAdd()
			if !sess.isConn {
				return int(msgLen), 0, nil, fmt.Errorf("rsp session is not from connect: %s", sess.socket.RemoteAddr().String())
			}

			var rsp RouterMsgRsp
			e := Unmarshal(cmd.CmdData, &rsp, 0)
			if e != nil {
				return int(msgLen), 0, nil, e
			}

			rs := service.base.GetSession(rsp.ToSessID)
			if rs == nil {
				sysLog.Error("session is not found: id=%d,toaddr=%s", rsp.ToSessID, rsp.ToAddr)
				return int(msgLen), -1, nil, nil //ignore this msg
			}

			buf, _ := encodeProtocol(&rsp, 0)
			buf[0] = flag | 0x4
			rs.Send(buf, rs.peer)
			return int(msgLen), -1, nil, nil
		}
	}
	return int(msgLen), 0, nil, fmt.Errorf("message is invalid,id: %d", cmd.CmdId)
}

func (service *RouterService) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return -1
}

// RouterClient connect to router
type RouterClient struct {
	base     *Service
	rpc      *ServiceRpc
	register *RouterRegister
	regBuff  []byte

	defaultIp *Connector
	validIps  []*Connector
	ips       []string
	ipLock    sync.RWMutex

	domains   []string
	domainIPs map[string]int

	pingBuff     []byte
	lastLoopTime time.Time
}

func newRouterClient(rpc *ServiceRpc, register *RouterRegister, ips []string) *RouterClient {
	var rc RouterClient
	rc.rpc = rpc
	rc.register = register
	rc.ips = ips

	return &rc
}

func (service *RouterClient) getValidConnector() *Connector {
	service.ipLock.RLock()
	defer service.ipLock.RUnlock()
	if len(service.validIps) == 0 {
		return service.defaultIp
	}
	return service.validIps[randInt()%len(service.validIps)]
}
func (service *RouterClient) Init() bool {
	service.lastLoopTime = time.Now()

	d, _ := Marshal(service.register, 0)
	var cmd JsonProto
	cmd.CmdId = RouterMsgTypeReg
	cmd.CmdData = d
	buf, _ := encodeProtocol(&cmd, 0)
	service.regBuff = buf

	var msg RouterPing
	msg.Token = service.register.Token
	d1, _ := Marshal(&msg, 0)
	var cmd1 JsonProto
	cmd1.CmdId = RouterMsgTypePing
	cmd1.CmdData = d1
	buf1, _ := encodeProtocol(&cmd1, 0)
	service.pingBuff = buf1

	//find and resolve domain
	var defaultIP string
	ips := make(map[string]int)
	for _, a := range service.ips {
		_, e := netip.ParseAddrPort(a)
		if e != nil { //is domain
			ss := strings.Split(a, ":")
			if len(ss) != 2 {
				sysLog.Error("error ip format: %s", a)
				return false
			}
			ads, er := net.LookupHost(ss[0])
			if er != nil {
				sysLog.Error("domain %s lookup failed: %s", a, er.Error())
				return false
			}
			service.domains = append(service.domains, a)
			for _, ad := range ads {
				ips[ad+":"+ss[1]] = 1
				service.domainIPs[ad+":"+ss[1]] = 1
			}
		} else {
			defaultIP = a
			ips[a] = 1
		}
	}
	var newIPs []string
	for k, _ := range ips {
		newIPs = append(newIPs, k)
	}
	if len(newIPs) < 1 {
		sysLog.Error("router ips is null")
		return false
	}
	if defaultIP == "" {
		defaultIP = newIPs[0]
	}
	service.ips = newIPs

	service.ipLock.Lock()
	service.defaultIp = service.base.NewConnect(defaultIP, nil)
	for _, v := range service.ips {
		if v == defaultIP {
			continue
		}
		service.base.NewConnect(v, nil)
	}
	service.ipLock.Unlock()
	return service.rpc.Init()
}
func (service *RouterClient) Loop() {
	service.rpc.Loop()

	now := time.Now()
	//send ping every 2min
	//resolve domain every 2min
	if now.Sub(service.lastLoopTime) > 120*time.Second {
		service.lastLoopTime = now

		service.ipLock.RLock()
		for _, c := range service.validIps {
			service.sendPing(c.Session())
		}
		service.ipLock.RUnlock()

		if len(service.domains) > 0 {
			go service.resolveDomain()
		}
	}
}

func (service *RouterClient) resolveDomain() {
	ips := make(map[string]int)
	for _, a := range service.domains {
		ss := strings.Split(a, ":")
		ads, er := net.LookupHost(ss[0])
		if er != nil {
			sysLog.Error("domain %s lookup failed: %s", a, er.Error())
			return
		}
		for _, ad := range ads {
			ips[ad+":"+ss[1]] = 1
		}
	}
	if len(ips) == 0 {
		sysLog.Error("resolveDomain result is null")
		return
	}

	var addIPs []string
	var delIPs []string
	for k, _ := range ips {
		if _, ok := service.domainIPs[k]; !ok {
			addIPs = append(addIPs, k)
		} else {
			delete(service.domainIPs, k)
		}
	}
	for k, _ := range service.domainIPs {
		delIPs = append(delIPs, k)
	}

	service.domainIPs = ips
	for _, v := range addIPs {
		service.base.NewConnect(v, nil)
	}
	isDelDefault := false
	for _, v := range delIPs {
		if v == service.defaultIp.Addr() {
			isDelDefault = true
		}
		service.base.IterateConnect(func(c *Connector) bool {
			if v == c.Addr() {
				c.Close()
				return false
			}
			return true
		})
	}
	if isDelDefault {
		service.base.IterateConnect(func(c *Connector) bool {
			if !c.IsClose() {
				service.ipLock.Lock()
				service.defaultIp = c
				service.ipLock.Unlock()
				return false
			}
			return true
		})
	}
}

func (service *RouterClient) Destroy() {
	service.rpc.Destroy()
}
func (service *RouterClient) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {
	service.rpc.HandleMessage(current, msgID, msg)
}
func (service *RouterClient) SessionOpen(sess *Session) {
	service.rpc.SessionOpen(sess)

	service.ipLock.Lock()
	service.validIps = append(service.validIps, sess.Connector())
	service.ipLock.Unlock()

	e := service.sendMsgReg(sess)
	if e != nil {
		sysLog.Error("%s sendMsgReg error: %s", service.register.AppID, e.Error())
	}
}
func (service *RouterClient) SessionClose(sess *Session) {
	service.rpc.SessionClose(sess)

	service.ipLock.Lock()
	for i, v := range service.validIps {
		if v.Addr() == sess.Connector().Addr() {
			service.validIps = append(service.validIps[0:i], service.validIps[i+1:]...)
			break
		}
	}
	service.ipLock.Unlock()
}
func (service *RouterClient) HeartBeatTimeOut(sess *Session) {
	service.rpc.HeartBeatTimeOut(sess)
}
func (service *RouterClient) HandleError(current *CurrentContent, err error) {
	service.rpc.HandleError(current, err)
}
func (service *RouterClient) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	return service.rpc.Unmarshal(sess, data)
}
func (service *RouterClient) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return 0
}

func (service *RouterClient) sendPing(sess *Session) error {
	return sess.Send(service.pingBuff, nil)
}

func (service *RouterClient) sendMsgReg(sess *Session) error {
	return sess.Send(service.regBuff, nil)
}

func (service *RouterClient) RpcCall(serviceName string, funcName string, params ...interface{}) error {
	if serviceName == "" {
		return fmt.Errorf("RpcCall serviceName cannot be null")
	}
	c := service.getValidConnector()
	return service.rpc.rpc_call(false, true, c.Session(), nil, funcName+"@"+serviceName, params...)
}

func (service *RouterClient) RpcCallSync(serviceName string, funcName string, params ...interface{}) error {
	if serviceName == "" {
		return fmt.Errorf("RpcCall serviceName cannot be null")
	}
	c := service.getValidConnector()
	return service.rpc.rpc_call(true, true, c.Session(), nil, funcName+"@"+serviceName, params...)
}

type RouterRpcServiceEx struct {
	h func(msg *PushProto)
}

func (r *RouterRpcServiceEx) Loop() {
}
func (r *RouterRpcServiceEx) HandleError(current *CurrentContent, err error) {
	sysLog.Error("router rpc error: %s", err.Error())
}
func (r *RouterRpcServiceEx) HashProcessor(*CurrentContent) (processorID int) {
	return 0
}
func (r *RouterRpcServiceEx) HandlePush(current *CurrentContent, msg *PushProto) {
	if r.h != nil {
		r.h(msg)
	}
}
