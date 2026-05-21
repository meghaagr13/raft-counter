// fsm.go - The replicated state machine. YOUR JOB: fill in the TODOs.
//
// Recap from the walkthrough:
//   - Raft commits log entries in order on every replica.
//   - For each committed entry, Raft calls FSM.Apply on every replica.
//   - Apply MUST be deterministic - same inputs -> same state on every node.
//   - Periodically Raft calls FSM.Snapshot to capture state, so the log can
//     be truncated. On restart or when a new replica joins, Restore loads
//     state from a snapshot.
//
// What our FSM does: holds an int64 counter. The only command is "+1".
// (You'll add more commands in the bonus exercises.)

package main

import (
	"encoding/binary"
	"errors"
	"io"
	"sync/atomic"

	"github.com/hashicorp/raft"
)

// We use the function-style sync/atomic API (atomic.LoadInt64, AddInt64,
// StoreInt64) because the typed atomic.Int64 wrapper requires Go 1.19+.

// Command kinds we'll encode into Raft log entries.
// (A single byte at the start of each log entry. Lets you add more commands
// without breaking existing logs.)
const (
	CmdIncrement byte = 1
	// TODO bonus: CmdDecrement, CmdSet, ...
)

// CounterFSM is the state machine. It's just an atomic int64 wrapped to
// satisfy raft.FSM. The mutex/atomic discipline matters because:
//   - Apply() runs on Raft's single goroutine.
//   - Snapshot() can be called concurrently with Apply.
//   - Your HTTP GET /value reads it from another goroutine.
type CounterFSM struct {
	value int64 // accessed only via sync/atomic
}

func NewCounterFSM() *CounterFSM {
	return &CounterFSM{}
}

// Value is what GET /value calls. Local read, possibly slightly stale on
// followers (the walkthrough's "eventual consistency for reads").
func (f *CounterFSM) Value() int64 {
	return atomic.LoadInt64(&f.value)
}

// EncodeIncrement builds the bytes that the leader's HTTP handler will
// pass to raft.Apply(). Helper so http.go doesn't have to know the format.
func EncodeIncrement() []byte {
	return []byte{CmdIncrement}
}

// ---------------------------------------------------------------------------
// raft.FSM interface - the three methods you MUST implement.
// ---------------------------------------------------------------------------

// Apply is called by Raft for every committed log entry, in order, on every
// replica. The return value is delivered to the caller of raft.Apply() on the
// leader (followers ignore it).
//
// TODO 1: Parse log.Data. First byte = command kind.
// TODO 2: For CmdIncrement, atomically increment f.value and return the new value.
// TODO 3: For unknown commands, return an error (Raft will surface it to the leader).
//
// Hint: see EncodeIncrement above for the wire format.
// Hint: use atomic.AddInt64(&f.value, 1) for the atomic increment-and-return.
func (f *CounterFSM) Apply(log *raft.Log) interface{} {
	// --- BEGIN your code ---
	_ = log
	kind := log.Data[0]
	
	switch kind {
		case CmdIncrement:
			val := atomic.AddInt64(&f.value, 1)
			return val
		default:
			return errors.New("TODO: implement Apply")
		}
	// --- END your code ---
}

// Snapshot is called by Raft (on the Raft goroutine) when it wants to
// capture state for log compaction. You must return quickly with a value
// that captures the CURRENT state; the actual write to disk happens later
// inside FSMSnapshot.Persist, possibly on another goroutine.
//
// TODO: Read the current counter value (atomic load is fine, the value is
//       immutable from the snapshot's perspective once you've captured it),
//       and return a *counterSnapshot wrapping it.
//
// Why capture-then-persist? So Apply isn't blocked on disk while a snapshot
// is being written.
func (f *CounterFSM) Snapshot() (raft.FSMSnapshot, error) {
	// --- BEGIN your code ---
	return nil, errors.New("TODO: implement Snapshot")
	// --- END your code ---
}

// Restore is called by Raft on startup if there's a snapshot on disk, OR
// when this node receives an InstallSnapshot RPC from the leader because
// it's too far behind.
//
// TODO 1: Read 8 bytes from rc (the encoded int64; see counterSnapshot.Persist).
// TODO 2: Decode (binary.BigEndian.Uint64), store into f.value.
// TODO 3: Close rc.
//
// Hint: io.ReadFull(rc, buf) reads exactly 8 bytes or returns an error.
func (f *CounterFSM) Restore(rc io.ReadCloser) error {
	// --- BEGIN your code ---
	_ = rc
	return errors.New("TODO: implement Restore")
	// --- END your code ---
}

// ---------------------------------------------------------------------------
// counterSnapshot - the thing Snapshot() returns. Raft will call its Persist
// method on a background goroutine, then Release. This part IS implemented
// for you so you can focus on the FSM logic above.
// ---------------------------------------------------------------------------

type counterSnapshot struct {
	value int64
}

// Persist writes the snapshot to the provided sink. On success, you MUST call
// sink.Close(). On failure, sink.Cancel(). The 8 bytes we write here are what
// Restore will read back.
func (s *counterSnapshot) Persist(sink raft.SnapshotSink) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(s.value))
	if _, err := sink.Write(buf); err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *counterSnapshot) Release() {}
