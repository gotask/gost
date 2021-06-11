package upgrade

import (
	"os"
	"runtime"
	"time"

	"github.com/gotask/gost/stnet"
	"github.com/gotask/gost/stutil"
)

var (
	c        *stnet.Server
	rpc      *stnet.ServiceRpc
	connect  *stnet.Connect
	fileName string

	clientVersion uint32
)

type OnAppCloseAndUpdate func() error

func StartClient(remote string, ver uint32) error {
	clientVersion = ver

	fileName = stutil.FileBase("")

	c = stnet.NewServer("upgradeclient", 100, 1)
	rpc = stnet.NewServiceRpc(&UpgradeServer{})
	svc, e := c.AddRpcService("upgrade", "", 0, rpc, 0)
	if e != nil {
		return e
	}
	connect = svc.NewConnect(remote, nil)

	e = c.Start()
	if e != nil {
		return e
	}

	go func() {
		for {
			newVer := clientVersion
			rpc.RpcCall_Sync(connect.Session(), "GetLastedVersion", fileName, func(ver uint32) {
				newVer = ver
			}, func(e int32) {
				LOG.Error("rpc error %d", e)
			})

			if newVer > clientVersion { //need update
				fileBuf := getRemoteFile(fileName)
				var e error
				for fileBuf != nil {
					e = stutil.FileCreateAndWrite(fileName+".new", stutil.UnsafeBytesToString(fileBuf))
					if e != nil {
						break
					}
					binname := "update"
					if runtime.GOOS == "windows" {
						binname += ".exe"
					}
					binbuf := getRemoteFile(binname)
					if binbuf == nil {
						break
					}
					e = stutil.FileCreateAndWrite(binname, stutil.UnsafeBytesToString(binbuf))
					if e != nil {
						break
					}
					var cmd string
					for i, s := range os.Args {
						if i == 0 {
							continue
						}
						cmd += " " + s
					}
					LOG.Info("exit and update, version %d->%d.", clientVersion, newVer)
					sg <- binname + " " + fileName + cmd
					return
				}
				if e != nil {
					LOG.Error("update error: %s", e.Error())
				}
			}
			time.Sleep(oneTime * 5)
		}
	}()

	return nil
}

func getRemoteFile(fileName string) []byte {
	info := FileContent{}
	rpc.RpcCall_Sync(connect.Session(), "UpdateFile", fileName, 0, func(c FileContent) {
		info = c
	}, func(e int32) {
		LOG.Error("rpc error %d", e)
	})

	if info.Err != "" {
		LOG.Error(info.Err)
	} else if info.Total > 0 {
		fileBuf := make([]byte, info.Total)
		fileIdx := uint64(len(info.Content))
		copy(fileBuf, info.Content)
		for fileIdx < info.Total {
			content := FileContent{}
			rpc.RpcCall_Sync(connect.Session(), "UpdateFile", fileName, fileIdx, func(c FileContent) {
				content = c
			}, func(e int32) {
				LOG.Error("rpc error %d", e)
			})

			if content.Err != "" {
				LOG.Error(content.Err)
				break
			}
			if content.Name != fileName || content.Total != info.Total || content.Version != info.Version || len(content.Content) == 0 {
				LOG.Error("update error,exist new version %s-%d-%d", fileName, content.Version, info.Version)
				break
			}
			copy(fileBuf[fileIdx:], content.Content)
			fileIdx += uint64(len(content.Content))
		}
		if fileIdx == info.Total {
			return fileBuf
		}
	}
	return nil
}
