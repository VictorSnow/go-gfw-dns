package main

import (
	"net/http"
)

func adminHandle(adminAddr string) {
	http.HandleFunc("/clear", dnsClear)
	http.ListenAndServe(adminAddr, nil)
}

func dnsClear(w http.ResponseWriter, req *http.Request) {
	Cdns.Flush()
	w.Write([]byte("Success"))
}
