package main

import (
	"slices"
	"testing"
)

func TestApplyLegalMoves(t *testing.T) {
	r1 := &Robot{id: 1, row: 1, col: 3, facing: "N"}
	r2 := &Robot{id: 2, row: 0, col: 3, facing: "N"}
	m := &Map{size: 10}
	var actions []MoveAction
	for _, r := range []*Robot{r1, r2} {
		row, col := r.PreMove()
		actions = append(actions, MoveAction{r.id, row, col})
	}
	legal, _ := CheckMoveActions(m, actions)
	ApplyLegalMoves([]*Robot{r1, r2}, legal, 7)
	if r1.row != 0 || r2.row != 0 {
		t.Errorf("r1.row=%d r2.row=%d, want 0 and 0 (r2 blocked at the edge)", r1.row, r2.row)
	}
	if r1.arrived != 7 || r2.arrived != 0 {
		t.Errorf("arrived = %d and %d, want 7 (moved) and 0 (blocked)", r1.arrived, r2.arrived)
	}
}

func TestCheckMoveActions(t *testing.T) {
	m := &Map{size: 10}
	legal, out := CheckMoveActions(m, []MoveAction{{1, 0, 3}, {2, -1, 3}, {3, 9, 10}})
	if len(legal) != 1 || legal[0].robotID != 1 {
		t.Errorf("legal = %v, want only robot 1", legal)
	}
	if len(out) != 2 {
		t.Errorf("outOfBounds = %v, want robots 2 and 3", out)
	}
}

func TestCheckPickupActions(t *testing.T) {
	tgt := &Target{id: 0, row: 2, col: 2}
	at := func(ids ...int) []PickupAction {
		var a []PickupAction
		for _, id := range ids {
			a = append(a, PickupAction{id, 2, 2})
		}
		return a
	}
	tests := []struct {
		name                         string
		targets                      []*Target
		actions                      []PickupAction
		noTarget, under, over, legal int
	}{
		{"understaffed", []*Target{tgt}, at(1), 0, 1, 0, 0},
		{"legal", []*Target{tgt}, at(1, 2), 0, 0, 0, 1},
		{"overstaffed", []*Target{tgt}, at(1, 2, 3), 0, 0, 1, 0},
		{"no target + overstaffed", nil, at(1, 2, 3), 1, 0, 1, 0},
	}
	for _, tc := range tests {
		r := CheckPickupActions(tc.targets, tc.actions)
		got := [4]int{len(r.noTarget), len(r.understaffed), len(r.overstaffed), len(r.legal)}
		want := [4]int{tc.noTarget, tc.under, tc.over, tc.legal}
		if got != want || len(r.legalTargets) != tc.legal {
			t.Errorf("%s: got %v (legalTargets %d), want %v", tc.name, got, len(r.legalTargets), want)
		}
	}
}

func TestApplyLegalPickups(t *testing.T) {
	a := &Target{id: 0, row: 2, col: 2}
	b := &Target{id: 1, row: 5, col: 5}
	rep := CheckPickupActions([]*Target{a, b}, []PickupAction{{1, 2, 2}, {2, 2, 2}})
	left := ApplyLegalPickups([]*Target{a, b}, rep)
	if len(left) != 1 || left[0] != b {
		t.Errorf("left = %v, want only the target at (5,5)", left)
	}
}

func TestValidatePlacement(t *testing.T) {
	m := &Map{size: 10}
	r := func(id, row, col int) *Robot { return &Robot{id: id, row: row, col: col, facing: "N"} }
	tests := []struct {
		name    string
		robots  []*Robot
		targets []*Target
		ok      bool
	}{
		{"legal", []*Robot{r(1, 0, 0), r(2, 0, 1)}, []*Target{{id: 0, row: 5, col: 5}}, true},
		{"duplicate id", []*Robot{r(1, 0, 0), r(1, 0, 1)}, nil, false},
		{"robots on one cell", []*Robot{r(1, 0, 0), r(2, 0, 0)}, nil, false},
		{"robot on a target", []*Robot{r(1, 5, 5)}, []*Target{{id: 0, row: 5, col: 5}}, false},
		{"out of bounds", []*Robot{r(1, 10, 0)}, nil, false},
		{"invalid facing", []*Robot{{id: 1, row: 0, col: 0, facing: "X"}}, nil, false},
	}
	for _, tc := range tests {
		if err := ValidatePlacement(m, tc.robots, tc.targets); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok = %v", tc.name, err, tc.ok)
		}
	}
}

func TestDrawOrder(t *testing.T) {
	a := &Robot{id: 3, arrived: 1}
	b := &Robot{id: 2, arrived: 5} // arrived last: on top
	c := &Robot{id: 1, arrived: 5} // same turn as b, lower ID: below b
	d := &Robot{id: 4, arrived: 1} // same turn as a, higher ID: above a
	got := drawOrder([]*Robot{a, b, c, d})
	want := []*Robot{a, d, c, b}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order[%d] = r%d, want r%d", i, got[i].id, want[i].id)
		}
	}
}

func TestRandomPlacement(t *testing.T) {
	// A completely full grid is the hardest case: every cell gets used once.
	robots, targets := RandomPlacement(10, 60, 40)
	if len(robots) != 60 || len(targets) != 40 {
		t.Fatalf("got %d robots and %d targets, want 60 and 40", len(robots), len(targets))
	}
	if err := ValidatePlacement(&Map{size: 10}, robots, targets); err != nil {
		t.Errorf("random placement is not a legal layout: %v", err)
	}
}

func TestTick(t *testing.T) {
	m := &Map{size: 10}
	robots, targets := RandomPlacement(10, 5, 3)
	for i := 1; i <= 200; i++ {
		before := len(targets)
		_, targets = Tick(m, robots, targets)
		if m.turn != i {
			t.Fatalf("turn = %d after %d ticks", m.turn, i)
		}
		if len(targets) > before {
			t.Fatalf("targets grew from %d to %d", before, len(targets))
		}
		for _, r := range robots {
			if !m.IsInBounds(r.row, r.col) {
				t.Fatalf("%s left the grid at (%d, %d)", r.label(), r.row, r.col)
			}
		}
	}
}

func TestSense(t *testing.T) {
	r1 := &Robot{id: 1, row: 5, col: 5, facing: "E"}
	ahead := &Robot{id: 2, row: 5, col: 6, facing: "N"}
	here := &Robot{id: 3, row: 5, col: 5, facing: "N"}
	far := &Robot{id: 4, row: 0, col: 0, facing: "N"}
	robots := []*Robot{r1, ahead, here, far}
	targets := []*Target{{id: 0, row: 5, col: 6}}

	m := &Map{size: 10}

	got := r1.Sense(m, robots, targets)
	if !got.targetAhead || !slices.Equal(got.robotsAhead, []int{2}) || !slices.Equal(got.robotsHere, []int{3}) {
		t.Errorf("r1 sensed %+v, want target ahead, robot 2 ahead, robot 3 here", got)
	}

	// Facing the edge: nothing is in front, and a target behind is not sensed.
	if got := far.Sense(m, robots, targets); got.targetAhead || len(got.robotsAhead) != 0 || len(got.robotsHere) != 0 {
		t.Errorf("far sensed %+v, want nothing", got)
	}
	if got := far.Sense(m, robots, targets); !got.wallAhead {
		t.Errorf("far sensed %+v, want wall ahead", got)
	}
}

// history holds the last four actions, oldest first.
func TestRobotHistory(t *testing.T) {
	r := &Robot{id: 1, row: 5, col: 5, facing: "N"}

	// Four forward moves look the same as an empty history; the next turn must
	// still be recorded as the newest entry.
	for i := 0; i < 4; i++ {
		r.Move()
	}
	r.TurnLeft()
	if want := [4]int{Forward, Forward, Forward, Left}; r.history != want {
		t.Errorf("after 4 moves and a left turn history = %v, want %v", r.history, want)
	}

	r.TurnRight()
	r.TurnRight()
	if want := [4]int{Forward, Left, Right, Right}; r.history != want {
		t.Errorf("after a further right, right history = %v, want %v", r.history, want)
	}
}

// A robot that sees a target in front steps onto it and remembers it is there.
func TestRobotStepsOntoTarget(t *testing.T) {
	m := &Map{size: 10}
	r := &Robot{id: 1, row: 5, col: 4, facing: "E"}
	targets := []*Target{{id: 0, row: 5, col: 5}}

	Tick(m, []*Robot{r}, targets)

	if r.row != 5 || r.col != 5 {
		t.Fatalf("robot at (%d, %d), want it on the target at (5, 5)", r.row, r.col)
	}
	if !r.onTarget {
		t.Error("robot stepped onto the target but onTarget is false")
	}
}

// A robot on a target waits there, picking up every tick, until a partner joins.
func TestRobotWaitsOnTargetUntilPartnerArrives(t *testing.T) {
	m := &Map{size: 10}
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true}
	targets := []*Target{{id: 0, row: 5, col: 5}}

	for i := 0; i < 5; i++ {
		_, targets = Tick(m, []*Robot{waiter}, targets)
	}
	if waiter.row != 5 || waiter.col != 5 || !waiter.onTarget || len(targets) != 1 {
		t.Fatalf("lone waiter moved or left: at (%d, %d), onTarget=%v, %d targets left",
			waiter.row, waiter.col, waiter.onTarget, len(targets))
	}

	// The partner steps onto the target; next tick both pick up together.
	partner := &Robot{id: 2, row: 5, col: 4, facing: "E"}
	robots := []*Robot{waiter, partner}
	_, targets = Tick(m, robots, targets)
	_, targets = Tick(m, robots, targets)

	if len(targets) != 0 {
		t.Errorf("%d targets left, want the pair to have picked it up", len(targets))
	}
	if waiter.onTarget || partner.onTarget {
		t.Errorf("robots still waiting after the pickup: waiter=%v partner=%v", waiter.onTarget, partner.onTarget)
	}
}
