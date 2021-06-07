// loadconfig
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gotask/gost/stconfig"
)

type proxyWeight struct {
	address []string
	weight  []int
}

var (
	listenAddress  map[string]string
	connectAddress map[string]string
	listenConnect  = make(map[string]proxyWeight)
	listenListen   = make(map[string]proxyWeight)
	connectConnect = make(map[string]proxyWeight)
)

func getType(src, dst string) string {
	_, slok := listenAddress[src]
	_, dlok := listenAddress[dst]
	_, scok := connectAddress[src]
	_, dcok := connectAddress[dst]
	if slok && dcok {
		return "lc"
	} else if slok && dlok {
		return "ll"
	} else if scok && dcok {
		return "cc"
	}
	return ""
}

func LoadCfg() error {
	c, e := stconfig.LoadINI("ss.ini")
	if e != nil {
		return e
	}
	listenAddress = c.Section("listen")
	connectAddress = c.Section("connect")
	for k, _ := range listenAddress {
		if _, ok := connectAddress[k]; ok {
			return fmt.Errorf("listen connect keys are same: %s", k)
		}
	}

	trans := c.Section("transport")
	for _, v := range trans {
		e := fmt.Errorf("transport error format: %s", v)
		ss := strings.Split(v, "=>")
		if len(ss) != 2 {
			return e
		}
		src := ss[0]
		dst := ss[1]
		weight := 0
		if strings.Index(dst, ":") > 0 {
			tmps := strings.Split(dst, ":")
			dst = tmps[0]
			w, e := strconv.Atoi(tmps[1])
			if e != nil {
				return e
			}
			weight = w
		}
		ty := getType(src, dst)
		if ty == "lc" { //传统proxy，本地监听端口转发到远程监听端口，可以在transport里配置权重
			pw := listenConnect[listenAddress[src]]
			pw.address = append(pw.address, connectAddress[dst])
			pw.weight = append(pw.weight, weight)
			listenConnect[listenAddress[src]] = pw
		} else if ty == "ll" { //监听2个本地端口，端口1的数据会全部转发给端口2上的socket(包有重新组装，为了区分不同socket),配合cc使用穿透内网，权重无效
			pw := listenListen[listenAddress[src]]
			if len(pw.address) != 0 {
				return fmt.Errorf("listen_listen cannot use weight: %s", src)
			}
			pw.address = append(pw.address, listenAddress[dst])
			listenListen[listenAddress[src]] = pw
		} else if ty == "cc" { //连接2个远程端口,连接1的消息会转发到连接2,配合ll使用穿透内网，可以在transport里配置权重
			pw := connectConnect[connectAddress[src]]
			pw.address = append(pw.address, connectAddress[dst])
			pw.weight = append(pw.weight, weight)
			connectConnect[connectAddress[src]] = pw
		} else {
			return e
		}
	}
	return nil
}
