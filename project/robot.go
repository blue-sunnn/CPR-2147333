package main

import "fmt"

// A robot faces one of four directions: "N", "E", "S" or "W".
var (
	// deltas is the (row change, column change) of one step forward.
	deltas = map[string][2]int{"N": {-1, 0}, "E": {0, 1}, "S": {1, 0}, "W": {0, -1}}

	// leftOf and rightOf give the facing after turning 90 degrees.
	leftOf  = map[string]string{"N": "W", "W": "S", "S": "E", "E": "N"}
	rightOf = map[string]string{"N": "E", "E": "S", "S": "W", "W": "N"}
)

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

func (r *Robot) Move() {
	r.row, r.col = r.PreMove()
}

func (r *Robot) TurnLeft() {
	r.facing = leftOf[r.facing]
}

func (r *Robot) TurnRight() {
	r.facing = rightOf[r.facing]
}

func (r *Robot) Report() {
	fmt.Printf("Robot ID: %d, Position: (%d, %d), Facing: %s\n", r.id, r.row, r.col, r.facing)
}

// label is the name shown on screen and in messages, e.g. "r1".
func (r *Robot) label() string { return fmt.Sprintf("r%d", r.id) }
