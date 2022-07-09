package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type State int

const (
	FollowerState State = iota + 1
	CandidateState
	LeaderState
)

type Node struct {
	state     State
	nodesURLs []string
	extAddr   string

	electionTimeout time.Duration
	termID          uint64
	termMu          sync.Mutex

	heartbeatChan chan struct{}
}

func NewNode(extAddr string, nodesURLs []string) *Node {
	return &Node{
		extAddr:   extAddr,
		nodesURLs: nodesURLs,
		state:     FollowerState,

		electionTimeout: time.Duration(rand.Int63n(1000-500)+500) * time.Millisecond,

		heartbeatChan: make(chan struct{}),
	}
}

type VoteRequest struct {
	Term uint64 `json:"term"`
}

func (n *Node) requestForVotes() int {
	n.termID++

	var wg sync.WaitGroup
	wg.Add(len(n.nodesURLs))

	voted := 1
	for _, nodeURL := range n.nodesURLs {
		msg, _ := json.Marshal(VoteRequest{n.termID})

		go func(nodeURL string, voted int) {
			defer wg.Done()

			rsp, err := http.Post(nodeURL+"/requestVote", "application/json", bytes.NewBuffer(msg))
			if err != nil {
				log.Printf("can't request node %s for a vote: %v", nodeURL, err)
				return
			}

			if rsp.StatusCode == http.StatusOK {
				voted++
			}
		}(nodeURL, voted)
	}

	wg.Wait()

	return voted
}

func (n *Node) vote(term uint64, nodeURL string) error {
	n.termMu.Lock()
	defer n.termMu.Unlock()

	if term < n.termID {
		return errors.New("term id is bigger")
	}

	n.termID = term
	return nil
}

func (n *Node) startElection() {
	log.Printf("node became a candidate")
	n.state = CandidateState
	voted := n.requestForVotes()
	if voted < len(n.nodesURLs)+1/2 {
		n.state = FollowerState
	} else {
		log.Printf("node became the leader")
		n.state = LeaderState
	}
}

func (n *Node) heartbeat() {
	for _, nodeURL := range n.nodesURLs {
		go func(nodeURL string) {
			_, err := http.Post(nodeURL+"/appendEntries", "application/json", nil)
			if err != nil {
				log.Printf("can't heartbeat node %s: %v", nodeURL, err)
			}
		}(nodeURL)
	}
}

func (n *Node) Run(addr string) {
	go func() {
		for {
			if n.state != FollowerState {
				continue
			}

			select {
			case <-n.heartbeatChan:
				continue

			case <-time.After(n.electionTimeout):
				n.startElection()
			}
		}
	}()

	go func() {
		// if master then heartbeat all nodes
		// what info should it send to other nodes?
		//   * other nodes urls?
		//   * current term?
		for {
			if n.state != LeaderState {
				continue
			}

			n.heartbeat()

			time.Sleep(200 * time.Millisecond)
		}
	}()

	http.HandleFunc("/appendEntries", func(w http.ResponseWriter, r *http.Request) {
		log.Println("heartbeat")
		n.heartbeatChan <- struct{}{}
	})

	http.HandleFunc("/requestVote", func(w http.ResponseWriter, r *http.Request) {
		type rsp struct {
			Msg string `json:"msg"`
		}

		var vote VoteRequest
		if err := json.NewDecoder(r.Body).Decode(&vote); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	})

	http.ListenAndServe(addr, nil)
}
