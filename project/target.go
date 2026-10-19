package main

import "fmt"

// Target sits on one cell until two robots pick it up together.
type Target struct {
	id, row, col int
}

// label is the name shown in messages, e.g. "target_0".
func (t *Target) label() string { return fmt.Sprintf("target_%d", t.id) }
