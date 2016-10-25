package main

import (
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"
)

var Hosts map[string]int
var lock *sync.Mutex

func init() {
	lock = &sync.Mutex{}
	Hosts = make(map[string]int)
	buf, err := ioutil.ReadFile("host.txt")
	if err != nil {
		return
	}

	// 解析host
	tmpStr := string(buf)
	tmpStr = strings.Replace(tmpStr, "\r", "", -1)
	tmpHostArr := strings.Split(tmpStr, "\n")

	for _, v := range tmpHostArr {
		v = strings.TrimSpace(v)
		if v != "" {
			Hosts[v] = 1
		}
	}
}

func inHost(host string) bool {
	host = strings.Trim(host, ".")
	rhost := host

	for {
		if _, ok := Hosts[host]; ok {
			if rhost != host {
				lock.Lock()
				defer lock.Unlock()
				Hosts[rhost] = 1
				ioutil.WriteFile("host.txt", []byte("\n"+rhost), os.ModeAppend)
			}
			return true
		}

		if index := strings.Index(host, "."); index >= 0 {
			host = host[index+1:]
		} else {
			break
		}
	}
	return false
}

func parseGfw() {
	buf, err := ioutil.ReadFile("gfwlist.txt")
	if err != nil {
		return
	}

	tmpStr := string(buf)
	tmpStr = strings.Replace(tmpStr, "\r", "", -1)

	// remove white list
	s := strings.Index(tmpStr, "!################Whitelist Start################")
	tmpStr = tmpStr[:s-1]

	reg, _ := regexp.Compile("(\\w+)(\\.\\w+)+")
	strArr := reg.FindAllString(tmpStr, -1)

	tmpStr = strings.Join(strArr, "\n")
	ioutil.WriteFile("host.txt", []byte(tmpStr), os.ModePerm)
	return
}
