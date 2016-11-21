package main

import (
	"net/http"
	_ "net/http/pprof"
)

func adminHandle(adminAddr string) {
	http.HandleFunc("/clear", dnsClear)
	http.ListenAndServe(adminAddr, nil)
}

func dnsClear(w http.ResponseWriter, req *http.Request) {
	Cdns.Flush()
	w.Write([]byte("Success"))
}
