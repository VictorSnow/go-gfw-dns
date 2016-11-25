package main

import (
	"fmt"
	"io"
	"sync"
	"time"
)

var logs []string
var locker *sync.Mutex

func init() {
	logs = make([]string, 0)
	locker = &sync.Mutex{}
}

func debug(a ...interface{}) {
	if !ServerConfig.Debug {
		return
	}

	locker.Lock()
	defer locker.Unlock()

	message := time.Now().String() + " : " + fmt.Sprint(a)
	logs = append(logs, message)

	if length := len(logs); length > 5000 {
		logs = logs[length-5000:]
	}
}

func printLogs(w io.Writer) {
	locker.Lock()
	defer locker.Unlock()

	for _, v := range logs {
		w.Write([]byte(v + "\n"))
	}
}
