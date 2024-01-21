## bilibili 乐享互联 

## 基于 TCP NAT3 打洞实现的 P2P 聊天室简易 Demo

B站视频:
```
https://www.bilibili.com/video/BV1mi4y1W7cb/?spm_id_from=333.999.0.0&vd_source=2d0c706e18e52ecf183100ed5009fe51
```

### 项目说明

本项目是Up主基于Golang编写的TCP打洞实现的 P2P 聊天室简易 Demo。

已经支持 NAT1、2、3 三种打洞,并未实现Stun探测NAT类型服务,只是作为一个简易Demo使用。

部署后,请将客户端发送给你的好朋友进行测试。关闭云端服务后,你仍然可以和对等节点 Peer(你的朋友) 进行聊天通信。


### 部署说明

本demo基于`Golang`开发,因此请确保有Go环境

#### 一、编译启动云端
编译
```
cd cloud
go build -o cloud main.go
```
请将编译后的`cloud`文件放置云端然后启动,启动后,云端会监听在 7709 端口
```
./cloud
```

#### 二、启动客户端

启动客户端前,请打开 `client/main.go` 文件将 cloud 常量改写为你自己的云端地址

编译客户端
```
cd client
go build -o client main.go
```
启动客户端
```
./client.exe
```
并且将客户端发送一份至你的朋友共同启动,双方将会自动进行打洞。