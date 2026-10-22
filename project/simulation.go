package main

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
)

// The rules of the simulation (nothing here knows about the window). Every tick:
// decide what each robot wants, check it (legal or illegal), apply only the
// legal actions. MoveStep and PickupStep do this for one action kind (keyboard).

// N is the width and height of the grid.
const N = 10

// pos is a cell on the grid.
type pos struct{ row, col int }

// ---------------------------------------------------------------- moves

// MoveAction is a robot's wish to move into the cell (row, col).
type MoveAction struct {
	robotID, row, col int
}

// CheckMoveActions splits the actions into those that stay on the grid and
// those that would leave it.
func CheckMoveActions(m *Map, actions []MoveAction) (legal, outOfBounds []MoveAction) {
	for _, a := range actions {
		if m.IsInBounds(a.row, a.col) {
			legal = append(legal, a)
		} else {
			outOfBounds = append(outOfBounds, a)
		}
	}
	return legal, outOfBounds
}

// ApplyLegalMoves moves every robot that has a legal action and records the
// turn it arrived on. A robot whose move was rejected stays where it is.
func ApplyLegalMoves(robots []*Robot, legal []MoveAction, turn int) {
	canMove := map[int]bool{}
	for _, a := range legal {
		canMove[a.robotID] = true
	}
	for _, r := range robots {
		if canMove[r.id] {
			r.Move()
			r.arrived = turn
		}
	}
}

// -------------------------------------------------------------- pickups

// PickupAction is a robot's wish to pick up a target from the cell (row, col).
type PickupAction struct {
	robotID, row, col int
}

// PickupReport sorts the cells where a pickup was attempted. A cell can be in
// more than one list (for example noTarget and overstaffed).
type PickupReport struct {
	noTarget     []pos     // at least one pickup, but no target there
	understaffed []pos     // a target there, but only one robot
	overstaffed  []pos     // more than two robots
	legal        []pos     // a target there, and exactly two robots
	legalTargets []*Target // the targets at the legal cells
}

// hasIllegal reports whether any attempted pickup broke the rule.
func (r PickupReport) hasIllegal() bool {
	return len(r.noTarget)+len(r.understaffed)+len(r.overstaffed) > 0
}

// CheckPickupActions applies the pickup rule: a target can only be picked up
// by exactly two robots at the same time.
func CheckPickupActions(targets []*Target, actions []PickupAction) PickupReport {
	targetAt := map[pos]*Target{}
	for _, t := range targets {
		targetAt[pos{t.row, t.col}] = t
	}

	// Count the pickups per cell. `cells` remembers the order cells were first
	// seen so the report comes out in the same order every run.
	pickupsAt := map[pos]int{}
	var cells []pos
	for _, a := range actions {
		cell := pos{a.row, a.col}
		if pickupsAt[cell] == 0 {
			cells = append(cells, cell)
		}
		pickupsAt[cell]++
	}

	var report PickupReport
	for _, cell := range cells {
		count := pickupsAt[cell]
		target := targetAt[cell]

		switch {
		case target == nil:
			report.noTarget = append(report.noTarget, cell)
		case count == 1:
			report.understaffed = append(report.understaffed, cell)
		case count == 2:
			report.legal = append(report.legal, cell)
			report.legalTargets = append(report.legalTargets, target)
		}

		// Too many robots is illegal whether or not there is a target.
		if count > 2 {
			report.overstaffed = append(report.overstaffed, cell)
		}
	}
	return report
}

// ApplyLegalPickups returns the targets that are still on the grid after the
// legal pickups. Illegal pickups leave their targets alone.
func ApplyLegalPickups(targets []*Target, report PickupReport) []*Target {
	var remaining []*Target
	for _, t := range targets {
		if !slices.Contains(report.legalTargets, t) {
			remaining = append(remaining, t)
		}
	}
	return remaining
}

// ReleaseRobots resets the robots that took part in a legal pickup. A robot that
// merely walked into the cell keeps its own job.
func ReleaseRobots(robots []*Robot, actions []PickupAction, report PickupReport) {
	for _, a := range actions {
		if !slices.Contains(report.legal, pos{a.row, a.col}) {
			continue
		}
		for _, r := range robots {
			if r.id == a.robotID {
				r.resetJob()
			}
		}
	}
}

// ------------------------------------------------------------ placement

// ValidatePlacement rejects a starting layout with duplicate IDs, bad facings,
// cells off the grid or two things on one cell (robots may share cells later).
func ValidatePlacement(m *Map, robots []*Robot, targets []*Target) error {
	ids := map[int]bool{}
	for _, r := range robots {
		if ids[r.id] {
			return fmt.Errorf("duplicate robot ID %d", r.id)
		}
		ids[r.id] = true

		if _, ok := deltas[r.facing]; !ok {
			return fmt.Errorf("%s has an invalid facing %q", r.label(), r.facing)
		}
	}

	occupant := map[pos]string{} // who starts on each cell
	place := func(label string, cell pos) error {
		if !m.IsInBounds(cell.row, cell.col) {
			return fmt.Errorf("%s starts outside the %dx%d grid", label, m.size, m.size)
		}
		if other, taken := occupant[cell]; taken {
			return fmt.Errorf("%s starts on the same cell as %s", label, other)
		}
		occupant[cell] = label
		return nil
	}
	for _, r := range robots {
		if err := place(r.label(), pos{r.row, r.col}); err != nil {
			return err
		}
	}
	for _, t := range targets {
		if err := place(t.label(), pos{t.row, t.col}); err != nil {
			return err
		}
	}
	return nil
}

// RandomPlacement puts robots (IDs 1, 2, ...) and targets (IDs 0, 1, ...) on
// distinct random cells, robots facing randomly. The caller checks it all fits.
func RandomPlacement(n, robots, targets int) ([]*Robot, []*Target) {
	cells := rand.Perm(n * n)
	facings := []string{"N", "E", "S", "W"}

	var rs []*Robot
	for i := range robots {
		c := cells[i]
		rs = append(rs, &Robot{id: i + 1, row: c / n, col: c % n, facing: facings[rand.IntN(len(facings))]})
	}
	var ts []*Target
	for i := range targets {
		c := cells[robots+i]
		ts = append(ts, &Target{id: i, row: c / n, col: c % n})
	}
	return rs, ts
}

// SpawnTargets adds m.respawn targets on random cells without a robot.
func SpawnTargets(m *Map, robots []*Robot) []*Target {
	occupied := map[pos]bool{}
	for _, r := range robots {
		occupied[pos{r.row, r.col}] = true
	}

	var targets []*Target
	for _, c := range rand.Perm(m.size * m.size) {
		if len(targets) == m.respawn {
			break
		}
		cell := pos{c / m.size, c % m.size}
		if !occupied[cell] {
			targets = append(targets, &Target{id: m.nextTarget, row: cell.row, col: cell.col})
			m.nextTarget++
		}
	}
	return targets
}

// ---------------------------------------------------------------- ticks

// MoveStep runs one tick where every robot steps forward if legal. Returns a verdict.
func MoveStep(m *Map, robots []*Robot, name string) string {
	m.turn++

	actions := make([]MoveAction, len(robots))
	for i, r := range robots {
		row, col := r.PreMove()
		actions[i] = MoveAction{r.id, row, col}
	}

	legal, outOfBounds := CheckMoveActions(m, actions)
	verdict := "[" + name + "] MOVE"
	if len(outOfBounds) > 0 {
		verdict = "[" + name + "] " + describeMoves(outOfBounds)
	}

	ApplyLegalMoves(robots, legal, m.turn)
	return verdict
}

// PickupStep runs one PICKUP tick: every robot tries to pick up from the cell
// it stands on. It returns a one-line verdict and the targets that remain.
func PickupStep(robots []*Robot, targets []*Target, name string) (string, []*Target) {
	actions := make([]PickupAction, len(robots))
	for i, r := range robots {
		actions[i] = PickupAction{r.id, r.row, r.col}
	}

	report := CheckPickupActions(targets, actions)
	verdict := "[" + name + "] " + describePickups(report, actions)

	ReleaseRobots(robots, actions, report)
	return verdict, ApplyLegalPickups(targets, report)
}

// Tick runs one iteration, in the order the project requires: robots read their
// messages, sense and decide, the moves and pickups are checked, only the legal
// ones happen, and finally the robots send their messages. The iteration number
// is m.turn. It returns a one-line verdict and the targets that remain.
func Tick(m *Map, robots []*Robot, targets []*Target) (string, []*Target) {
	m.turn++

	// When no targets are left, a new set appears before anyone moves.
	spawned := 0
	if len(targets) == 0 && m.respawn > 0 {
		targets = SpawnTargets(m, robots)
		spawned = len(targets)
	}

	for _, r := range robots {
		r.ReceiveMessage(m)
	}

	// Everyone senses and decides before anything changes, so no robot
	// sees the result of another robot's action this iteration.
	chosen := make([]string, len(robots))
	for i, r := range robots {
		chosen[i] = r.Decide(r.Sense(m, robots, targets))
	}

	var moves []MoveAction
	var pickups []PickupAction
	for i, r := range robots {
		switch chosen[i] {
		case actionMove:
			row, col := r.PreMove()
			moves = append(moves, MoveAction{r.id, row, col})
		case actionLeft: // turning is always legal, so it needs no check
			r.TurnLeft()
		case actionRight:
			r.TurnRight()
		case actionPickup:
			pickups = append(pickups, PickupAction{r.id, r.row, r.col})
		}
	}

	legalMoves, outOfBounds := CheckMoveActions(m, moves)
	report := CheckPickupActions(targets, pickups)
	m.illegal = len(outOfBounds) > 0 || report.hasIllegal()
	ApplyLegalMoves(robots, legalMoves, m.turn)

	var events []string
	if spawned > 0 {
		events = append(events, fmt.Sprintf("spawned %d new targets", spawned))
	}
	if len(outOfBounds) > 0 {
		events = append(events, describeMoves(outOfBounds))
	}
	if len(pickups) > 0 {
		events = append(events, describePickups(report, pickups))
	}
	ReleaseRobots(robots, pickups, report)
	for _, r := range robots {
		if line := r.SendMessage(m, robots); line != "" {
			events = append(events, line)
		}
	}

	verdict := fmt.Sprintf("[%d]", m.turn)
	if len(events) > 0 {
		verdict += "\n  " + strings.Join(events, "\n  ")
	}
	return verdict, ApplyLegalPickups(targets, report)
}

// ------------------------------------------------------ verdict messages

func describeMoves(outOfBounds []MoveAction) string {
	sorted := slices.Clone(outOfBounds)
	slices.SortFunc(sorted, func(a, b MoveAction) int { return cmp.Compare(a.robotID, b.robotID) })

	var parts []string
	for _, a := range sorted {
		parts = append(parts, fmt.Sprintf("('r%d', (%d, %d))", a.robotID, a.row, a.col))
	}
	return fmt.Sprintf("OUT_OF_BOUNDS MOVE by [%s] (blocked)", strings.Join(parts, ", "))
}

func describePickups(report PickupReport, actions []PickupAction) string {
	var parts []string
	add := func(name string, cells []pos) {
		if len(cells) > 0 {
			parts = append(parts, name+" at "+formatCells(cells))
		}
	}
	add("NO_TARGET", report.noTarget)
	add("UNDERSTAFFED", report.understaffed)
	add("OVERSTAFFED", report.overstaffed)
	for _, cell := range report.legal {
		var ids []string
		for _, a := range actions {
			if a.row == cell.row && a.col == cell.col {
				ids = append(ids, fmt.Sprintf("r%d", a.robotID))
			}
		}
		parts = append(parts, fmt.Sprintf("LEGAL pickup at (%d, %d) by %s", cell.row, cell.col, strings.Join(ids, ", ")))
	}

	if len(parts) == 0 {
		return "no PICKUP decided this tick"
	}
	return strings.Join(parts, "; ")
}

// formatCells lists cells in row-then-column order, e.g. "[(1, 1), (5, 5)]".
func formatCells(cells []pos) string {
	sorted := slices.Clone(cells)
	slices.SortFunc(sorted, func(a, b pos) int {
		return cmp.Or(cmp.Compare(a.row, b.row), cmp.Compare(a.col, b.col))
	})

	var parts []string
	for _, c := range sorted {
		parts = append(parts, fmt.Sprintf("(%d, %d)", c.row, c.col))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
