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
	ListenAndServe(
		"0.0.0.0:53",
		[]string{"114.114.114.114:53"},
		[]string{"114.114.114.114:53", "106.187.35.20:53"})
}
