package main

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Partner search with Paxos: one instance per target and epoch (see instance).
// The waiter proposes the pair (waiter, helper) in two phases, PREPARE/PROMISE and
// ACCEPT/ACCEPTED. Paxos decides who; GO/ACK/READY then make the pickup
// synchronous: the helper commits with ACK, the waiter answers READY, and only
// then does the helper step into the cell. A pair not finished by the end of its
// epoch is dropped, so a lost pair never blocks a target for ever.
// A robot sends one packet per turn; `to` makes others ignore what is not for them.
//
// Timeouts adapt to the network: every robot keeps a running average of how late
// the packets it receives are (meanDelay) and stretches resendEvery, giveUpAfter,
// helpAfter and reserveFor when that average is above baseDelay. The epoch is
// different: a waiter and its helper must end it on the same tick, so its length
// is part of the instance number (level k = epochLen<<k) and the helper reads the
// end of the epoch from the GO it received, never from its own estimate.

const (
	baseDelay   = 3.0 // mean delay (iterations) the timeouts below are sized for
	maxLevel    = 6   // longest epoch is epochLen<<maxLevel
	delayAlpha  = 32  // meanDelay moves 1/delayAlpha of the way to each new observation
	resendEvery = 15  // ticks before a waiter without a decision starts a new round
	giveUpAfter = 45  // ticks before a waiter walks away from an undecided or unanswered pair
	helpAfter   = 8   // ticks before a waiter offers to help a lower ID (else all-waiting deadlock)
	epochLen    = 100 // ticks a level-0 Paxos instance of a target lasts
	reserveFor  = 30  // ticks a helper stays free after accepting a value naming it
	instScale   = 100000
	levelScale  = 10000 // instance = target*instScale + level*levelScale + epoch number
	msgPrepare  = "PREPARE"
	msgPromise  = "PROMISE"
	msgAccept   = "ACCEPT"
	msgAccepted = "ACCEPTED"
	msgGo       = "GO"
	msgAck      = "ACK"
	msgReady    = "READY"
	msgCancel   = "CANCEL"
)

// instance is target t's level-0 Paxos instance at iteration turn.
func instance(t, turn int) int { return instanceAt(t, turn, 0) }

// instanceAt is target t's instance at iteration turn with epochs of epochLen<<level ticks.
func instanceAt(t, turn, level int) int {
	return t*instScale + level*levelScale + turn/(epochLen<<level)
}

// epochEnd is the first iteration after instance inst's epoch, read from the number itself.
func epochEnd(inst int) int {
	level, epoch := inst%instScale/levelScale, inst%levelScale
	return (epoch + 1) * (epochLen << level)
}

// ticks stretches a timeout for a slow network: unchanged up to baseDelay, then in proportion.
func (r *Robot) ticks(base int) int {
	if r.meanDelay <= baseDelay {
		return base
	}
	return int(math.Ceil(float64(base) * (r.meanDelay + 1) / (baseDelay + 1)))
}

// level is the epoch level this robot proposes: epochs about as much longer than
// epochLen as the network is slower than baseDelay (rounded to a power of two).
func (r *Robot) level() int {
	if r.meanDelay <= baseDelay {
		return 0
	}
	k := int(math.Round(math.Log2((r.meanDelay + 1) / (baseDelay + 1))))
	return min(max(k, 0), maxLevel)
}

// observe folds one packet's delay (iterations beyond the minimum) into meanDelay.
func (r *Robot) observe(delay int) {
	if r.meanDelay == 0 {
		r.meanDelay = baseDelay // start from the delay the constants are sized for
	}
	r.meanDelay += (float64(delay) - r.meanDelay) / delayAlpha
}

// decidedNow reports whether target t's instance is decided at any level this epoch.
func (r *Robot) decidedNow(t, turn int) bool {
	for k := 0; k <= maxLevel; k++ {
		if r.isDecided(instanceAt(t, turn, k)) {
			return true
		}
	}
	return false
}

type Message struct {
	kind     string
	from, to int  // to is 0 for everyone
	target   int  // Paxos instance, see instance
	n        int  // proposal number
	accN     int  // PROMISE: n of the value already accepted (0 = none)
	val      pair // the pair, in PROMISE (old value), ACCEPT, ACCEPTED, GO
	row, col int  // where the waiter stands
	dist     int  // PROMISE: how far the sender is from (row, col)
	free     bool // PROMISE: the sender can be a helper
}

// delivery is a packet read after wait more iterations.
type delivery struct {
	msgs []Message
	wait int
	sent int // the iteration it was sent in
}

type pair struct{ a, b int } // a = waiter, b = helper

// acceptor is a robot's memory of one instance.
type acceptor struct {
	promised, accN int  // highest n promised; n of the value accepted (0 = none)
	val            pair // the value accepted at accN
}

// vote is one proposal, for counting ACCEPTED.
type vote struct{ target, n int }

func (r *Robot) send(msg Message) { r.outbox = append(r.outbox, msg) }

// emit queues msg for the others and also handles it here if it is meant for us
// (SendMessage skips the sender, and a proposer must hear itself).
func (r *Robot) emit(m *Map, msg Message) {
	if msg.to != r.id {
		r.send(msg)
	}
	if msg.to == 0 || msg.to == r.id {
		r.handle(m, msg)
	}
}

// String is how a message appears in the log, e.g. "PROMISE 101, dist: 4 to r1".
func (msg Message) String() string {
	s := msg.kind
	switch msg.kind {
	case msgPrepare:
		s += fmt.Sprintf(" %d", msg.n)
	case msgPromise:
		s += fmt.Sprintf(" %d, dist: %d", msg.n, msg.dist)
	case msgAccept:
		s += fmt.Sprintf(" n=%d r%d+r%d", msg.n, msg.val.a, msg.val.b)
	case msgAccepted:
		s += fmt.Sprintf(" n=%d r%d+r%d", msg.n, msg.val.a, msg.val.b)
	}

	if msg.to != 0 {
		s += fmt.Sprintf(" to r%d", msg.to)
	}
	return s
}

// SendMessage broadcasts the outbox as one packet, each receiver with its own
// delay, and returns the log line ("" if there was nothing to send).
func (r *Robot) SendMessage(m *Map, robots []*Robot) string {
	if len(r.outbox) == 0 {
		return ""
	}
	parts := make([]string, len(r.outbox))
	for i, msg := range r.outbox {
		parts[i] = msg.String()
	}
	for _, o := range robots {
		if o != r {
			o.inbox = append(o.inbox, delivery{r.outbox, m.messageDelay(), m.turn})
		}
	}
	r.outbox = nil
	return fmt.Sprintf("r%d broadcasts: %s", r.id, strings.Join(parts, "; "))
}

// ReceiveMessage handles the packets that are due and counts the rest down.
func (r *Robot) ReceiveMessage(m *Map) {
	pending := r.inbox
	r.inbox = nil
	for _, d := range pending {
		if d.wait == 0 {
			r.observe(m.turn - d.sent - 1)
			for _, msg := range d.msgs {
				r.handle(m, msg)
			}
		} else {
			d.wait--
			r.inbox = append(r.inbox, d)
		}
	}
}

func (r *Robot) handle(m *Map, msg Message) {
	if msg.to != 0 && msg.to != r.id {
		return
	}
	switch msg.kind {
	case msgPrepare:
		acc := r.accOf(msg.target)
		if msg.n > acc.promised {
			acc.promised = msg.n
			r.emit(m, Message{kind: msgPromise, from: r.id, to: msg.from, target: msg.target,
				free: (!r.onTarget && r.partner == 0 && !r.reserved(m.turn)) || r.willingToHelp(m.turn, msg.from, msg.row, msg.col),
				n:    msg.n, accN: acc.accN, val: acc.val, dist: abs(r.row-msg.row) + abs(r.col-msg.col)})
		}
	case msgPromise:
		if !r.onTarget || r.jobTarget != msg.target || msg.n != r.propN || r.accSent {
			return // not my round, or a late promise
		}
		if r.promises == nil {
			r.promises = map[int]Message{}
		}
		r.promises[msg.from] = msg
		if len(r.promises) < m.quorum() {
			return
		}
		val, ok := r.chooseValue()
		if !ok {
			return // no free promiser yet; wait for more
		}
		r.accSent = true
		r.emit(m, Message{kind: msgAccept, from: r.id, target: msg.target, n: r.propN,
			val: val, row: r.row, col: r.col})
	case msgAccept:
		acc := r.accOf(msg.target)
		busyHelper := msg.val.b == r.id && (r.partner != 0 ||
			(r.onTarget && !r.willingToHelp(m.turn, msg.val.a, msg.row, msg.col)) ||
			(r.reserved(m.turn) && r.reservedFor != msg.target))
		if msg.n < acc.promised || busyHelper {
			return
		}
		acc.promised, acc.accN, acc.val = msg.n, msg.n, msg.val
		if msg.val.b == r.id {
			r.reservedFor, r.reservedUntil = msg.target, m.turn+r.ticks(reserveFor)
		}
		r.emit(m, Message{kind: msgAccepted, from: r.id, target: msg.target, n: msg.n,
			val: msg.val, row: msg.row, col: msg.col})

	case msgGo:
		// Commit to one waiter; the ACK follows from decideHelping once outside its cell.
		if msg.val.b != r.id || r.partner != 0 || m.turn >= epochEnd(msg.target) {
			return
		}
		if r.onTarget {
			if !r.willingToHelp(m.turn, msg.val.a, msg.row, msg.col) {
				return
			}
			r.resetJob()
		}
		r.partner, r.jobTarget, r.targetID = msg.val.a, msg.target, msg.target/instScale
		r.goalRow, r.goalCol = msg.row, msg.col
		r.reservedUntil = 0

	case msgAck:
		// The helper committed: I may pick up when I sense it; tell it to come.
		if !r.onTarget || r.jobTarget != msg.target || msg.from != r.partner || r.acked {
			return
		}
		r.acked = true
		r.emit(m, Message{kind: msgReady, from: r.id, to: msg.from, target: msg.target, val: msg.val})

	case msgCancel:
		// My waiter gave up before I ACKed, so it never sent READY: free again.
		if !r.onTarget && !r.ready && msg.from == r.partner && msg.target == r.jobTarget {
			r.resetJob()
		}

	case msgReady:
		if !r.onTarget && msg.from == r.partner && msg.target == r.jobTarget {
			r.ready = true
		}

	case msgAccepted:
		if _, done := r.decided[msg.target]; done {
			return
		}
		v := vote{msg.target, msg.n}
		if r.tally == nil {
			r.tally = map[vote]map[int]bool{}
		}
		if r.tally[v] == nil {
			r.tally[v] = map[int]bool{}
		}
		r.tally[v][msg.from] = true
		if len(r.tally[v]) < m.quorum() {
			return
		}
		if r.decided == nil {
			r.decided = map[int]pair{}
		}
		r.decided[msg.target] = msg.val
		r.learn(m, msg)
	}
}

// learn acts on a decided pair: the waiter takes its partner and sends GO, a
// waiter left out leaves, everyone else only remembers (Decide avoids the target).
func (r *Robot) learn(m *Map, msg Message) {
	if !r.onTarget || r.jobTarget != msg.target {
		return
	}
	switch r.id {
	case msg.val.a:
		r.partner, r.acked = msg.val.b, false
		r.emit(m, Message{kind: msgGo, from: r.id, to: msg.val.b, target: msg.target,
			val: msg.val, row: r.row, col: r.col})
	case msg.val.b:
		// Named as helper while waiting here: stay; the epoch ends this attempt.
	default:
		r.resetJob()
	}
}

// willingToHelp: a waiter with no partner offers to help a lower-ID waiter at
// (row, col) at once if it shares that cell, else after helpAfter ticks. Without
// this, with targets everywhere every robot waits on its own target for ever.
func (r *Robot) willingToHelp(turn, waiterID, row, col int) bool {
	if !r.onTarget || r.partner != 0 || waiterID >= r.id {
		return false
	}
	return (r.row == row && r.col == col) || (r.waitingSince != 0 && turn-r.waitingSince >= r.ticks(helpAfter))
}

// reserved reports whether the robot is holding itself free for a value it accepted.
func (r *Robot) reserved(turn int) bool { return turn < r.reservedUntil }

// accOf returns the acceptor state of instance t, creating it on first use.
func (r *Robot) accOf(t int) *acceptor {
	if r.acc == nil {
		r.acc = map[int]*acceptor{}
	}
	if r.acc[t] == nil {
		r.acc[t] = &acceptor{}
	}
	return r.acc[t]
}

// chooseValue picks the pair to propose: the accepted value with the highest accN
// if any promiser has one (Paxos safety), else me and the closest free promiser
// (lowest ID on a tie). ok is false while there is no partner to offer.
func (r *Robot) chooseValue() (val pair, ok bool) {
	var best Message
	for _, p := range r.promises {
		if p.accN > best.accN {
			best = p
		}
	}
	if best.accN > 0 {
		return best.val, true
	}

	partner := 0
	for _, p := range r.promises {
		if p.from == r.id || !p.free {
			continue
		}
		if partner == 0 || p.dist < r.promises[partner].dist ||
			(p.dist == r.promises[partner].dist && p.from < partner) {
			partner = p.from
		}
	}
	return pair{a: r.id, b: partner}, partner != 0
}

// startWaiting is called when the robot steps onto a target.
func (r *Robot) startWaiting(s Sensing) {
	r.resetJob()
	r.targetID = s.targetID
	r.jobTarget = instanceAt(s.targetID, s.turn, r.level())
	r.onTarget = true
}

// resetJob forgets the job and the round in progress, not what it promised, accepted or learned.
func (r *Robot) resetJob() {
	r.onTarget = false
	r.jobTarget, r.targetID, r.partner, r.waitingSince, r.goalRow, r.goalCol = 0, 0, 0, 0, 0, 0
	r.acked, r.ready, r.ackSent, r.sweepStage = false, false, false, 0
	r.propN = 0
	r.promises = nil
	r.accSent = false
}

// decideWaiting runs the Paxos rounds, gives up when nothing works, and picks up with its partner.
func (r *Robot) decideWaiting(m *Map, s Sensing) string {
	if s.turn >= epochEnd(r.jobTarget) {
		// New epoch: drop the old attempt and start a fresh instance.
		r.jobTarget, r.partner, r.acked = instanceAt(r.targetID, s.turn, r.level()), 0, false
		r.propN, r.promises, r.accSent = 0, nil, false
		r.waitingSince = s.turn
	}

	if r.partner != 0 && !r.acked && s.turn-r.propAt >= r.ticks(giveUpAfter) {
		// The helper never committed. Tell it, so it does not wait at the door for
		// the rest of the epoch. Safe: a waiter that has not acked can never pick up.
		r.emit(m, Message{kind: msgCancel, from: r.id, to: r.partner, target: r.jobTarget})
		r.resetJob()
		return actionStay
	}

	if r.partner == 0 {
		if r.waitingSince == 0 {
			r.waitingSince = s.turn
		}
		if s.turn-r.waitingSince >= r.ticks(giveUpAfter) {
			r.resetJob()
			return actionStay
		}
		// Only the lowest ID in a cell proposes: it can recruit the others, and a higher
		// ID's proposal numbers would always outrank its rounds.
		lowerHere := slices.ContainsFunc(s.robotsHere, func(id int) bool { return id < r.id })
		if !lowerHere && (r.propN == 0 || s.turn-r.propAt >= r.ticks(resendEvery)) {
			r.counter++
			r.propN = r.counter*100 + r.id
			r.propAt = s.turn
			r.promises = map[int]Message{}
			r.accSent = false
			r.emit(m, Message{kind: msgPrepare, from: r.id, target: r.jobTarget, n: r.propN,
				row: r.row, col: r.col})
		}
	}

	// Pick up once the helper committed (ACK) and is alone with me.
	if r.acked && slices.Equal(s.robotsHere, []int{r.partner}) {
		return actionPickup
	}
	return actionStay
}

// decideHelping walks to the waiter's cell but stops at its edge until READY, ACKs
// once outside the cell (a helper that starts inside steps out first), then picks up.
// A helper never lets go on its own timer: a waiter that already sent READY would
// then take any robot wandering into its cell for its partner. Only the waiter
// frees it (CANCEL) or the epoch ends.
func (r *Robot) decideHelping(m *Map, s Sensing) string {
	if s.turn >= epochEnd(r.jobTarget) {
		r.resetJob() // epoch over
		return actionStay
	}
	atGoal := r.row == r.goalRow && r.col == r.goalCol
	if !r.ready && atGoal {
		if s.wallAhead {
			return actionLeft
		}
		return actionMove
	}
	if !r.ackSent && !atGoal {
		r.ackSent = true
		r.emit(m, Message{kind: msgAck, from: r.id, to: r.partner, target: r.jobTarget, val: pair{r.partner, r.id}})
	}
	if atGoal {
		if slices.Equal(s.robotsHere, []int{r.partner}) {
			return actionPickup
		}
		return actionStay
	}
	if row, col := r.PreMove(); !r.ready && row == r.goalRow && col == r.goalCol {
		return actionStay
	}
	return r.headToward(r.goalRow, r.goalCol)
}

// headToward turns or steps towards a cell, columns first.
func (r *Robot) headToward(row, col int) string {
	want := "N"
	switch {
	case col > r.col:
		want = "E"
	case col < r.col:
		want = "W"
	case row > r.row:
		want = "S"
	}
	switch {
	case r.facing == want:
		return actionMove
	case leftOf[r.facing] == want:
		return actionLeft
	default:
		return actionRight
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (r *Robot) isDecided(t int) bool { _, ok := r.decided[t]; return ok }
