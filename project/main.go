package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("CPR")
	if err := ebiten.RunGame(NewGame()); err != nil {
		log.Fatal(err)
	}
}
