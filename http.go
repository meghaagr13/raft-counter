// http.go - HTTP API that fronts the Raft cluster. YOUR JOB: fill in the TODOs.
//
// Endpoints to implement:
//   POST /increment   - propose a +1 command to Raft (leader only)
//   GET  /value       - read the locally-applied counter (any node)
//   GET  /status      - { id, role, term, leader, last_index, last_applied }
//   POST /join        - admin: add a new node as voter (leader only)
//
// Conceptual points to internalize while coding this:
//   1. Writes go via raft.Apply() on the leader. If you're not the leader,
//      tell the client who is. (Real systems often proxy; we'll just redirect
//      so you can SEE the redirect in your terminal.)
//   2. Reads go straight to the FSM. They might be stale on followers. That
//      is the design tradeoff we discussed.
//   3. raft.Apply() returns an ApplyFuture. .Error() blocks until commit OR
//      timeout. .Response() returns whatever Apply() in fsm.go returned.

package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/hashicorp/raft"
)

type HTTPServer struct {
	addr string
	raft *raft.Raft
	fsm  *CounterFSM
	mux  *http.ServeMux
}

func NewHTTPServer(addr string, r *raft.Raft, fsm *CounterFSM) *http.Server {
	h := &HTTPServer{
		addr: addr,
		raft: r,
		fsm:  fsm,
		mux:  http.NewServeMux(),
	}
	h.mux.HandleFunc("/increment", h.handleIncrement)
	h.mux.HandleFunc("/value", h.handleValue)
	h.mux.HandleFunc("/status", h.handleStatus)
	h.mux.HandleFunc("/join", h.handleJoin)
	return &http.Server{Addr: addr, Handler: h.mux}
}

// ---------------------------------------------------------------------------
// POST /increment
//
// TODO 1: If h.raft.State() != raft.Leader, this node can't accept writes.
//         Return 503 with a JSON body { "error": "not leader", "leader": "<addr>" }.
//         h.raft.Leader() returns the leader's RAFT address (not HTTP - we don't
//         track HTTP addrs in this exercise). That's still useful for debugging.
// TODO 2: Build the command bytes (use EncodeIncrement() from fsm.go).
// TODO 3: Call h.raft.Apply(cmd, 5*time.Second). This returns a Future.
// TODO 4: future.Error() blocks until commit. If non-nil, return 500.
// TODO 5: future.Response() is the value your FSM.Apply returned. If it's an
//         error, surface it as 500. Otherwise it's the new counter value; return
//         200 with { "value": <int> }.
// ---------------------------------------------------------------------------
func (h *HTTPServer) handleIncrement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// --- BEGIN your code ---
	_ = time.Second
	http.Error(w, "TODO: implement /increment", http.StatusNotImplemented)
	// --- END your code ---
}

// ---------------------------------------------------------------------------
// GET /value
//
// TODO: Read h.fsm.Value() and return { "value": <int> } as JSON.
// Easiest endpoint - one line of useful logic. No Raft call needed; this is
// a local read of the state machine.
// ---------------------------------------------------------------------------
func (h *HTTPServer) handleValue(w http.ResponseWriter, r *http.Request) {
	// --- BEGIN your code ---
	http.Error(w, "TODO: implement /value", http.StatusNotImplemented)
	// --- END your code ---
}

// ---------------------------------------------------------------------------
// GET /status - implemented for you. Read it to understand what's introspectable.
// ---------------------------------------------------------------------------
func (h *HTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	stats := h.raft.Stats() // map[string]string of all the juicy internals
	resp := map[string]any{
		"role":         h.raft.State().String(), // Leader / Follower / Candidate / Shutdown
		"leader":       string(h.raft.Leader()),
		"term":         stats["term"],
		"last_index":   stats["last_log_index"],
		"last_applied": stats["applied_index"],
		"value":        h.fsm.Value(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---------------------------------------------------------------------------
// POST /join?id=<nodeID>&addr=<raftAddr>
// Admin endpoint: ask the LEADER to add a new node as a voter.
// Implemented for you. Use it once when bringing up nodes 2 and 3.
// ---------------------------------------------------------------------------
func (h *HTTPServer) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	addr := r.URL.Query().Get("addr")
	if id == "" || addr == "" {
		http.Error(w, "id and addr required", http.StatusBadRequest)
		return
	}
	if h.raft.State() != raft.Leader {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":  "not leader",
			"leader": string(h.raft.Leader()),
		})
		return
	}
	future := h.raft.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 10*time.Second)
	if err := future.Error(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// Use this in your /increment when not leader, to keep error handling consistent.
var errNotLeader = errors.New("not leader")
