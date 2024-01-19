package main

import (
	"github.com/xhyonline/nat3p2p/server"
)

func main() {
	cloud := server.NewCloud("0.0.0.0:7709")
	cloud.Run()
}
