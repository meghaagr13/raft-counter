// main.go - Wires the node together. You shouldn't need to edit this for the
// core exercises (1-7). Read it carefully though - it shows how the pieces
// from the walkthrough map to a real Raft library.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
)

func main() {
	var (
		nodeID    = flag.String("id", "", "unique node id, e.g. node1")
		raftAddr  = flag.String("raft", "127.0.0.1:7001", "raft TCP bind addr")
		httpAddr  = flag.String("http", "127.0.0.1:8001", "http API bind addr")
		dataDir   = flag.String("data", "", "data directory (created if missing)")
		bootstrap = flag.Bool("bootstrap", false, "bootstrap a new cluster with this node as the only voter")
	)
	flag.Parse()

	if *nodeID == "" || *dataDir == "" {
		log.Fatal("--id and --data are required")
	}
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("mkdir data: %v", err)
	}

	// 1. The state machine. This is YOUR code (fsm.go).
	//    Raft will hand it committed log entries via Apply().
	fsm := NewCounterFSM()

	// 2. Raft config. Defaults are fine for a learning project, but knobs we
	//    talked about live here: ElectionTimeout, HeartbeatTimeout,
	//    SnapshotInterval, SnapshotThreshold.
	cfg := raft.DefaultConfig()
	cfg.LocalID = raft.ServerID(*nodeID)
	cfg.SnapshotInterval = 30 * time.Second
	cfg.SnapshotThreshold = 1024

	// 3. The persistent log + stable store (think: Raft's WAL + metadata).
	//    BoltDB is a simple embedded key-value file. One file per node.
	logStore, err := boltdb.NewBoltStore(filepath.Join(*dataDir, "raft-log.bolt"))
	if err != nil {
		log.Fatalf("log store: %v", err)
	}
	stableStore, err := boltdb.NewBoltStore(filepath.Join(*dataDir, "raft-stable.bolt"))
	if err != nil {
		log.Fatalf("stable store: %v", err)
	}

	// 4. Snapshot store. When the FSM snapshots, this is where it lands.
	snapshotStore, err := raft.NewFileSnapshotStore(*dataDir, 2, os.Stderr)
	if err != nil {
		log.Fatalf("snapshot store: %v", err)
	}

	// 5. Transport - the inter-node network for Raft RPCs (AppendEntries, vote,
	//    InstallSnapshot). Each node listens here for the OTHER nodes.
	tcpAddr, err := net.ResolveTCPAddr("tcp", *raftAddr)
	if err != nil {
		log.Fatalf("resolve raft addr: %v", err)
	}
	transport, err := raft.NewTCPTransport(*raftAddr, tcpAddr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		log.Fatalf("transport: %v", err)
	}

	// 6. Build the Raft node. This starts the election timer, opens connections,
	//    and starts serving the protocol.
	r, err := raft.NewRaft(cfg, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		log.Fatalf("raft: %v", err)
	}

	// 7. If --bootstrap, declare a cluster consisting of just this node. Use
	//    this on the FIRST node only, the first time you start it. Other nodes
	//    join via the /join admin endpoint after this one is leader.
	if *bootstrap {
		future := r.BootstrapCluster(raft.Configuration{
			Servers: []raft.Server{
				{ID: cfg.LocalID, Address: transport.LocalAddr()},
			},
		})
		if err := future.Error(); err != nil {
			log.Fatalf("bootstrap: %v", err)
		}
		fmt.Println("bootstrapped cluster as", *nodeID)
	}

	// 8. HTTP API. YOUR code (http.go).
	srv := NewHTTPServer(*httpAddr, r, fsm)
	log.Printf("node %s ready: raft=%s http=%s", *nodeID, *raftAddr, *httpAddr)
	log.Fatal(srv.ListenAndServe())
}
