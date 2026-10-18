package main

import (
	"cmp"
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// This file connects the simulation to the window: it reads the keyboard,
// runs the simulation steps, and keeps the animated "views" up to date.
// draw.go turns those views into pixels.
//
// The simulation changes instantly (a robot jumps from one cell to the next).
// Each robot and target has a view that glides toward the simulation state a
// little every frame, which is what makes the movement look smooth.

const (
	easing      = 0.2 // fraction of the remaining distance a view covers each frame
	stackOffset = 0.2 // gap, in cells, between robots that share a cell
)

// view is how one robot or target is currently drawn.
type view struct {
	x, y  float64 // position in cell units, so 2.5 is halfway across the third cell
	angle float64 // direction the robot points, in radians (robots only)
	scale float64 // 0 is invisible, 1 is full size
	alive bool    // false once a target has been picked up and is shrinking away (targets only)
	clr   color.RGBA
}

// ease moves *cur a fraction of the way to goal, and snaps once it is close.
func ease(cur *float64, goal float64) {
	*cur += (goal - *cur) * easing
	if math.Abs(goal-*cur) < 0.001 {
		*cur = goal
	}
}

// Game is the window. It owns the simulation state (the map, the robots and
// the targets) and the views that animate it.
type Game struct {
	m       *Map
	robots  []*Robot
	targets []*Target
	title   string // status line shown above the board

	rv    map[*Robot]*view
	tv    map[*Target]*view
	order []*Robot // robots from the bottom layer to the top layer, see drawOrder

	board *ebiten.Image // the empty board, drawn once and reused every frame
}

// NewGame checks that the starting layout is legal and builds the window state.
func NewGame(size int, robots []*Robot, targets []*Target) (*Game, error) {
	m := &Map{size: size}
	if err := ValidatePlacement(m, robots, targets); err != nil {
		return nil, err
	}
	return &Game{
		m:       m,
		robots:  robots,
		targets: targets,
		rv:      map[*Robot]*view{},
		tv:      map[*Target]*view{},
	}, nil
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}

func (g *Game) Update() error {
	g.handleKeys()
	g.animate()
	return nil
}

// handleKeys is a manual stepper: each key press runs one tick for every robot.
func (g *Game) handleKeys() {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		g.title = MoveStep(g.m, g.robots, "manual")
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft):
		for _, r := range g.robots {
			r.TurnLeft()
		}
		g.title = "Turn left"
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowRight):
		for _, r := range g.robots {
			r.TurnRight()
		}
		g.title = "Turn right"
	case inpututil.IsKeyJustPressed(ebiten.KeyP):
		g.title, g.targets = PickupStep(g.robots, g.targets, "manual")
	}
}

// animate moves every view one frame closer to the simulation state.
func (g *Game) animate() {
	g.animateRobots()
	g.animateTargets()
}

func (g *Game) animateRobots() {
	for i, r := range g.robots {
		if g.rv[r] == nil {
			g.rv[r] = newRobotView(r, robotColors[i%len(robotColors)])
		}
	}
	g.order = drawOrder(g.robots)

	robotsInCell := map[pos]int{}
	for _, r := range g.robots {
		robotsInCell[pos{r.row, r.col}]++
	}

	// Robots sharing a cell are stacked in drawing order, each one shifted a
	// little so the ones underneath stay visible. `layer` counts how many
	// robots of each cell have been placed so far.
	layer := map[pos]int{}
	for _, r := range g.order {
		cell := pos{r.row, r.col}
		shift := stackShift(layer[cell], robotsInCell[cell])
		layer[cell]++

		v := g.rv[r]
		ease(&v.x, float64(r.col)+shift)
		ease(&v.y, float64(r.row)+shift)
		ease(&v.scale, 1)
		v.angle += turnToward(v.angle, facingAngle(r.facing)) * easing
	}
}

func (g *Game) animateTargets() {
	// A target that is no longer in the simulation has been picked up.
	for _, v := range g.tv {
		v.alive = false
	}
	for _, t := range g.targets {
		v := g.tv[t]
		if v == nil {
			v = &view{x: float64(t.col), y: float64(t.row)} // starts at scale 0 and grows in
			g.tv[t] = v
		}
		v.alive = true
	}

	for t, v := range g.tv {
		goal := 0.0
		if v.alive {
			goal = 1
		}
		ease(&v.scale, goal)

		if !v.alive && v.scale < 0.01 {
			delete(g.tv, t) // fully shrunk away
		}
	}
}

// newRobotView creates a view that appears at the robot's cell and grows in.
func newRobotView(r *Robot, clr color.RGBA) *view {
	return &view{x: float64(r.col), y: float64(r.row), angle: facingAngle(r.facing), clr: clr}
}

// drawOrder layers robots: whoever arrived at its cell first is drawn first
// (below); robots that arrived on the same turn are ordered by ID, lower below.
func drawOrder(robots []*Robot) []*Robot {
	order := slices.Clone(robots)
	slices.SortFunc(order, func(a, b *Robot) int {
		return cmp.Or(cmp.Compare(a.arrived, b.arrived), cmp.Compare(a.id, b.id))
	})
	return order
}

// stackShift is how far, in cells, a robot is nudged along both axes when
// `count` robots share a cell and it is number `layer` from the bottom.
// The group stays centered on the cell.
func stackShift(layer, count int) float64 {
	return (float64(layer) - float64(count-1)/2) * stackOffset
}

// facingAngle is the angle a robot facing N, E, S or W points at on screen.
// Rows grow downward, so N is -90 degrees and E is 0.
func facingAngle(facing string) float64 {
	d := deltas[facing]
	return math.Atan2(float64(d[0]), float64(d[1]))
}

// turnToward is the shortest rotation, between -180 and 180 degrees, that
// takes the angle `from` to the angle `to`.
func turnToward(from, to float64) float64 {
	turn := to - from
	for turn > math.Pi {
		turn -= 2 * math.Pi
	}
	for turn < -math.Pi {
		turn += 2 * math.Pi
	}
	return turn
}
