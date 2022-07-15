package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type RemoteNode interface {
	Vote(termID uint64) error
	Heartbeat() error // will be replaced by AppendEntries in the future
}

type State int

const (
	FollowerState State = iota + 1
	CandidateState
	LeaderState
)

type entry struct {
	key    string
	value  []byte
	termID uint64
}

type Node struct {
	state       State
	remoteNodes []RemoteNode

	electionTimeout time.Duration
	termID          uint64
	termMu          sync.Mutex // do we really need it?

	heartbeatChan chan struct{}

	entries                []entry
	currState              map[string][]byte // commited entries
	lastCommitedEntryIndex int

	logger *log.Logger
}

func NewNode(logger *log.Logger) *Node {
	return &Node{
		state:           FollowerState,
		electionTimeout: time.Duration(rand.Int63n(1000-500)+500) * time.Millisecond,
		heartbeatChan:   make(chan struct{}),
		entries:         make([]entry, 0),

		logger: logger,
	}
}

func (n *Node) requestForVotes() int {

	var (
		votes = 1 // the node votes for itself too
		wg    sync.WaitGroup
	)

	for _, rn := range n.remoteNodes {
		wg.Add(1)

		go func(rn RemoteNode) {
			defer wg.Done()
			if err := rn.Vote(n.termID); err != nil {
				n.logger.Printf("can't request node to vote: %v", err)
				return
			}
			n.logger.Printf("received a vote for term %d", n.termID)
			votes++
		}(rn)
	}

	wg.Wait()

	return votes
}

func (n *Node) AddRemoteNodes(nodes ...RemoteNode) {
	for _, rn := range nodes {
		n.remoteNodes = append(n.remoteNodes, rn)
	}
}

func (n *Node) Vote(term uint64) error {
	if term <= n.termID {
		return errors.New("term id is bigger")
	}
	n.termID = term
	return nil
}

func (n *Node) Heartbeat() error {
	n.heartbeatChan <- struct{}{}
	return nil
}

func (n *Node) startElection() {
	n.logger.Printf("node became a candidate")
	n.state = CandidateState
	n.termID++

	votes := n.requestForVotes()
	if votes < len(n.remoteNodes)+1/2 {
		n.logger.Printf("node became a follower")
		n.state = FollowerState
	} else {
		n.logger.Printf("node became a leader")
		n.state = LeaderState
	}
}

func (n *Node) Run(ctx context.Context) {
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

	for {
		if n.state == LeaderState {
			for _, rn := range n.remoteNodes {
				go rn.Heartbeat()
			}

			time.Sleep(200 * time.Millisecond) // heartbeat timeout
		}
	}
}

func (n *Node) RunHTTP(ctx context.Context, addr string) error {
	go n.Run(ctx)

	http.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		_ = n.Heartbeat()
	})

	http.HandleFunc("/vote", func(w http.ResponseWriter, r *http.Request) {
		var req VoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := n.Vote(req.TermID); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
	})

	return http.ListenAndServe(addr, nil)
}

type HTTPRemoteNode struct {
	url string
}

func NewHTTPRemoteNode(url string) RemoteNode {
	return &HTTPRemoteNode{url: url}
}

type VoteRequest struct {
	TermID uint64 `json:"term_ID"`
}

func (n *HTTPRemoteNode) Vote(termID uint64) error {
	reqJSON, _ := json.Marshal(VoteRequest{termID})

	resp, err := http.Post(n.url+"/vote", "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unsuccessful code %d", resp.StatusCode)
	}

	return nil
}

func (n *HTTPRemoteNode) Heartbeat() error {
	resp, err := http.Post(n.url+"/heartbeat", "application/json", nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unsuccessful code %d", resp.StatusCode)
	}

	return nil
}
