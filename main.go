package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	addr := flag.String("addr", ":3000", "listen address")
	nodesFlag := flag.String("nodes", "", "external nodes")

	flag.Parse()

	node := NewNode(log.New(os.Stdout, "", log.Ltime))
	nodesURLs := strings.Split(*nodesFlag, ",")
	if *nodesFlag != "" {
		for _, nurl := range nodesURLs {
			node.AddRemoteNodes(NewHTTPRemoteNode(nurl))
		}
	}

	log.Fatal(node.RunHTTP(context.Background(), *addr))
}
