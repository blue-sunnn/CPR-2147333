package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	// A starting layout to look at. Replace it with the real setup (random
	// targets, the robot count from the project description) when the full
	// robot behavior is built.
	robots := []*Robot{
		{id: 1, row: 2, col: 0, facing: "E"},
		{id: 2, row: 0, col: 2, facing: "S"},
		{id: 3, row: 6, col: 6, facing: "W"},
		{id: 4, row: 8, col: 4, facing: "N"},
	}
	targets := []*Target{
		{id: 0, row: 2, col: 2},
		{id: 1, row: 6, col: 4},
	}

	game, err := NewGame(N, robots, targets)
	if err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("CPR")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
