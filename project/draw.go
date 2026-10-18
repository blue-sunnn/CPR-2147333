package main

import (
	"bytes"
	"image/color"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"
)

// This file turns the views from visualize.go into pixels.

// Window layout, in pixels.
const (
	cellSize     = 52 // one square of the grid
	gap          = 4  // space between squares, and around the edge of the board
	margin       = 20 // space left and right of the board
	topBar       = 64 // room above the board for the status line
	bottomBar    = 44 // room below the board for the key hints
	boardSize    = N*cellSize + (N+1)*gap
	ScreenWidth  = boardSize + 2*margin
	ScreenHeight = topBar + boardSize + bottomBar
)

// Sizes of the things drawn on the board.
const (
	boardRadius = 10 // corner rounding of the board frame
	cellRadius  = 6  // corner rounding of each empty square

	robotSize     = 0.33 // from center to tip of a robot triangle, as a fraction of cellSize
	robotRounding = 6    // pixels; the outline that softens the triangle's corners
	shadowDrop    = 2    // pixels the robot's shadow sits below it

	labelSize    = 13 // font size of the robot ID
	minLabelSize = 7  // while a robot is still growing in, its label is too small to read

	starRadius     = 0.32 // from center to tip of a star, as a fraction of cellSize
	starInnerRatio = 0.4  // inner points sit this fraction of the way out
	starOutline    = 5    // pixels

	titleSize = 18
	hintSize  = 13
)

var (
	bgColor     = color.RGBA{0xfa, 0xf8, 0xef, 0xff}
	frameColor  = color.RGBA{0xbb, 0xad, 0xa0, 0xff}
	emptyColor  = color.RGBA{0xcd, 0xc1, 0xb4, 0xff}
	textColor   = color.RGBA{0x77, 0x6e, 0x65, 0xff}
	hintColor   = color.RGBA{0xa3, 0x9a, 0x8d, 0xff}
	starColor   = color.RGBA{0xed, 0xc2, 0x2e, 0xff}
	starEdge    = color.RGBA{0xd9, 0xa4, 0x1c, 0xff}
	shadowColor = color.NRGBA{0, 0, 0, 45}

	// Robot i gets robotColors[i % len(robotColors)], by position in the robot list.
	robotColors = []color.RGBA{
		{0xf6, 0x7c, 0x5f, 0xff},
		{0x6c, 0x9b, 0xd1, 0xff},
		{0x8c, 0xb3, 0x69, 0xff},
		{0xa4, 0x7b, 0xc0, 0xff},
	}

	fontSource *text.GoTextFaceSource
)

func init() {
	source, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		log.Fatal(err)
	}
	fontSource = source
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(bgColor)
	drawText(screen, g.title, ScreenWidth/2, topBar/2, titleSize, textColor)
	drawText(screen, "Space  move      Left / Right  turn      P  pick up",
		ScreenWidth/2, ScreenHeight-bottomBar/2, hintSize, hintColor)

	g.drawBoard(screen)

	for _, v := range g.tv {
		drawStar(screen, v)
	}
	// Bottom layer first, so later arrivals are drawn on top.
	for _, r := range g.order {
		v := g.rv[r]
		drawRobot(screen, v, r.label())
	}
}

// drawBoard draws the frame and the empty squares. They never change, so they
// are drawn into an image once and that image is reused.
func (g *Game) drawBoard(screen *ebiten.Image) {
	if g.board == nil {
		g.board = ebiten.NewImage(boardSize, boardSize)
		fillRoundRect(g.board, 0, 0, boardSize, boardSize, boardRadius, frameColor)
		for row := range N {
			for col := range N {
				x := float32(gap + col*(cellSize+gap))
				y := float32(gap + row*(cellSize+gap))
				fillRoundRect(g.board, x, y, cellSize, cellSize, cellRadius, emptyColor)
			}
		}
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(margin, topBar)
	screen.DrawImage(g.board, op)
}

// screenX and screenY convert a position in cell units (0 is the center of the
// first cell, 1 the center of the next) to a pixel on the window.
func screenX(c float64) float32 { return margin + gap + float32(c)*(cellSize+gap) + cellSize/2 }
func screenY(c float64) float32 { return topBar + gap + float32(c)*(cellSize+gap) + cellSize/2 }

// drawRobot draws a rounded triangle pointing the way the robot faces, with
// its ID on it.
func drawRobot(dst *ebiten.Image, v *view, label string) {
	cx, cy := screenX(v.x), screenY(v.y)
	size := float32(robotSize * cellSize * v.scale)
	rounding := float32(robotRounding * v.scale)

	drawShape(dst, triangle(cx, cy+shadowDrop, size, v.angle), shadowColor, rounding)
	drawShape(dst, triangle(cx, cy, size, v.angle), v.clr, rounding)

	// Slightly toward the wide end of the triangle, where there is more room.
	dx, dy := float32(math.Cos(v.angle)), float32(math.Sin(v.angle))
	if fontSize := labelSize * v.scale; fontSize >= minLabelSize {
		drawText(dst, label, float64(cx-size*dx/4), float64(cy-size*dy/4), fontSize, color.White)
	}
}

// triangle returns the corners of an isosceles triangle centered on (cx, cy).
// The tip is `size` away from the center, in the direction of `angle`.
func triangle(cx, cy, size float32, angle float64) [][2]float32 {
	dx, dy := float32(math.Cos(angle)), float32(math.Sin(angle)) // direction of the tip
	return [][2]float32{
		{cx + size*dx, cy + size*dy},                     // tip
		{cx - size*dx - size*dy, cy - size*dy + size*dx}, // back, one side
		{cx - size*dx + size*dy, cy - size*dy - size*dx}, // back, other side
	}
}

// drawStar draws a five-pointed target with one point facing up.
func drawStar(dst *ebiten.Image, v *view) {
	cx, cy := screenX(v.x), screenY(v.y)
	outer := float32(starRadius * cellSize * v.scale)

	// Ten corners, alternating between a tip and an inner notch.
	corners := make([][2]float32, 10)
	for k := range corners {
		radius := outer
		if k%2 == 1 {
			radius = outer * starInnerRatio
		}
		angle := -math.Pi/2 + float64(k)*math.Pi/5
		corners[k] = [2]float32{
			cx + radius*float32(math.Cos(angle)),
			cy + radius*float32(math.Sin(angle)),
		}
	}

	drawShape(dst, corners, starEdge, starOutline) // darker outline underneath
	drawShape(dst, corners, starColor, 0)
}

// drawShape fills a polygon. A positive `edge` also strokes the outline in the
// same color with round joins, which rounds off the corners.
func drawShape(dst *ebiten.Image, corners [][2]float32, clr color.Color, edge float32) {
	var path vector.Path
	for i, c := range corners {
		if i == 0 {
			path.MoveTo(c[0], c[1])
		} else {
			path.LineTo(c[0], c[1])
		}
	}
	path.Close()

	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, &path, nil, op)
	if edge > 0 {
		vector.StrokePath(dst, &path, &vector.StrokeOptions{Width: edge, LineJoin: vector.LineJoinRound}, op)
	}
}

// fillRoundRect fills a rectangle whose corners are quarter-curves of the given radius.
func fillRoundRect(dst *ebiten.Image, x, y, w, h, radius float32, clr color.Color) {
	var path vector.Path
	path.MoveTo(x+radius, y)
	path.LineTo(x+w-radius, y)
	path.QuadTo(x+w, y, x+w, y+radius)
	path.LineTo(x+w, y+h-radius)
	path.QuadTo(x+w, y+h, x+w-radius, y+h)
	path.LineTo(x+radius, y+h)
	path.QuadTo(x, y+h, x, y+h-radius)
	path.LineTo(x, y+radius)
	path.QuadTo(x, y, x+radius, y)
	path.Close()

	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, &path, nil, op)
}

// drawText draws centered text; (x, y) is the middle of the text.
func drawText(dst *ebiten.Image, s string, x, y, size float64, clr color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(clr)
	op.PrimaryAlign = text.AlignCenter
	op.SecondaryAlign = text.AlignCenter
	text.Draw(dst, s, &text.GoTextFace{Source: fontSource, Size: size}, op)
}
