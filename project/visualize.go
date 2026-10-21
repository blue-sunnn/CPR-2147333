package main

import (
	"cmp"
	"fmt"
	"image/color"
	"math"
	"slices"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Connects the simulation to the window: input, simulation steps and animated
// "views". The simulation jumps instantly; each view glides toward it a little
// every frame. draw.go turns the views into pixels.

const (
	easing      = 0.2 // fraction of the remaining distance a view covers each frame
	stackOffset = 0.2 // gap, in cells, between robots that share a cell

	tickFrames = 10 // frames between ticks while running (60 frames per second)
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

// Game is the window: the simulation state plus the views that animate it.
type Game struct {
	m       *Map
	robots  []*Robot
	targets []*Target

	panel      *Panel
	iterations int  // how many ticks a run lasts, from the panel
	running    bool // true while the simulation ticks by itself
	wait       int  // frames since the last tick

	rv    map[*Robot]*view
	tv    map[*Target]*view
	order []*Robot // robots from the bottom layer to the top layer, see drawOrder

	board *ebiten.Image // the empty board, drawn once and reused every frame
}

// NewGame returns an empty board; the panel's Apply button fills it.
func NewGame() *Game {
	return &Game{
		m:     &Map{size: N},
		rv:    map[*Robot]*view{},
		tv:    map[*Target]*view{},
		panel: NewPanel(),
	}
}

// apply reads the panel. If only the iterations changed, the run continues on
// the current board up to the new total; otherwise it starts a new random board.
func (g *Game) apply() {
	if slices.Contains(g.panel.values[:], "") {
		fmt.Println("[panel] fill in robots, targets and iterations first")
		return
	}
	robots, targets, iterations := g.panel.Value(fieldRobots), g.panel.Value(fieldTargets), g.panel.Value(fieldIterations)
	if robots+targets > N*N {
		fmt.Println(fmt.Sprintf("[panel] cannot fit %d robots and %d targets on a %dx%d grid", robots, targets, N, N))
		return
	}

	if robots == len(g.robots) && targets == g.m.respawn && g.robots != nil {
		g.iterations = iterations
		g.running = g.m.turn < iterations
		g.wait = 0
		if g.running {
			fmt.Println(fmt.Sprintf("[panel] continuing from iteration %d up to %d", g.m.turn, iterations))
		} else {
			fmt.Println(fmt.Sprintf("[panel] already at iteration %d; enter more than that to continue", g.m.turn))
		}
		return
	}

	g.robots, g.targets = RandomPlacement(N, robots, targets)
	g.m.respawn, g.m.nextTarget = targets, targets
	g.iterations = iterations
	g.running = iterations > 0
	g.wait = 0
	g.m.turn = 0
	clear(g.rv)
	clear(g.tv)
	g.order = nil
	fmt.Println(fmt.Sprintf("[panel] new run: %d robots, %d targets, %d iterations", robots, targets, iterations))
}

// Layout, Update and Draw (in draw.go) are what Ebitengine calls to run the window.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}

// Update runs 60 times a second: it reads the panel, advances the simulation,
// then moves the animated views one step.
func (g *Game) Update() error {
	if g.panel.Update() {
		g.apply()
	}
	if g.running {
		g.runTick()
	} else {
		g.handleKeys()
	}
	g.animate()
	return nil
}

// runTick runs one tick every tickFrames frames until the iterations are used up.
func (g *Game) runTick() {
	g.wait++
	if g.wait < tickFrames {
		return
	}
	g.wait = 0

	var verdict string
	verdict, g.targets = Tick(g.m, g.robots, g.targets)
	fmt.Println(verdict)
	if g.m.turn >= g.iterations {
		g.running = false
		fmt.Println(fmt.Sprintf("[sim] finished after %d iterations", g.iterations))
	}
}

// handleKeys is a manual stepper: each key press runs one tick for every robot.
func (g *Game) handleKeys() {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		fmt.Println(MoveStep(g.m, g.robots, "manual"))
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft):
		for _, r := range g.robots {
			r.TurnLeft()
		}
		fmt.Println("[manual] TURN LEFT")
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowRight):
		for _, r := range g.robots {
			r.TurnRight()
		}
		fmt.Println("[manual] TURN RIGHT")
	case inpututil.IsKeyJustPressed(ebiten.KeyP):
		var verdict string
		verdict, g.targets = PickupStep(g.robots, g.targets, "manual")
		fmt.Println(verdict)
	}
}

// animate moves every view one frame closer to the simulation state.
func (g *Game) animate() {
	g.animateRobots()
	g.animateTargets()
}

// animateRobots glides each robot view to its cell, direction and stack position.
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

// animateTargets grows new targets in and shrinks picked-up ones away.
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

// stackShift nudges robot number `layer` of `count` sharing a cell, keeping the group centered.
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

// ------------------------------------------------------- settings panel

// Right of the board: robots, targets and iterations fields plus Apply.
// Click a field to type, Tab moves on, Enter applies.

const (
	fieldRobots = iota
	fieldTargets
	fieldIterations
	fieldCount
)

const (
	panelPad     = 16
	panelHead    = 44 // room for the heading at the top of the panel
	fieldLabelH  = 24 // room for a field's label above its box
	fieldHeight  = 36
	fieldSpacing = 16 // between one field box and the next label
	buttonHeight = 40

	backspaceDelay  = 30 // frames the key is held before it starts repeating
	backspaceRepeat = 3  // then it deletes once every this many frames
)

var fieldLabels = [fieldCount]string{"Robots", "Targets", "Iterations"}

// maxDigits per field; robots and targets are bounded by the grid.
var maxDigits = [fieldCount]int{3, 3, 6}

var (
	fieldColor  = color.RGBA{0xfa, 0xf8, 0xef, 0xff}
	buttonColor = color.RGBA{0xf6, 0x7c, 0x5f, 0xff}
)

// Panel is the settings panel: the three number fields as typed text.
type Panel struct {
	values [fieldCount]string
	focus  int // index of the field being typed in, or -1
	frames int // counts frames, to blink the caret
}

// NewPanel returns a panel with empty fields and nothing focused.
func NewPanel() *Panel {
	return &Panel{focus: -1}
}

// Value is the number typed in field i; an empty field counts as 0.
func (p *Panel) Value(i int) int {
	n, _ := strconv.Atoi(p.values[i])
	return n
}

// rect is a rectangle in window pixels.
type rect struct{ x, y, w, h float32 }

func (r rect) contains(px, py int) bool {
	return float32(px) >= r.x && float32(px) < r.x+r.w && float32(py) >= r.y && float32(py) < r.y+r.h
}

func panelRect() rect {
	return rect{margin + boardSize + margin, topBar, panelWidth, boardSize}
}

func fieldRect(i int) rect {
	pr := panelRect()
	y := pr.y + panelHead + float32(i)*(fieldLabelH+fieldHeight+fieldSpacing) + fieldLabelH
	return rect{pr.x + panelPad, y, pr.w - 2*panelPad, fieldHeight}
}

func buttonRect() rect {
	last := fieldRect(fieldCount - 1)
	return rect{last.x, last.y + last.h + 2*fieldSpacing, last.w, buttonHeight}
}

// Update handles the mouse and keyboard. It reports true when the user asks
// to apply the settings.
func (p *Panel) Update() (apply bool) {
	p.frames++

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		p.focus = -1
		for i := range p.values {
			if fieldRect(i).contains(mx, my) {
				p.focus = i
			}
		}
		if buttonRect().contains(mx, my) {
			apply = true
		}
	}

	if p.focus < 0 {
		return apply
	}

	v := &p.values[p.focus]
	for _, c := range ebiten.AppendInputChars(nil) {
		if c >= '0' && c <= '9' && len(*v) < maxDigits[p.focus] {
			*v += string(c)
		}
	}
	held := inpututil.KeyPressDuration(ebiten.KeyBackspace)
	if len(*v) > 0 && (held == 1 || held > backspaceDelay && held%backspaceRepeat == 0) {
		*v = (*v)[:len(*v)-1]
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		p.focus = (p.focus + 1) % fieldCount
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		apply = true
	}
	return apply
}
