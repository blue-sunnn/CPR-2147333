package main

// Map is the square grid; rows and columns run 0..size-1, row 0 at the top.
type Map struct {
	size int
	turn int // iterations so far

	respawn, nextTarget int // targets added when none are left (0 = never); next new ID

	delay func() int // extra iterations per message per receiver; nil = none
}

func (m *Map) messageDelay() int {
	if m.delay == nil {
		return 0
	}
	return m.delay()
}

func (m *Map) IsInBounds(row, col int) bool {
	return row >= 0 && row < m.size && col >= 0 && col < m.size
}
