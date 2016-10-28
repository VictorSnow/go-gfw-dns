package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"
)

type Config struct {
	Mode          string
	Listen        string
	BypassTunnels map[string]string
	InDoorServers []string
	ServerTunnels map[string]string
}

var ServerConfig Config

func main() {

	config_file := flag.String("config", "", "config file")
	flag.Parse()

	if *config_file == "" {
		*config_file = "config.json"
	}

	f, e := os.OpenFile(*config_file, os.O_CREATE, os.ModePerm)
	if e != nil {
		log.Println("打开文件错误", *f)
		return
	}

	ServerConfig := Config{}
	err := json.NewDecoder(f).Decode(&ServerConfig)
	if err != nil {
		log.Println("解析配置文件错误", err)
	}

	// 客户端模式
	if ServerConfig.Mode == "client" {
		var tmpLocal []string
		for k, v := range ServerConfig.BypassTunnels {
			tmpLocal = append(tmpLocal, k)
			go tunnelClientServe(k, v)
		}
		ListenAndServe(ServerConfig.Listen, ServerConfig.InDoorServers, tmpLocal)
	} else {
		// 服务器转发模式
		for k, v := range ServerConfig.ServerTunnels {
			go tunnelClientServe(k, v)
		}
		// loop forever
		for {
			time.Sleep(60 * time.Second)
		}
	}
}
