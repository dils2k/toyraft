package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net/http"
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

	go node.Run(context.Background())

	http.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		_ = node.Heartbeat()
	})

	http.HandleFunc("/vote", func(w http.ResponseWriter, r *http.Request) {
		var req VoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := node.Vote(req.TermID); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
	})

	http.ListenAndServe(*addr, nil)
}
