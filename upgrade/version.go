package upgrade

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotask/gost/stutil"
)

var (
	verMap  map[string]int = make(map[string]int) //filename->version
	verLock sync.Mutex
	oneTime time.Duration = time.Second * 10
)

//xxxxx.v1 xxx.v3
func versionMoniter() {
	go func() {
		for {
			newVer := make(map[string]int)
			stutil.FileIterateDir(".", "", false, func(f string) bool {
				fl := stutil.FileBase(f)
				idx := strings.LastIndex(fl, ".")
				if idx > 0 && idx < len(fl)-1 {
					fname := fl[0:idx]
					fver := fl[idx+1:]
					if fver[0] == 'v' {
						ver, e := strconv.Atoi(fver[1:])
						if e == nil {
							if v, ok := newVer[fname]; ok {
								if v < ver {
									newVer[fname] = ver
								}
							} else {
								newVer[fname] = ver
							}
						}
					}
				}
				return true
			})
			verLock.Lock()
			verMap = newVer
			verLock.Unlock()

			time.Sleep(oneTime * 3)
		}
	}()
}

func getLastedFile(name string) (int, string) {
	verLock.Lock()
	defer verLock.Unlock()
	if v, ok := verMap[name]; ok {
		return v, name + ".v" + strconv.Itoa(v)
	}
	return 0, name
}
