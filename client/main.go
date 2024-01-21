package main

import (
	"bufio"
	"fmt"
	"github.com/gogf/gf/util/grand"
	"github.com/xhyonline/nat3p2p/server"
	"os"
	"strings"
)

const cloud = "此处请填写你的云端地址" // 示例:120.120.120.120:7709

func main() {
	fmt.Printf("请输入昵称:")
	inputReader := bufio.NewReader(os.Stdin)
	nickname, err := inputReader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	nickname = strings.TrimSuffix(nickname, "\n")
	port := grand.N(10000, 20000)
	localAddress := fmt.Sprintf("0.0.0.0:%d", port)
	client := server.NewClient(cloud, localAddress, nickname)
	client.Run()
}
