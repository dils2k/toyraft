package main

import (
	"flag"
	"math/rand"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	addr := flag.String("addr", ":3000", "listen address")
	nodesFlag := flag.String("nodes", "", "listen address")

	flag.Parse()

	var nodes []string
	if *nodesFlag == "" {
		nodes = []string{}
	} else {
		nodes = strings.Split(*nodesFlag, ",")
	}

	node := NewNode("http://localhost:3000", nodes)
	node.Run(*addr)
}
