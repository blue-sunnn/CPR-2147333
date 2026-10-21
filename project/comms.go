package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Finding a partner: the first robot on a target (the waiter) broadcasts HELP,
// free robots (helpers) reply CLAIM with their distance, and the waiter ACCEPTs
// the closest and RELEASEs the rest. A helper holds one claim at a time, so it
// is never promised to two waiters. Sensing is instant, so the pickup needs no
// message: both robots pick up when they sense exactly each other in the cell.
// A robot broadcasts one packet per turn holding all its messages; `to` makes
// the other robots ignore the ones not meant for them.

const (
	msgHelp    = "HELP"
	msgClaim   = "CLAIM"
	msgAccept  = "ACCEPT"
	msgRelease = "RELEASE"
)

const (
	acceptWindow = 0  // ticks to keep collecting claims after the first; raise it when messages are delayed
	resendEvery  = 10 // ticks before a waiter without a partner calls again
	giveUpAfter  = 10 // ticks before an unanswered waiter helps a lower ID (else all-waiting deadlock)
)

type Message struct {
	kind     string
	from, to int // to is 0 for everyone
	seq      int // which request of the waiter
	row, col int // HELP: where the waiter is
	dist     int // CLAIM: how far the helper is
}

// delivery is a queued packet, read after wait more iterations.
type delivery struct {
	msgs []Message
	wait int
}

// offer is the request a helper has claimed or been accepted for (owner 0 = none).
type offer struct {
	owner, seq, row, col int
	accepted             bool
}

// send queues a message for SendMessage.
func (r *Robot) send(msg Message) { r.outbox = append(r.outbox, msg) }

// String is how a message appears in the log, e.g. "CLAIM dist 4 to r1".
func (msg Message) String() string {
	s := msg.kind
	switch msg.kind {
	case msgHelp:
		s += fmt.Sprintf(" at (%d, %d)", msg.row, msg.col)
	case msgClaim:
		s += fmt.Sprintf(" dist %d", msg.dist)
	}
	if msg.to != 0 {
		s += fmt.Sprintf(" to r%d", msg.to)
	}
	return s
}

// SendMessage broadcasts the outbox as one packet, each receiver with its own
// delay. It returns the log line, or "" if there was nothing to send.
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
			o.inbox = append(o.inbox, delivery{r.outbox, m.messageDelay()})
		}
	}
	r.outbox = nil
	return fmt.Sprintf("r%d broadcasts: %s", r.id, strings.Join(parts, "; "))
}

// ReceiveMessage reads the messages that are due and counts the rest down.
func (r *Robot) ReceiveMessage(m *Map) {
	pending := r.inbox
	r.inbox = nil
	for _, d := range pending {
		if d.wait == 0 {
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
	case msgHelp:
		atHelpCell := msg.row == r.row && msg.col == r.col
		if r.onTarget && msg.from < r.id {
			// Higher ID leaves a shared cell; an unanswered waiter helps a lower ID.
			idle := r.waitingSince != 0 && r.partner == 0 && len(r.claims) == 0 && m.turn-r.waitingSince >= giveUpAfter
			if atHelpCell || idle {
				r.stopWaiting()
			}
		}
		if r.onTarget || r.offer.owner != 0 || atHelpCell {
			return
		}
		r.offer = offer{owner: msg.from, seq: msg.seq, row: msg.row, col: msg.col}
		r.send(Message{kind: msgClaim, from: r.id, to: msg.from, seq: msg.seq,
			dist: abs(r.row-msg.row) + abs(r.col-msg.col)})

	case msgClaim:
		if !r.onTarget || msg.seq != r.seq || r.partner != 0 {
			r.release(msg.from, msg.seq) // too late, or no longer waiting
			return
		}
		if len(r.claims) == 0 {
			r.claimedAt = m.turn
			r.claims = map[int]int{}
		}
		r.claims[msg.from] = msg.dist

	case msgAccept:
		if r.offer.owner == msg.from && r.offer.seq == msg.seq {
			r.offer.accepted = true
		}

	case msgRelease:
		if r.offer.owner == msg.from && r.offer.seq == msg.seq {
			r.offer = offer{}
		}
	}
}

// release tells a helper it is not needed for request seq.
func (r *Robot) release(to, seq int) {
	r.send(Message{kind: msgRelease, from: r.id, to: to, seq: seq})
}

// startWaiting is called when the robot steps onto a target.
func (r *Robot) startWaiting() {
	r.resetJob()
	r.onTarget = true
	r.seq++
}

// stopWaiting gives up the target and releases every claimant.
func (r *Robot) stopWaiting() {
	for _, id := range slices.Sorted(maps.Keys(r.claims)) {
		r.release(id, r.seq)
	}
	r.resetJob()
}

// resetJob forgets the current job. seq is kept so late messages can be told apart.
func (r *Robot) resetJob() {
	r.onTarget = false
	r.partner, r.helpAt, r.waitingSince, r.claimedAt = 0, 0, 0, 0
	r.claims = nil
	r.offer = offer{}
}

// decideWaiting: call for help, accept the closest claim, pick up with the partner.
func (r *Robot) decideWaiting(s Sensing) string {
	if r.helpAt == 0 || (r.partner == 0 && s.turn-r.helpAt >= resendEvery) {
		if r.waitingSince == 0 {
			r.waitingSince = s.turn
		}
		r.helpAt = s.turn
		r.send(Message{kind: msgHelp, from: r.id, seq: r.seq, row: r.row, col: r.col})
	}

	if r.partner == 0 && len(r.claims) > 0 && s.turn-r.claimedAt >= acceptWindow {
		r.acceptClosest()
	}

	if r.partner != 0 && slices.Equal(s.robotsHere, []int{r.partner}) {
		return actionPickup
	}
	return actionStay
}

// acceptClosest accepts the nearest claim (lowest ID on a tie) and releases the rest.
func (r *Robot) acceptClosest() {
	best := 0
	for id, d := range r.claims {
		if best == 0 || d < r.claims[best] || (d == r.claims[best] && id < best) {
			best = id
		}
	}
	for _, id := range slices.Sorted(maps.Keys(r.claims)) {
		if id != best {
			r.release(id, r.seq)
		}
	}
	r.claims = map[int]int{best: r.claims[best]}
	r.partner = best
	r.send(Message{kind: msgAccept, from: r.id, to: best, seq: r.seq})
}

// decideHelping: walk to the waiter, then pick up when it is alone with us.
func (r *Robot) decideHelping(s Sensing) string {
	if r.row != r.offer.row || r.col != r.offer.col {
		return r.headToward(r.offer.row, r.offer.col)
	}
	if slices.Equal(s.robotsHere, []int{r.offer.owner}) {
		return actionPickup
	}
	return actionStay
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
