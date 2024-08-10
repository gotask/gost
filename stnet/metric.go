package stnet

import "sync/atomic"

func metricSet(ov *int64, nv int64) {
	atomic.StoreInt64(ov, nv)
}
func metricAdd(ov *int64, v int64) {
	atomic.AddInt64(ov, v)
}

type RpcCollector struct {
	requestCnt    int64
	responseCnt   int64
	successCnt    int64
	failedCnt     int64
	overtimeCnt   int64
	remoteReqCnt  int64
	invalidReqCnt int64
	recvPushCnt   int64
	sendPushCnt   int64
}

func rpcRequestMetricAdd() {
	metricAdd(&gRpcMetricInfo.requestCnt, 1)
}
func rpcResponseMetricAdd() {
	metricAdd(&gRpcMetricInfo.responseCnt, 1)
}
func rpcSuccessMetricAdd() {
	metricAdd(&gRpcMetricInfo.successCnt, 1)
}
func rpcFailedMetricAdd() {
	metricAdd(&gRpcMetricInfo.failedCnt, 1)
}
func rpcOverTimeMetricAdd() {
	metricAdd(&gRpcMetricInfo.overtimeCnt, 1)
}
func rpcRemoteReqMetricAdd() {
	metricAdd(&gRpcMetricInfo.remoteReqCnt, 1)
}
func rpcInvalidReqMetricAdd() {
	metricAdd(&gRpcMetricInfo.invalidReqCnt, 1)
}
func rpcRecvPushMetricAdd() {
	metricAdd(&gRpcMetricInfo.recvPushCnt, 1)
}
func rpcSendPushMetricAdd() {
	metricAdd(&gRpcMetricInfo.sendPushCnt, 1)
}

type SessionCollector struct {
	openCnt        int64
	closeCnt       int64
	reconnectCnt   int64
	beClosedCnt    int64
	sendMsgCnt     int64
	sendSuccessCnt int64
	recvPackCnt    int64
	recvMsgCnt     int64
}

func sessionOpenMetricAdd() {
	metricAdd(&gSessionMetricInfo.openCnt, 1)
}
func sessionCloseMetricAdd() {
	metricAdd(&gSessionMetricInfo.closeCnt, 1)
}
func sessionReconnectMetricAdd() {
	metricAdd(&gSessionMetricInfo.reconnectCnt, 1)
}
func sessionBeClosedMetricAdd() {
	metricAdd(&gSessionMetricInfo.beClosedCnt, 1)
}
func sessionSendMsgMetricAdd() {
	metricAdd(&gSessionMetricInfo.sendMsgCnt, 1)
}
func sessionSendSuccessMetricAdd() {
	metricAdd(&gSessionMetricInfo.sendSuccessCnt, 1)
}
func sessionRecvPackMetricAdd() {
	metricAdd(&gSessionMetricInfo.recvPackCnt, 1)
}
func sessionRecvMsgMetricAdd() {
	metricAdd(&gSessionMetricInfo.recvMsgCnt, 1)
}

type RouterCollector struct {
	sessionOpenCnt  int64
	sessionCloseCnt int64
	connectOpenCnt  int64
	connectCloseCnt int64
	registerCnt     int64
	unregisterCnt   int64
	requestCnt      int64
	responseCnt     int64
	invalidMsgCnt   int64
	recvPushCnt     int64
	sendPushCnt     int64
}

func routerSessionOpenMetricAdd() {
	metricAdd(&gRouterMetricInfo.sessionOpenCnt, 1)
}
func routerSessionCloseMetricAdd() {
	metricAdd(&gRouterMetricInfo.sessionCloseCnt, 1)
}
func routerConnectOpenMetricAdd() {
	metricAdd(&gRouterMetricInfo.connectOpenCnt, 1)
}
func routerConnectCloseMetricAdd() {
	metricAdd(&gRouterMetricInfo.connectCloseCnt, 1)
}
func routerRegisterMetricAdd() {
	metricAdd(&gRouterMetricInfo.registerCnt, 1)
}
func routerUnregisterMetricAdd() {
	metricAdd(&gRouterMetricInfo.unregisterCnt, 1)
}
func routerRequestMetricAdd() {
	metricAdd(&gRouterMetricInfo.requestCnt, 1)
}
func routerResponseMetricAdd() {
	metricAdd(&gRouterMetricInfo.responseCnt, 1)
}
func routerInvalidMsgMetricAdd() {
	metricAdd(&gRouterMetricInfo.invalidMsgCnt, 1)
}
func routerRecvPushMetricAdd() {
	metricAdd(&gRouterMetricInfo.recvPushCnt, 1)
}
func routerSendPushMetricAdd() {
	metricAdd(&gRouterMetricInfo.sendPushCnt, 1)
}

var (
	gRpcMetricInfo     RpcCollector
	gSessionMetricInfo SessionCollector
	gRouterMetricInfo  RouterCollector
)

func resetMetricInfo() {
	metricSet(&gRpcMetricInfo.requestCnt, 0)
	metricSet(&gRpcMetricInfo.responseCnt, 0)
	metricSet(&gRpcMetricInfo.successCnt, 0)
	metricSet(&gRpcMetricInfo.failedCnt, 0)
	metricSet(&gRpcMetricInfo.overtimeCnt, 0)
	metricSet(&gRpcMetricInfo.remoteReqCnt, 0)
	metricSet(&gRpcMetricInfo.invalidReqCnt, 0)
	metricSet(&gRpcMetricInfo.recvPushCnt, 0)
	metricSet(&gRpcMetricInfo.sendPushCnt, 0)

	metricSet(&gSessionMetricInfo.openCnt, 0)
	metricSet(&gSessionMetricInfo.closeCnt, 0)
	metricSet(&gSessionMetricInfo.reconnectCnt, 0)
	metricSet(&gSessionMetricInfo.beClosedCnt, 0)
	metricSet(&gSessionMetricInfo.sendMsgCnt, 0)
	metricSet(&gSessionMetricInfo.sendSuccessCnt, 0)
	metricSet(&gSessionMetricInfo.recvPackCnt, 0)
	metricSet(&gSessionMetricInfo.recvMsgCnt, 0)

	metricSet(&gRouterMetricInfo.sessionOpenCnt, 0)
	metricSet(&gRouterMetricInfo.sessionCloseCnt, 0)
	metricSet(&gRouterMetricInfo.connectOpenCnt, 0)
	metricSet(&gRouterMetricInfo.connectCloseCnt, 0)
	metricSet(&gRouterMetricInfo.registerCnt, 0)
	metricSet(&gRouterMetricInfo.unregisterCnt, 0)
	metricSet(&gRouterMetricInfo.requestCnt, 0)
	metricSet(&gRouterMetricInfo.responseCnt, 0)
	metricSet(&gRouterMetricInfo.invalidMsgCnt, 0)
	metricSet(&gRouterMetricInfo.recvPushCnt, 0)
	metricSet(&gRouterMetricInfo.sendPushCnt, 0)
}

func logMetric() {
	sysLog.Info("rpc metric: %+v", gRpcMetricInfo)
	sysLog.Info("session metric: %+v", gSessionMetricInfo)
	sysLog.Info("router metric: %+v", gRouterMetricInfo)
}
