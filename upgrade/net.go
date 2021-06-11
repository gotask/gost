package upgrade

import (
	"io"
	"os"

	"github.com/gotask/gost/stlog"
	"github.com/gotask/gost/stnet"
)

var (
	LOG = stlog.NewFileLoggerWithoutTerm("update.log")
)

type FileVersion struct {
	Name    string
	Version uint32
}

type FileContent struct {
	Name    string
	Version uint32
	Index   uint64
	Total   uint64
	Content []byte
	Err     string
}

type UpgradeServer struct {
}

func (s *UpgradeServer) Loop() {

}

func (s *UpgradeServer) HandleError(current *stnet.CurrentContent, err error) {
	LOG.Error(err.Error())
}
func (s *UpgradeServer) HandleReq(current *stnet.CurrentContent, msg *stnet.ReqProto) {

}
func (s *UpgradeServer) HandleRsp(current *stnet.CurrentContent, msg *stnet.RspProto) {

}
func (s *UpgradeServer) HashProcessor(current *stnet.CurrentContent) (processorID int) {
	return int(current.Sess.GetID())
}

//rpc functions
func (s *UpgradeServer) GetLastedVersion(name string) uint32 {
	verLock.Lock()
	defer verLock.Unlock()
	if v, ok := verMap[name]; ok {
		return uint32(v)
	}
	return 0
}
func (s *UpgradeServer) UpdateFile(name string, index uint64) FileContent {
	var ret FileContent
	for {
		v, n := getLastedFile(name)
		f, e := os.Open(n)
		if e != nil {
			ret.Err = n + ": " + e.Error()
			break
		}
		defer f.Close()

		buf := make([]byte, 1024*32)
		i, e1 := f.ReadAt(buf, int64(index))
		if e1 != nil && e1 != io.EOF {
			ret.Err = n + ": " + e1.Error()
			break
		}
		if i <= 0 {
			ret.Err = n + " file read error"
			break
		}
		ret.Name = name
		ret.Version = uint32(v)
		ret.Index = index
		s, _ := f.Stat()
		ret.Total = uint64(s.Size())
		ret.Content = buf[0:i]
		break
	}
	return ret
}
