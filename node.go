package main

import (
	"errors"
	"log"
	"math/rand"
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

type Node struct {
	state       State
	remoteNodes []RemoteNode

	electionTimeout time.Duration
	termID          uint64
	termMu          sync.Mutex // do we really need it?

	heartbeatChan chan struct{}

	logger *log.Logger
}

func NewNode(logger *log.Logger) *Node {
	return &Node{
		state:           FollowerState,
		electionTimeout: time.Duration(rand.Int63n(1000-500)+500) * time.Millisecond,
		heartbeatChan:   make(chan struct{}),
		logger:          logger,
	}
}

func (n *Node) requestForVotes() int {
	n.termID++

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
	n.logger.Println("got heartbeat")
	n.heartbeatChan <- struct{}{}
	return nil
}

func (n *Node) startElection() {
	n.logger.Printf("node became a candidate")
	n.state = CandidateState

	votes := n.requestForVotes()
	if votes < len(n.remoteNodes)+1/2 {
		n.logger.Printf("node became a follower")
		n.state = FollowerState
	} else {
		n.logger.Printf("node became a leader")
		n.state = LeaderState
	}
}

func (n *Node) heartbeatRemoteNodes() {
	for _, rn := range n.remoteNodes {
		go func(rn RemoteNode) {
			if err := rn.Heartbeat(); err != nil {
				n.logger.Printf("can't heartbeat node: %v", err)
			}
		}(rn)
	}
}

func (n *Node) Run() {
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
			n.logger.Println("sending heartbeat requests")
			n.heartbeatRemoteNodes()
			time.Sleep(200 * time.Millisecond) // heartbeat timeout
		}
	}
}
