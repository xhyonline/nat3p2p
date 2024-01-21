package main

import (
	"bufio"
	"fmt"
	"github.com/gogf/gf/util/grand"
	"github.com/xhyonline/nat3p2p/server"
	"os"
	"strings"
)

func main() {
	fmt.Printf("请输入昵称:")
	inputReader := bufio.NewReader(os.Stdin)
	nickname, err := inputReader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	nickname = strings.TrimSuffix(nickname, "\n")
	port := grand.N(10000, 20000)
	address := fmt.Sprintf("0.0.0.0:%d", port)
	client := server.NewClient("39.96.14.232:7709", address, nickname)
	client.Run()
}
