package main

import (
	"fmt"
	"math/rand/v2"
	"slices"
)

// A robot faces one of four directions: "N", "E", "S" or "W".
var (
	// deltas is the (row change, column change) of one step forward.
	deltas = map[string][2]int{"N": {-1, 0}, "E": {0, 1}, "S": {1, 0}, "W": {0, -1}}

	// leftOf and rightOf give the facing after turning 90 degrees.
	leftOf  = map[string]string{"N": "W", "W": "S", "S": "E", "E": "N"}
	rightOf = map[string]string{"N": "E", "E": "S", "S": "W", "W": "N"}
)

const (
	Left    = -1
	Forward = 0
	Right   = 1
)

// Robot is one robot on the grid. IDs are unique and start at 1.
type Robot struct {
	id, row, col int
	facing       string
	arrived      int // the Map.turn when this robot last moved into its current cell
	history      [4]int
	onTarget     bool // true if the robot is currently on a target

	// Communication (comms.go).
	inbox     []delivery
	meanDelay float64 // running average of the delay of the packets received; 0 = none yet
	outbox    []Message

	// Paxos state every robot keeps as acceptor and learner (comms.go), by instance.
	acc     map[int]*acceptor     // what it promised and accepted
	decided map[int]pair          // the chosen pair
	tally   map[vote]map[int]bool // senders of ACCEPTED seen, per proposal

	// Job: the target it works on and its partner.
	jobTarget, partner, goalRow, goalCol int  // jobTarget is the Paxos instance
	targetID, waitingSince               int  // targetID is the plain target ID
	sweepStage                           int  // sweep: 0 straight, 1 step into the next row, 2 turn back
	sweepLeft                            bool // sweep: which way the next U-turn goes
	acked, ready, ackSent                bool // handshake progress, see comms.go
	reservedFor, reservedUntil           int  // helper: held free for this instance until this turn

	// Proposer: one round at a time, while waiting.
	counter, propN, propAt int
	promises               map[int]Message // PROMISE received, by sender
	accSent                bool            // ACCEPT already sent for propN
}

// PreMove returns the cell a forward move would enter, without moving.
func (r *Robot) PreMove() (row, col int) {
	d := deltas[r.facing]
	return r.row + d[0], r.col + d[1]
}

// Move steps one cell forward; the simulation checks the cell first.
func (r *Robot) Move() {
	r.row, r.col = r.PreMove()
	r.record(Forward)
}

// TurnLeft and TurnRight rotate the robot 90 degrees in place.
func (r *Robot) TurnLeft() {
	r.facing = leftOf[r.facing]
	r.record(Left)
}

func (r *Robot) TurnRight() {
	r.facing = rightOf[r.facing]
	r.record(Right)
}

// record appends a move to history, dropping the oldest. Forward is 0, so a
// fresh history reads as four forward moves.
func (r *Robot) record(move int) {
	copy(r.history[:3], r.history[1:])
	r.history[3] = move
}

// label is the robot's name, e.g. "r1".
func (r *Robot) label() string { return fmt.Sprintf("r%d", r.id) }

// Sensing is what a robot can sense at the start of an iteration.
type Sensing struct {
	turn        int // the current iteration
	wallAhead   bool
	targetAhead bool  // a target is in the cell in front
	targetID    int   // the target ahead's ID, valid if targetAhead
	robotsAhead []int // IDs of the robots in the cell in front, lowest first
	robotsHere  []int // IDs of the other robots in the robot's own cell, lowest first
}

// Sense looks at the world as it is now, changing nothing.
func (r *Robot) Sense(m *Map, robots []*Robot, targets []*Target) Sensing {
	frontRow, frontCol := r.PreMove()

	s := Sensing{turn: m.turn}
	s.wallAhead = !m.IsInBounds(frontRow, frontCol)

	for _, t := range targets {
		if t.row == frontRow && t.col == frontCol {
			s.targetAhead = true
			s.targetID = t.id
		}
	}
	for _, o := range robots {
		switch {
		case o == r:
		case o.row == frontRow && o.col == frontCol:
			s.robotsAhead = append(s.robotsAhead, o.id)
		case o.row == r.row && o.col == r.col:
			s.robotsHere = append(s.robotsHere, o.id)
		}
	}
	slices.Sort(s.robotsAhead)
	slices.Sort(s.robotsHere)
	return s
}

// What a robot does in one iteration.
const (
	actionMove   = "MOVE"
	actionLeft   = "LEFT"
	actionRight  = "RIGHT"
	actionPickup = "PICKUP"
	actionStay   = "STAY"
)

// Decide picks this iteration's action: waiters run Paxos (comms.go), helpers walk
// to their waiter, everyone else sweeps the grid.
func (r *Robot) Decide(m *Map, s Sensing) string {
	n := rand.IntN(100)

	switch {
	case r.onTarget:
		return r.decideWaiting(m, s)

	case r.partner != 0:
		return r.decideHelping(m, s)

	case s.targetAhead && (len(s.robotsAhead) > 0 || r.decidedNow(s.targetID, s.turn) || r.reserved(s.turn)):
		// Target taken or reserved for a helper: turn away.
		if n < 50 {
			return actionLeft
		}
		return actionRight

	case s.targetAhead:
		r.startWaiting(s)
		return actionMove

	default:
		return r.sweep(m, s)
	}
}

// sweep covers the grid row by row: straight on, and at a wall step into the next
// row and turn back, alternating sides (and flipping when that side is off the grid).
func (r *Robot) sweep(m *Map, s Sensing) string {
	turn := func() string {
		if r.sweepLeft {
			return actionLeft
		}
		return actionRight
	}
	switch {
	case r.sweepStage == 1 && !s.wallAhead: // stage 1 can be stale if a target or job interrupted the sweep
		r.sweepStage = 2
		return actionMove
	case r.sweepStage == 2:
		t := turn()
		r.sweepStage, r.sweepLeft = 0, !r.sweepLeft
		return t
	case !s.wallAhead:
		return actionMove
	}
	side := rightOf[r.facing]
	if r.sweepLeft {
		side = leftOf[r.facing]
	}
	if d := deltas[side]; !m.IsInBounds(r.row+d[0], r.col+d[1]) {
		r.sweepLeft = !r.sweepLeft
	}
	r.sweepStage = 1
	return turn()
}
