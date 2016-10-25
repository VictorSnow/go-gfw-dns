package main

func main() {
	//buf, err := ioutil.ReadFile("gfwlist.txt")
	//if err == nil {
	//	b, _ := base64.StdEncoding.DecodeString(string(buf))
	//	ioutil.WriteFile("gfwlist.txt", b, os.ModePerm)
	//}

	//log.Println(inHost("clients.google.com"))
	//log.Println(inHost("www.cnblogs.com"))
	//parseGfw()
	//return
	go tunnelClientServe("0.0.0.0:1090", "127.0.0.1:1091")
	go tunnelServerServe("0.0.0.0:1091", "114.114.114.114:53")

	ListenAndServe(
		"0.0.0.0:53",
		[]string{"127.0.0.1:1090"},
		[]string{"127.0.0.1:1090"})
}
