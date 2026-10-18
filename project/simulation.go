package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// The rules of the simulation. Nothing here knows about the window.
//
// Every tick follows the same three steps, so an illegal action is caught
// before anything changes:
//  1. decide:  list what each robot wants to do (MoveAction / PickupAction)
//  2. check:   CheckMoveActions / CheckPickupActions sort them into legal and illegal
//  3. apply:   ApplyLegalMoves / ApplyLegalPickups carry out only the legal ones

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

// ------------------------------------------------------------ placement

// ValidatePlacement rejects a starting layout that breaks the rules: every
// robot needs a unique ID and a real facing, everything must start on the
// grid, and nothing may share a cell. (Robots may share a cell later, while
// the simulation runs; that is how two of them pick up a target.)
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

// ---------------------------------------------------------------- ticks

// MoveStep runs one MOVE tick for the given robots: every robot decides to
// step forward, the steps are checked, and only the legal ones happen. It
// returns a one-line verdict. `name` only labels the verdict.
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
		fmt.Println(verdict)
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
	verdict := "[" + name + "] " + describePickups(report)
	fmt.Println(verdict)

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

func describePickups(report PickupReport) string {
	var parts []string
	add := func(name string, cells []pos) {
		if len(cells) > 0 {
			parts = append(parts, name+" at "+formatCells(cells))
		}
	}
	add("NO_TARGET", report.noTarget)
	add("UNDERSTAFFED", report.understaffed)
	add("OVERSTAFFED", report.overstaffed)
	add("LEGAL pickup", report.legal)

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
