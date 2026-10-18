package main

import "testing"

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
