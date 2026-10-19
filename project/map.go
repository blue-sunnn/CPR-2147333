package main

// Map is the square grid the robots live on. Rows and columns both run
// 0..size-1, and row 0 is the top of the screen.
type Map struct {
	size int
	turn int // iterations so far; robots record it as their arrival time
}

func (m *Map) IsInBounds(row, col int) bool {
	return row >= 0 && row < m.size && col >= 0 && col < m.size
}
