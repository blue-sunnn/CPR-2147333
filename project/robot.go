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

const (
	weightForward = 65
	weightLeft    = 20
	weightRight   = 15
)

// Robot is one robot on the grid. IDs are unique and start at 1.
type Robot struct {
	id, row, col int
	facing       string
	arrived      int // the Map.turn when this robot last moved into its current cell
	history      [4]int
	onTarget     bool // true if the robot is currently on a target

	// Communication (comms.go).
	inbox  []delivery
	outbox []Message

	// Waiter: request number, first/last call and first claim turns, partner,
	// and the claims received (helper ID -> distance).
	seq, waitingSince, helpAt, claimedAt, partner int
	claims                                        map[int]int

	// Helper: the request it claimed or was accepted for.
	offer offer
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

// Decide picks this iteration's action: waiters wait, accepted helpers walk to
// their waiter (comms.go), everyone else wanders at random.
func (r *Robot) Decide(s Sensing) string {
	n := rand.IntN(100)

	switch {
	case r.onTarget:
		return r.decideWaiting(s)

	case r.offer.accepted:
		return r.decideHelping(s)

	case s.targetAhead && (len(s.robotsAhead) > 0 || r.offer.owner != 0):
		// Target taken, or a claim still unanswered: turn away.
		if n < 50 {
			return actionLeft
		}
		return actionRight

	case s.targetAhead:
		r.startWaiting()
		return actionMove

	case s.wallAhead:
		if n < 80 {
			return actionLeft
		} else {
			return actionRight
		}

	case r.history[1] != Forward && r.history[1] == r.history[2] && r.history[2] == r.history[3]:
		return actionMove

	default:
		switch {
		case n < weightForward:
			return actionMove
		case n < weightForward+weightLeft:
			return actionLeft
		default:
			return actionRight
		}
	}
}
