package main

import (
	"log"
	"math/rand"
	"os"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	// addr := flag.String("addr", ":3000", "listen address")
	// nodesFlag := flag.String("nodes", "", "listen address")

	// flag.Parse()

	// var nodes []string
	// if *nodesFlag == "" {
	// 	nodes = []string{}
	// } else {
	// 	nodes = strings.Split(*nodesFlag, ",")
	// }

	node1 := NewNode(log.New(os.Stdout, "[node 1] ", log.Ltime))
	node2 := NewNode(log.New(os.Stdout, "[node 2] ", log.Ltime))
	node3 := NewNode(log.New(os.Stdout, "[node 3] ", log.Ltime))

	node1.AddRemoteNodes(node2, node3)
	node2.AddRemoteNodes(node1, node3)
	node3.AddRemoteNodes(node1, node2)

	go node1.Run()
	go node2.Run()
	node3.Run()
}
