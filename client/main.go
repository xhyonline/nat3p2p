package main

import (
	"fmt"
	"github.com/gogf/gf/util/grand"
	"github.com/xhyonline/nat3p2p/server"
)

func main() {
	nickname := server.GetFullName() // 生成昵称
	port := grand.N(10000, 20000)
	address := fmt.Sprintf("0.0.0.0:%d", port)
	client := server.NewClient("39.96.14.232:7709", address, nickname)
	client.Run()
}
