# raft-counter — Learn Raft by building it

A 3-node replicated counter using [hashicorp/raft](https://github.com/hashicorp/raft).
The library handles the Raft protocol; **you** write the state machine and the HTTP API.

By the end you'll have seen, with your own eyes:
- 3 processes electing a leader,
- agreeing on every increment,
- surviving a leader kill,
- a restarted node catching up.

---

## 0. The mental model

```
┌────────── one node ──────────┐
│ HTTP API ─▶ Raft ─▶ FSM      │     +  Raft TCP transport between nodes
│           on-disk log/snap   │
└──────────────────────────────┘
```

- **FSM** (`fsm.go`) — the replicated state machine. Holds the counter.
  Raft calls `Apply(entry)` on **every** replica for each committed log entry,
  in the same order. Determinism here = same state on every node.
- **HTTP** (`http.go`) — what clients talk to. `/increment` proposes a write
  via Raft on the leader; `/value` reads the local FSM.
- **main.go** — wires it together. You shouldn't need to edit it for the
  core exercises.

Library docs (skim, don't memorize): https://pkg.go.dev/github.com/hashicorp/raft

---

## 1. Build & sanity check

```bash
go build ./...   # should fail with TODOs in fsm.go and http.go — that's correct
```

Once you've implemented Exercise 2 it will compile. Exercise 3 makes it useful.

---

## Exercise 1 — Read main.go (5 min)

Open `main.go`. For each of the 8 numbered blocks, identify which concept
from our walkthrough it corresponds to. Possible answers:

```
log store / WAL    stable store    snapshot store
network transport   state machine    raft node
bootstrap (single-server initial config)    HTTP layer
```

(There's a one-to-one mapping. No code to write here.)

---

## Exercise 2 — Implement the FSM

Open `fsm.go`. Three TODOs:
- `Apply(log)` — parse the command byte, mutate the counter, return new value.
- `Snapshot()` — capture current value into a `counterSnapshot{}` and return it.
- `Restore(rc)` — read 8 bytes from `rc`, decode into `f.value`, close `rc`.

When it compiles, run:
```bash
go build ./...
```

**Conceptual question to answer before moving on:** if `Apply` returned a
random number, what would happen across replicas after several commits? Why
must it be deterministic?

---

## Exercise 3 — Implement the HTTP handlers

Open `http.go`. Implement `/increment` and `/value`.

`/increment` is the interesting one — it must:
1. Check leadership (`h.raft.State() == raft.Leader`).
2. If not leader, return 503 with the leader address.
3. If leader, call `h.raft.Apply(EncodeIncrement(), 5*time.Second)`.
4. Wait on `future.Error()`. Surface the FSM's response as JSON.

`/value` is one line of useful logic.

Build:
```bash
go build ./...
```

---

## Exercise 4 — Single node bring-up

```bash
chmod +x scripts/*.sh
rm -rf data
./scripts/run-node.sh 1 --bootstrap
```

In another terminal:
```bash
curl http://127.0.0.1:8001/status   | jq    # role should be "Leader"
curl -X POST http://127.0.0.1:8001/increment | jq
curl -X POST http://127.0.0.1:8001/increment | jq
curl http://127.0.0.1:8001/value | jq        # should be 2
```

**Look for in the logs:** the line about "entering Leader state" (it's the
quorum-of-1 self-election).

Stop with Ctrl-C.

---

## Exercise 5 — The 3-node cluster

```bash
./scripts/bootstrap.sh
```

Watch the logs scroll. After ~5s, nodes 2 and 3 should be followers of node 1.

In another terminal:
```bash
for i in 1 2 3; do curl -s http://127.0.0.1:800$i/status | jq -c; done
# one Leader, two Followers, all on the same term

curl -X POST http://127.0.0.1:8001/increment | jq
curl -X POST http://127.0.0.1:8001/increment | jq
curl -X POST http://127.0.0.1:8001/increment | jq

for i in 1 2 3; do curl -s http://127.0.0.1:800$i/value | jq -c; done
# all three should report 3

# Now try writing to a follower:
curl -i -X POST http://127.0.0.1:8002/increment
# should be 503 with leader info
```

---

## Exercise 6 — Kill the leader

With the cluster still running, find node 1's PID and kill it:
```bash
pkill -f "node1" || true   # or use jobs / ps
```

Watch logs/node2.log and logs/node3.log. Within ~1s one of them logs
"entering Candidate state" then "entering Leader state."

```bash
for i in 2 3; do curl -s http://127.0.0.1:800$i/status | jq -c; done
# one of them is now Leader, term bumped by 1+

# Writes resume against the new leader:
curl -X POST http://127.0.0.1:800<new-leader>/increment
```

**Mental check:** why did the term increase? (Hint: every election bumps it.)

---

## Exercise 7 — Restart the dead node, watch it catch up

```bash
./scripts/run-node.sh 1   # note: NO --bootstrap this time
```

It rejoins. In its log: "AppendEntries from leader" → applying entries it
missed → "joined as Follower."

```bash
curl -s http://127.0.0.1:8001/value
# matches the cluster's value
```

---

## Bonus exercises

### B1. Snapshot test
- Edit `main.go`: set `cfg.SnapshotInterval = 5 * time.Second`,
  `cfg.SnapshotThreshold = 50`.
- Rebuild and re-bootstrap. POST 200 increments.
- Kill node 3, delete `data/node3/raft-log.bolt` (keep snapshot dir!).
- Restart node 3. It should restore from snapshot, not replay every log entry.
  Look for "restoring from snapshot" in its log.

### B2. Partition test
- Use `iptables` to block traffic between node 1 and the others, OR just
  kill node 1 and node 2 simultaneously (force node 3 into the minority).
- Try POSTing to the minority side — it'll fail or block (no quorum).
- Bring nodes back; everything reconciles.

### B3. More commands
- Add `CmdDecrement`, `CmdSet`, `CmdReset` to `fsm.go`.
- Add corresponding HTTP endpoints.
- Verify that a Set-then-Increment from leader is applied in order on every
  follower.

---

## Conceptual recap (after you finish)

- **Determinism** of `Apply` is what makes "replicate the log" equivalent to
  "replicate the state."
- **Snapshot + log truncation** is the only thing that keeps a long-running
  cluster from a disk-full death.
- **Read-from-local** gives you throughput but exposes you to staleness.
  Strong-consistency reads would require routing reads through Raft too
  (see hashicorp/raft's `VerifyLeader` and lease-based reads).
- The leader's `raft.Apply` is exactly the "WAL → memtable → reply success"
  pattern, lifted to a cluster: the entry is in a quorum's logs before
  the caller hears success.

---

## File map

| File | Role | Edit? |
|---|---|---|
| `main.go` | Wires Raft node + transport + stores + HTTP | No (for core exercises) |
| `fsm.go` | The replicated state machine | **Yes (Exercise 2)** |
| `http.go` | The HTTP API | **Yes (Exercise 3)** |
| `scripts/run-node.sh` | Start one node | No |
| `scripts/bootstrap.sh` | 3-node cluster | No |
