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

// Robot is one robot on the grid. IDs are unique and start at 1.
type Robot struct {
	id, row, col int
	facing       string
	arrived      int // the Map.turn when this robot last moved into its current cell
}

// PreMove returns the cell the robot would enter if it moved forward,
// without moving it. The simulation checks that cell before calling Move.
func (r *Robot) PreMove() (row, col int) {
	d := deltas[r.facing]
	return r.row + d[0], r.col + d[1]
}

// Move steps one cell forward. The simulation checks the step first (see PreMove).
func (r *Robot) Move() {
	r.row, r.col = r.PreMove()
}

// TurnLeft and TurnRight rotate the robot 90 degrees in place.
func (r *Robot) TurnLeft() {
	r.facing = leftOf[r.facing]
}

func (r *Robot) TurnRight() {
	r.facing = rightOf[r.facing]
}

// label is the name shown on screen and in messages, e.g. "r1".
func (r *Robot) label() string { return fmt.Sprintf("r%d", r.id) }

// Sensing is what a robot can sense at the start of an iteration.
type Sensing struct {
	targetAhead bool  // a target is in the cell in front
	robotsAhead []int // IDs of the robots in the cell in front, lowest first
	robotsHere  []int // IDs of the other robots in the robot's own cell, lowest first
}

// Sense looks at the world as it is right now, without changing anything.
func (r *Robot) Sense(robots []*Robot, targets []*Target) Sensing {
	frontRow, frontCol := r.PreMove()

	var s Sensing
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

// The four things a robot can do in one iteration.
const (
	actionMove   = "MOVE"
	actionLeft   = "LEFT"
	actionRight  = "RIGHT"
	actionPickup = "PICKUP"
)

// allActions lists them in the order they are reported in the log.
var allActions = []string{actionMove, actionLeft, actionRight, actionPickup}

// Decide picks this iteration's action (one of allActions) from what the robot
// sensed. This is a placeholder that ignores `s` and acts at random; replace it
// with the real rules.
func (r *Robot) Decide(s Sensing) string { 
	// switch {
	// 	// actionMove
	// 	case s.targetAhead: return actionMove

	// 	// actionRight

	// 	// actionLeft

	// 	// actionPickup
		
	// }

	return allActions[rand.IntN(len(allActions))]
}
