package main

import (
	"io/ioutil"
	"log"
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

func addHost(rhost string) {
	lock.Lock()
	defer lock.Unlock()
	Hosts[rhost] = 1

	// 缓存到文件
	f, err := os.OpenFile("host.txt", os.O_APPEND, os.ModePerm)
	if err != nil {
		log.Println("Open error", err)
		return
	}
	defer f.Close()
	f.Write([]byte("\n" + rhost))
}

func inHost(host string) bool {
	host = strings.Trim(host, ".")
	rhost := host

	for {
		if _, ok := Hosts[host]; ok {
			if rhost != host {
				addHost(host)
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

	// match host
	reg, _ := regexp.Compile("(\\w+)(\\.\\w+)+")
	strArr := reg.FindAllString(tmpStr, -1)

	tmpStr = strings.Join(strArr, "\n")
	ioutil.WriteFile("host.txt", []byte(tmpStr), os.ModePerm)
	return
}
