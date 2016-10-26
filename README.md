# myDns

可以按照gfwlist来选择是否使用国内的dns还是国外的dns
 

# Usage


#### Client

````
ListenAndServe(
		"0.0.0.0:53",
		[]string{"114.114.114.114:53"},
		[]string{"114.114.114.114:53", "X.X.X.X:53"})
````  

2个列表用来转发dns请求， 通过gfwlist提供的主机名来辨别
- 国内的dns解析
- 加密的dns转发到国外的dns服务器解析， 目前仅提供一个国外的地址解析

提供Cache, dns故障时能使用cache

#### Server
````
	tunnelServerServe("127.0.0.1:1091", "X.X.X.X:1091")
```` 