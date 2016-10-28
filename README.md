# myDns

可以按照gfwlist来选择是否使用国内的dns还是国外的dns
 

# Usage


#### Client

编辑config.json设置
- Mode修改为client
- Listen 为本地的监听dns请求地址
- BypassTunnels 为转发udp的地址  本地  <----> 转发服务器
- InDoorServers 国内的dns解析服务器
- BlackIpList dns返回的污染的ip地址 解析会返回相同的ip地址


2个列表用来转发dns请求， 通过gfwlist提供的主机名来辨别
- 国内的dns解析
- 加密的dns转发到国外的dns服务器解析， 目前仅提供一个国外的地址解析
- 如果国内的dns解析到的ip在BlackIpList自动增加到host.txt走国外的线路

提供Cache, dns故障时能使用本地缓存的dns记录

#### Server
编辑config.json
- Mode修改为server
- ServerTunnels用于配置转发到哪些dns服务器


# 结构

````
     client ------> 本地dns  ---(host在gfw里面)---> 远程转发服务器 ------> 国外dns
	                  |
	                  |
	                  --------(host不在gfw里面) ---> 国内的dns服务器

````

