package main

import (
	"math/rand/v2"
	"slices"
	"strings"
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

// A lone robot on a target keeps waiting; it never picks up on its own.
func TestLoneWaiterKeepsWaiting(t *testing.T) {
	m := &Map{size: 10}
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true}
	targets := []*Target{{id: 0, row: 5, col: 5}}

	for i := 0; i < 30; i++ {
		_, targets = Tick(m, []*Robot{waiter}, targets)
	}
	if waiter.row != 5 || waiter.col != 5 || !waiter.onTarget || len(targets) != 1 {
		t.Fatalf("lone waiter moved or left: at (%d, %d), onTarget=%v, %d targets left",
			waiter.row, waiter.col, waiter.onTarget, len(targets))
	}
}

// illegal reports whether a verdict mentions one of the illegal pickups.
func illegal(verdict string) bool {
	return strings.Contains(verdict, "NO_TARGET") ||
		strings.Contains(verdict, "UNDERSTAFFED") ||
		strings.Contains(verdict, "OVERSTAFFED")
}

// runUntilDone ticks until every target is gone (or maxTicks pass) and fails
// the test on an illegal pickup. It returns the ticks used.
func runUntilDone(t *testing.T, m *Map, robots []*Robot, targets []*Target, maxTicks int) ([]*Target, int) {
	t.Helper()
	for len(targets) > 0 && m.turn < maxTicks {
		var verdict string
		verdict, targets = Tick(m, robots, targets)
		if illegal(verdict) {
			t.Fatalf("illegal behaviour: %s", verdict)
		}
	}
	return targets, m.turn
}

// A waiter calls for help and the closest free robot walks over; the far robot stays out.
func TestClosestRobotAnswersTheCall(t *testing.T) {
	m := &Map{size: 10}
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true}
	far := &Robot{id: 2, row: 0, col: 9, facing: "S"}
	near := &Robot{id: 3, row: 5, col: 3, facing: "E"}
	robots := []*Robot{waiter, far, near}
	targets := []*Target{{id: 0, row: 5, col: 5}}

	partner := 0
	for i := 0; i < 10 && partner == 0; i++ {
		Tick(m, robots, targets)
		partner = waiter.partner
	}
	if partner != near.id {
		t.Fatalf("partner = r%d, want the closer robot r%d", partner, near.id)
	}

	targets, _ = runUntilDone(t, m, robots, targets, 100)
	if len(targets) != 0 {
		t.Fatal("the pair never picked up the target")
	}
	for _, r := range robots {
		if r.onTarget || r.offer.owner != 0 || r.partner != 0 {
			t.Errorf("%s still holds a job after the pickup", r.label())
		}
	}
}

// Robots that step onto the same target in the same iteration sort it out:
// the higher IDs leave and the lowest ID gets a partner.
func TestRobotsArrivingTogetherSortItOut(t *testing.T) {
	for _, n := range []int{2, 3} {
		m := &Map{size: 10}
		robots := []*Robot{
			{id: 1, row: 5, col: 4, facing: "E"},
			{id: 2, row: 5, col: 6, facing: "W"},
			{id: 3, row: 4, col: 5, facing: "S"},
		}[:n]
		targets, ticks := runUntilDone(t, m, robots, []*Target{{id: 0, row: 5, col: 5}}, 500)
		if len(targets) != 0 {
			t.Errorf("%d robots arriving together: target still there after %d ticks", n, ticks)
		}
	}
}

// With nobody free, waiters on different targets must not all wait for ever.
func TestWaitersOnDifferentTargetsHelpEachOther(t *testing.T) {
	m := &Map{size: 10}
	robots := []*Robot{
		{id: 1, row: 2, col: 2, facing: "N", onTarget: true},
		{id: 2, row: 7, col: 7, facing: "N", onTarget: true},
	}
	targets := []*Target{{id: 0, row: 2, col: 2}, {id: 1, row: 7, col: 7}}

	targets, ticks := runUntilDone(t, m, robots, targets, 500)
	// The freed helper may find the other target and finish it too, so 0 or 1 is fine.
	if len(targets) == 2 {
		t.Fatalf("both targets left after %d ticks, want at least one picked up", ticks)
	}
}

// A message with a delay of d is read d iterations later than normal.
func TestMessageDelay(t *testing.T) {
	m := &Map{size: 10, delay: func() int { return 2 }}
	sender := &Robot{id: 1, row: 5, col: 5, facing: "N"}
	receiver := &Robot{id: 2, row: 0, col: 0, facing: "N"}

	sender.send(Message{kind: msgHelp, from: 1, row: 5, col: 5})
	sender.SendMessage(m, []*Robot{sender, receiver})
	if len(sender.inbox) != 0 || len(receiver.inbox) != 1 {
		t.Fatalf("inboxes hold %d and %d messages, want 0 and 1", len(sender.inbox), len(receiver.inbox))
	}

	for i := 0; i < 2; i++ {
		receiver.ReceiveMessage(m)
		if len(receiver.outbox) != 0 {
			t.Fatalf("read the message %d iterations early", 2-i)
		}
	}
	receiver.ReceiveMessage(m)
	if len(receiver.outbox) != 1 || receiver.outbox[0].kind != msgClaim {
		t.Fatalf("outbox = %v, want a CLAIM after the delay", receiver.outbox)
	}
}

// A claim for a request the waiter has moved on from is released, so the helper is not stuck.
func TestLateClaimIsReleased(t *testing.T) {
	m := &Map{size: 10}
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true, seq: 3}

	waiter.handle(m, Message{kind: msgClaim, from: 2, to: 1, seq: 2, dist: 4}) // an earlier request
	if len(waiter.claims) != 0 || len(waiter.outbox) != 1 || waiter.outbox[0].kind != msgRelease {
		t.Fatalf("stale claim: claims=%v outbox=%v, want it released", waiter.claims, waiter.outbox)
	}
}

// The whole protocol, with and without delay, never produces an illegal pickup
// and always finishes the targets.
func TestFullRun(t *testing.T) {
	delays := map[string]func() int{
		"no delay":    nil,
		"exponential": func() int { return int(rand.ExpFloat64() * 3) },
	}
	for name, delay := range delays {
		for run := 0; run < 20; run++ {
			m := &Map{size: 10, delay: delay}
			robots, targets := RandomPlacement(10, 6, 4)
			targets, ticks := runUntilDone(t, m, robots, targets, 20000)
			if len(targets) != 0 {
				t.Fatalf("%s, run %d: %d targets left after %d ticks", name, run, len(targets), ticks)
			}
		}
	}
}

// A new set of targets appears when none are left, on cells without robots.
func TestTickRespawnsTargets(t *testing.T) {
	m := &Map{size: 10, respawn: 3, nextTarget: 3}
	robots, _ := RandomPlacement(10, 5, 0)
	before := map[pos]bool{} // robots may step onto a new target during the tick itself
	for _, r := range robots {
		before[pos{r.row, r.col}] = true
	}

	verdict, targets := Tick(m, robots, nil)
	if len(targets) != 3 || !strings.Contains(verdict, "spawned 3 new targets") {
		t.Fatalf("got %d targets and verdict %q, want 3 new targets", len(targets), verdict)
	}
	seen := map[pos]bool{}
	for i, tg := range targets {
		cell := pos{tg.row, tg.col}
		if seen[cell] || tg.id != 3+i {
			t.Errorf("target %d at %v: shares a cell or has the wrong ID", tg.id, cell)
		}
		seen[cell] = true
		if before[cell] {
			t.Errorf("%s spawned on top of a robot", tg.label())
		}
	}

	// While targets remain, nothing is added.
	if _, again := Tick(m, robots, targets); len(again) != 3 {
		t.Errorf("%d targets after a normal tick, want 3", len(again))
	}
}

// A run keeps going: every time the targets are collected, a new set appears.
func TestRespawnKeepsTheRunGoing(t *testing.T) {
	m := &Map{size: 10, respawn: 2, nextTarget: 2}
	robots, targets := RandomPlacement(10, 6, 2)
	for m.turn < 5000 && m.nextTarget < 8 {
		var verdict string
		verdict, targets = Tick(m, robots, targets)
		if illegal(verdict) {
			t.Fatalf("illegal behaviour: %s", verdict)
		}
	}
	if m.nextTarget < 8 {
		t.Fatalf("only %d targets ever existed after %d ticks, want the sets to be replaced", m.nextTarget, m.turn)
	}
}

// Changing only the iterations continues the board; changing a count restarts it.
func TestApplyContinuesTheBoard(t *testing.T) {
	g := NewGame()
	g.panel.values = [fieldCount]string{"4", "3", "10"}
	g.apply()
	robots := g.robots
	if len(robots) != 4 || len(g.targets) != 3 || !g.running {
		t.Fatalf("first apply: %d robots, %d targets, running=%v", len(robots), len(g.targets), g.running)
	}

	g.m.turn = 10 // the run used up its iterations
	g.running = false
	g.panel.values[fieldIterations] = "25"
	g.apply()
	if g.robots[0] != robots[0] || g.m.turn != 10 || g.iterations != 25 || !g.running {
		t.Errorf("more iterations: turn=%d iterations=%d running=%v, want the same board continuing", g.m.turn, g.iterations, g.running)
	}

	g.panel.values[fieldIterations] = "5" // fewer than already done
	g.apply()
	if g.running || g.m.turn != 10 {
		t.Errorf("iterations below the current turn: running=%v turn=%d, want it stopped", g.running, g.m.turn)
	}

	g.panel.values[fieldRobots] = "5"
	g.apply()
	if g.m.turn != 0 || len(g.robots) != 5 {
		t.Errorf("new robot count: turn=%d robots=%d, want a fresh board", g.m.turn, len(g.robots))
	}
}

// With no delay, every receiver gets at most one packet per sender per turn.
func TestOnePacketPerTurn(t *testing.T) {
	m := &Map{size: 10, respawn: 5, nextTarget: 5}
	robots, targets := RandomPlacement(10, 10, 5)
	multi := 0
	for m.turn < 2000 {
		_, targets = Tick(m, robots, targets)
		for _, r := range robots {
			from := map[int]bool{}
			for _, d := range r.inbox {
				if from[d.msgs[0].from] {
					t.Fatalf("turn %d: r%d got two packets from r%d", m.turn, r.id, d.msgs[0].from)
				}
				from[d.msgs[0].from] = true
				if len(d.msgs) > 1 {
					multi++
				}
			}
		}
	}
	if multi == 0 {
		t.Error("no packet ever held more than one message; the test is not exercising bundling")
	}
}

// Any pickup that is not exactly two robots on a target counts as illegal and
// makes the run stop; a correct pair, or no pickups at all, does not.
func TestPickupReportHasIllegal(t *testing.T) {
	targets := []*Target{{id: 0, row: 1, col: 1}}
	pick := func(cells ...pos) []PickupAction {
		var as []PickupAction
		for i, c := range cells {
			as = append(as, PickupAction{i + 1, c.row, c.col})
		}
		return as
	}
	here, elsewhere := pos{1, 1}, pos{2, 2}

	for name, tc := range map[string]struct {
		actions []PickupAction
		want    bool
	}{
		"none":         {nil, false},
		"legal pair":   {pick(here, here), false},
		"no target":    {pick(elsewhere), true},
		"understaffed": {pick(here), true},
		"overstaffed":  {pick(here, here, here), true},
	} {
		if got := CheckPickupActions(targets, tc.actions).hasIllegal(); got != tc.want {
			t.Errorf("%s: hasIllegal = %v, want %v", name, got, tc.want)
		}
	}
}

// A tick with nothing illegal leaves the flag off.
func TestTickIllegalFlagOffWhenLegal(t *testing.T) {
	m := &Map{size: 10, illegal: true}
	r := &Robot{id: 1, row: 5, col: 4, facing: "E"}
	Tick(m, []*Robot{r}, []*Target{{id: 0, row: 5, col: 5}})
	if m.illegal {
		t.Error("illegal still set after a legal tick")
	}
}
