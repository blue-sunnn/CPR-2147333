package main

import (
	"math"
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

// A lone robot on a target never picks up on its own: there is nobody to pair
// with, so it waits, gives up after giveUpAfter ticks and the target stays.
func TestLoneWaiterNeverPicksUp(t *testing.T) {
	m := &Map{size: 10}
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true}
	targets := []*Target{{id: 0, row: 5, col: 5}}

	for i := 0; i < giveUpAfter-1; i++ {
		_, targets = Tick(m, []*Robot{waiter}, targets)
	}
	if waiter.row != 5 || waiter.col != 5 || !waiter.onTarget || len(targets) != 1 {
		t.Fatalf("lone waiter moved or left early: at (%d, %d), onTarget=%v, %d targets left",
			waiter.row, waiter.col, waiter.onTarget, len(targets))
	}

	for i := 0; i < 200; i++ {
		var verdict string
		verdict, targets = Tick(m, []*Robot{waiter}, targets)
		if illegal(verdict) {
			t.Fatalf("illegal behaviour: %s", verdict)
		}
	}
	if len(targets) != 1 {
		t.Fatal("a lone robot picked up a target")
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

// A waiter runs a Paxos round, a free robot is chosen and walks over, the pair
// picks up, and afterwards nobody keeps a job. The third robot stays out.
func TestWaiterGetsAPartner(t *testing.T) {
	for name, delay := range map[string]func() int{
		"no delay":    nil,
		"exponential": func() int { return int(rand.ExpFloat64() * 3) },
	} {
		m := &Map{size: 10, delay: delay}
		waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true}
		a := &Robot{id: 2, row: 0, col: 9, facing: "S"}
		b := &Robot{id: 3, row: 5, col: 3, facing: "E"}
		robots := []*Robot{waiter, a, b}
		targets := []*Target{{id: 0, row: 5, col: 5}}

		targets, ticks := runUntilDone(t, m, robots, targets, 500)
		if len(targets) != 0 {
			t.Fatalf("%s: the target was still there after %d ticks", name, ticks)
		}
		for _, r := range robots {
			if r.onTarget || r.partner != 0 {
				t.Errorf("%s: %s still holds a job after the pickup", name, r.label())
			}
			if pr, ok := r.decided[0]; ok && pr.a != waiter.id {
				t.Errorf("%s: %s decided %+v, want the waiter r1 in the pair", name, r.label(), pr)
			}
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
		{id: 2, row: 7, col: 7, facing: "N", onTarget: true, targetID: 1, jobTarget: instance(1, 0)},
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

	sender.send(Message{kind: msgPrepare, from: 1, target: 0, n: 101, row: 5, col: 5})
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
	if len(receiver.outbox) != 1 || receiver.outbox[0].kind != msgPromise {
		t.Fatalf("outbox = %v, want a PROMISE after the delay", receiver.outbox)
	}
}

// outboxKinds lists the kinds of the messages a robot has queued.
func outboxKinds(r *Robot) []string {
	var kinds []string
	for _, msg := range r.outbox {
		kinds = append(kinds, msg.kind)
	}
	return kinds
}

// firstOfKind returns the first queued message of the given kind, or nil.
func firstOfKind(msgs []Message, kind string) *Message {
	for i := range msgs {
		if msgs[i].kind == kind {
			return &msgs[i]
		}
	}
	return nil
}

// An acceptor promises only to a proposal number higher than any it has promised.
func TestAcceptorPromisesOnlyHigherNumbers(t *testing.T) {
	m := &Map{size: 10, robots: 3}
	r := &Robot{id: 2, row: 5, col: 3, facing: "N"}

	for _, n := range []int{101, 101, 5, 102} { // the 2nd and 3rd are not higher
		r.handle(m, Message{kind: msgPrepare, from: 1, target: 0, n: n, row: 5, col: 5})
	}
	if got := outboxKinds(r); !slices.Equal(got, []string{msgPromise, msgPromise}) {
		t.Fatalf("outbox = %v, want exactly two PROMISEs (for 101 and 102)", got)
	}
	if r.outbox[1].n != 102 || !r.outbox[1].free || r.outbox[1].dist != 2 {
		t.Errorf("second PROMISE = %+v, want n 102, free, dist 2", r.outbox[1])
	}
}

// A PROMISE reports a value the acceptor already accepted, so a later proposer can adopt it.
func TestPromiseReportsAcceptedValue(t *testing.T) {
	m := &Map{size: 10, robots: 3}
	r := &Robot{id: 2, row: 5, col: 3, facing: "N"}

	r.handle(m, Message{kind: msgAccept, from: 1, target: 0, n: 101, val: pair{1, 2}, row: 5, col: 5})
	r.outbox = nil
	r.handle(m, Message{kind: msgPrepare, from: 3, target: 0, n: 203, row: 5, col: 5})

	if len(r.outbox) != 1 || r.outbox[0].accN != 101 || r.outbox[0].val != (pair{1, 2}) {
		t.Fatalf("outbox = %+v, want one PROMISE carrying accN 101 and the pair r1+r2", r.outbox)
	}
}

// An acceptor refuses an ACCEPT below its promise, and accepts one at or above it.
func TestAcceptorRefusesBelowPromise(t *testing.T) {
	m := &Map{size: 10, robots: 3}
	r := &Robot{id: 2, row: 5, col: 3, facing: "N"}
	r.handle(m, Message{kind: msgPrepare, from: 3, target: 0, n: 203, row: 5, col: 5})
	r.outbox = nil

	r.handle(m, Message{kind: msgAccept, from: 1, target: 0, n: 101, val: pair{1, 2}})
	if len(r.outbox) != 0 || r.accOf(0).accN != 0 {
		t.Fatalf("accepted n=101 after promising 203: outbox=%v accN=%d", r.outbox, r.accOf(0).accN)
	}
	r.handle(m, Message{kind: msgAccept, from: 3, target: 0, n: 203, val: pair{3, 2}})
	if got := outboxKinds(r); !slices.Equal(got, []string{msgAccepted}) || r.accOf(0).accN != 203 {
		t.Fatalf("outbox = %v accN = %d, want ACCEPTED for n=203", got, r.accOf(0).accN)
	}
}

// A helper that already works on one target is never named for another.
func TestBusyHelperRefusesSecondTarget(t *testing.T) {
	m := &Map{size: 10, robots: 3}
	r := &Robot{id: 2, row: 5, col: 3, facing: "N", partner: 1, jobTarget: 0}

	r.handle(m, Message{kind: msgAccept, from: 3, target: 1, n: 103, val: pair{3, 2}})
	if len(r.outbox) != 0 || r.accOf(1).accN != 0 {
		t.Fatalf("a busy helper accepted a second target: outbox=%v", r.outbox)
	}
}

// The proposer must adopt a value an acceptor already accepted, not its own choice.
func TestProposerAdoptsAcceptedValue(t *testing.T) {
	m := &Map{size: 10, robots: 3}
	r3 := &Robot{id: 3, row: 5, col: 5, facing: "N", onTarget: true, propN: 203}

	// r1 is free and close, but r2 already accepted the pair r1+r2 with n=101.
	r3.handle(m, Message{kind: msgPromise, from: 1, to: 3, target: 0, n: 203, free: true, dist: 1})
	r3.handle(m, Message{kind: msgPromise, from: 2, to: 3, target: 0, n: 203, accN: 101, val: pair{1, 2}})

	if acc := firstOfKind(r3.outbox, msgAccept); acc == nil || acc.val != (pair{1, 2}) {
		t.Fatalf("outbox = %+v, want an ACCEPT for the old pair r1+r2", r3.outbox)
	}
}

// With no old value, the proposer picks the closest free promiser (never itself).
func TestProposerPicksClosestFreeRobot(t *testing.T) {
	m := &Map{size: 10, robots: 5}
	r1 := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true, propN: 101}

	for _, p := range []Message{
		{from: 2, dist: 6, free: true},
		{from: 3, dist: 2, free: false}, // busy, even though closest
		{from: 4, dist: 3, free: true},
	} {
		p.kind, p.to, p.target, p.n = msgPromise, 1, 0, 101
		r1.handle(m, p)
	}
	if acc := firstOfKind(r1.outbox, msgAccept); acc == nil || acc.val != (pair{1, 4}) {
		t.Fatalf("outbox = %+v, want an ACCEPT for r1+r4", r1.outbox)
	}
}

// A late PROMISE from an old round is ignored.
func TestStalePromiseIsIgnored(t *testing.T) {
	m := &Map{size: 10, robots: 3}
	r1 := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true, propN: 201}

	r1.handle(m, Message{kind: msgPromise, from: 2, to: 1, target: 0, n: 101, free: true, dist: 1})
	r1.handle(m, Message{kind: msgPromise, from: 3, to: 1, target: 0, n: 101, free: true, dist: 1})
	if len(r1.outbox) != 0 || len(r1.promises) != 0 {
		t.Fatalf("a promise for n=101 was used in round 201: outbox=%v", r1.outbox)
	}
}

// A value is decided only after a majority of matching ACCEPTED messages. Then
// the waiter of the pair takes its partner and sends GO, and a waiter that was
// left out leaves. The helper does nothing until it gets GO.
func TestLearnerDecidesOnMajority(t *testing.T) {
	m := &Map{size: 10, robots: 3}
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true}
	helper := &Robot{id: 2, row: 5, col: 2, facing: "N"}
	loser := &Robot{id: 3, row: 5, col: 5, facing: "N", onTarget: true}
	robots := []*Robot{waiter, helper, loser}

	accepted := func(from int) Message {
		return Message{kind: msgAccepted, from: from, target: 0, n: 101, val: pair{1, 2}, row: 5, col: 5}
	}
	for _, r := range robots {
		r.handle(m, accepted(1))
		if _, done := r.decided[0]; done {
			t.Fatalf("%s decided after a single ACCEPTED", r.label())
		}
	}
	for _, r := range robots {
		r.handle(m, accepted(2))
		if r.decided[0] != (pair{1, 2}) {
			t.Fatalf("%s did not decide r1+r2 after a majority: %+v", r.label(), r.decided)
		}
	}

	if waiter.partner != 2 || !waiter.onTarget {
		t.Errorf("waiter: partner=%d onTarget=%v, want partner r2 and still waiting", waiter.partner, waiter.onTarget)
	}
	if go1 := firstOfKind(waiter.outbox, msgGo); go1 == nil || go1.to != 2 {
		t.Errorf("waiter outbox = %v, want a GO to r2", waiter.outbox)
	}
	if helper.partner != 0 {
		t.Errorf("helper has partner r%d before any GO", helper.partner)
	}
	if loser.onTarget || loser.partner != 0 {
		t.Errorf("loser: onTarget=%v partner=%d, want it to have left", loser.onTarget, loser.partner)
	}

	// The hand-over: GO, then ACK back, then READY.
	helper.handle(m, *firstOfKind(waiter.outbox, msgGo))
	if helper.partner != 1 || helper.goalRow != 5 || helper.goalCol != 5 || helper.ready {
		t.Errorf("helper after GO: partner=%d goal=(%d,%d) ready=%v, want partner r1, goal (5,5), not ready yet",
			helper.partner, helper.goalRow, helper.goalCol, helper.ready)
	}
	if got := helper.Decide(m, Sensing{turn: 2}); got == actionPickup {
		t.Errorf("helper picked up before READY")
	}
	ack := firstOfKind(helper.outbox, msgAck)
	if ack == nil {
		t.Fatalf("helper outbox = %v, want an ACK", helper.outbox)
	}

	waiter.handle(m, *ack)
	ready := firstOfKind(waiter.outbox, msgReady)
	if !waiter.acked || ready == nil {
		t.Fatalf("waiter after ACK: acked=%v outbox=%v, want acked and a READY", waiter.acked, waiter.outbox)
	}
	helper.handle(m, *ready)
	if !helper.ready {
		t.Error("helper not ready after READY")
	}
	if got := helper.Decide(m, Sensing{turn: 3}); got == actionStay {
		t.Error("helper still waiting after READY, want it to walk")
	}
}

// A GO only starts a helper that is free and named in it; a stale one is ignored.
func TestGoOnlyStartsAFreeNamedHelper(t *testing.T) {
	m := &Map{size: 10, robots: 3, turn: 5}
	goMsg := Message{kind: msgGo, from: 1, to: 2, target: 0, val: pair{1, 2}, row: 5, col: 5}

	busy := &Robot{id: 2, row: 1, col: 1, facing: "N", onTarget: true}
	busy.handle(m, goMsg)
	if busy.partner != 0 {
		t.Error("a robot waiting on another target started on a GO")
	}

	stale := goMsg        // instance(0, 5): epoch 0
	m.turn = epochLen + 3 // but we are in epoch 1 now
	idle := &Robot{id: 2, row: 1, col: 1, facing: "N"}
	idle.handle(m, stale)
	if idle.partner != 0 {
		t.Error("a GO from an earlier epoch started a helper")
	}
}

// A helper that accepted a value naming it stays free for that instance only:
// it refuses to be named for a second one at the same time.
func TestHelperIsReservedForOneInstance(t *testing.T) {
	m := &Map{size: 10, robots: 3, turn: 5}
	r := &Robot{id: 2, row: 5, col: 3, facing: "N"}

	r.handle(m, Message{kind: msgAccept, from: 1, target: instance(0, 5), n: 101, val: pair{1, 2}})
	if r.accOf(instance(0, 5)).accN != 101 || !r.reserved(m.turn) {
		t.Fatalf("helper did not accept and reserve itself: accN=%d reserved=%v", r.accOf(instance(0, 5)).accN, r.reserved(m.turn))
	}
	r.outbox = nil
	r.handle(m, Message{kind: msgAccept, from: 5, target: instance(1, 5), n: 105, val: pair{5, 2}})
	if len(r.outbox) != 0 {
		t.Fatalf("a reserved helper accepted a second pair: %v", r.outbox)
	}
	m.turn += reserveFor // the reservation ran out
	r.handle(m, Message{kind: msgAccept, from: 5, target: instance(1, m.turn), n: 105, val: pair{5, 2}})
	if len(r.outbox) == 0 {
		t.Fatal("the helper stayed reserved after the reservation ran out")
	}
}

// A pair that has not picked up by the end of its epoch is dropped: the waiter
// starts a fresh Paxos instance instead of waiting for ever.
func TestPairIsDroppedAtEpochEnd(t *testing.T) {
	m := &Map{size: 10, robots: 3, turn: epochLen - 2}
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true, partner: 2, acked: true, propAt: m.turn, waitingSince: m.turn}

	for i := 0; i < 4; i++ {
		m.turn++
		waiter.Decide(m, Sensing{turn: m.turn})
	}
	if waiter.partner != 0 || waiter.jobTarget != instance(0, epochLen) {
		t.Fatalf("partner=%d instance=%d, want the old pair dropped and the epoch-1 instance", waiter.partner, waiter.jobTarget)
	}
	if firstOfKind(waiter.outbox, msgPrepare) == nil {
		t.Error("the waiter did not start a new round after the epoch changed")
	}
}

// A waiter only picks up once its helper has committed (ACK). A robot that
// merely walks through the cell is not its partner, even if the IDs match.
func TestWaiterPicksUpOnlyAfterAck(t *testing.T) {
	m := &Map{size: 10, robots: 3, turn: 1}
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true, partner: 2}

	if got := waiter.Decide(m, Sensing{turn: 1, robotsHere: []int{2}}); got == actionPickup {
		t.Fatal("picked up with a partner that never sent ACK")
	}
	waiter.handle(m, Message{kind: msgAck, from: 2, to: 1, target: 0, val: pair{1, 2}})
	if got := waiter.Decide(m, Sensing{turn: 2, robotsHere: []int{2}}); got != actionPickup {
		t.Fatalf("got %s, want PICKUP once the partner has committed", got)
	}
	if got := waiter.Decide(m, Sensing{turn: 3, robotsHere: []int{2, 3}}); got == actionPickup {
		t.Error("picked up with a third robot in the cell")
	}
}

// A helper takes a GO once: a second waiter that names it too is ignored.
func TestHelperCommitsToOneWaiter(t *testing.T) {
	m := &Map{size: 10, robots: 3, turn: 5}
	helper := &Robot{id: 3, row: 1, col: 1, facing: "N"}

	helper.handle(m, Message{kind: msgGo, from: 1, to: 3, target: instance(0, 5), val: pair{1, 3}, row: 5, col: 5})
	helper.outbox = nil
	helper.handle(m, Message{kind: msgGo, from: 2, to: 3, target: instance(1, 5), val: pair{2, 3}, row: 7, col: 7})
	if helper.partner != 1 || len(helper.outbox) != 0 {
		t.Fatalf("partner=%d outbox=%v, want it to stay with r1 and send no second ACK", helper.partner, helper.outbox)
	}
}

// A helper that starts in the waiter's cell steps out first and sends its ACK
// only once outside, so the waiter can never see it before it has committed.
func TestHelperInTheWaitersCellStepsOutBeforeAck(t *testing.T) {
	m := &Map{size: 10, robots: 3, turn: 5}
	helper := &Robot{id: 3, row: 5, col: 5, facing: "E"}
	helper.handle(m, Message{kind: msgGo, from: 1, to: 3, target: instance(0, 5), val: pair{1, 3}, row: 5, col: 5})

	if got := helper.Decide(m, Sensing{turn: 5}); got != actionMove {
		t.Fatalf("got %s, want MOVE out of the waiter's cell", got)
	}
	if firstOfKind(helper.outbox, msgAck) != nil {
		t.Fatal("ACK sent while still in the waiter's cell")
	}
	helper.col++ // it moved out
	helper.Decide(m, Sensing{turn: 6})
	if firstOfKind(helper.outbox, msgAck) == nil {
		t.Fatal("no ACK once outside the waiter's cell")
	}
}

// A committed helper walks straight away, but stops at the edge of the waiter's
// cell until READY comes.
func TestHelperWalksButWaitsAtTheDoor(t *testing.T) {
	m := &Map{size: 10, robots: 3, turn: 5}
	helper := &Robot{id: 3, row: 5, col: 2, facing: "E", partner: 1, goalRow: 5, goalCol: 5, jobTarget: instance(0, 5)}

	for i := 0; i < 2; i++ { // (5,2) -> (5,4)
		if got := helper.Decide(m, Sensing{turn: 5}); got != actionMove {
			t.Fatalf("step %d: got %s, want MOVE", i, got)
		}
		helper.col++
	}
	if got := helper.Decide(m, Sensing{turn: 5}); got != actionStay {
		t.Fatalf("at the door: got %s, want STAY until READY", got)
	}
	helper.ready = true
	if got := helper.Decide(m, Sensing{turn: 5}); got != actionMove {
		t.Fatalf("after READY: got %s, want MOVE into the cell", got)
	}
}

// Two waiters in one cell: only the lower ID proposes (it can recruit the other,
// and the higher one's proposal numbers would otherwise always outrank it).
func TestOnlyTheLowerIDInACellProposes(t *testing.T) {
	m := &Map{size: 10, robots: 4, turn: 3}
	low := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true}
	high := &Robot{id: 2, row: 5, col: 5, facing: "N", onTarget: true}

	low.Decide(m, Sensing{turn: 3, robotsHere: []int{2}})
	high.Decide(m, Sensing{turn: 3, robotsHere: []int{1}})
	if firstOfKind(low.outbox, msgPrepare) == nil {
		t.Error("the lower ID did not propose")
	}
	if firstOfKind(high.outbox, msgPrepare) != nil {
		t.Error("the higher ID proposed while a lower ID shared its cell")
	}
}

// A waiter whose partner never commits (it went to another waiter) frees itself.
func TestWaiterGivesUpOnAPartnerThatNeverAcks(t *testing.T) {
	m := &Map{size: 10, robots: 3}
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true, partner: 3, propAt: 1, waitingSince: 1}

	waiter.Decide(m, Sensing{turn: 1 + giveUpAfter - 1})
	if !waiter.onTarget {
		t.Fatal("gave up too early")
	}
	waiter.Decide(m, Sensing{turn: 1 + giveUpAfter})
	if waiter.onTarget || waiter.partner != 0 {
		t.Errorf("onTarget=%v partner=%d, want it freed after giveUpAfter ticks", waiter.onTarget, waiter.partner)
	}
}

// A waiter that has waited a while offers itself as helper to a lower ID (and
// only a lower ID); a waiter in the proposer's own cell offers at once.
func TestWaitingRobotHelpsALowerID(t *testing.T) {
	r := &Robot{id: 3, row: 2, col: 2, facing: "N", onTarget: true, waitingSince: 1}
	turn := 1 + helpAfter

	if !r.willingToHelp(turn, 1, 5, 5) {
		t.Error("a waiter that waited long enough refused to help a lower ID")
	}
	if r.willingToHelp(turn-1, 1, 5, 5) {
		t.Error("offered to help before helpAfter ticks")
	}
	if !r.willingToHelp(1, 1, 2, 2) {
		t.Error("a waiter in the proposer's cell did not offer at once")
	}
	if r.willingToHelp(turn, 4, 5, 5) {
		t.Error("offered to help a higher ID")
	}
	r.partner = 1
	if r.willingToHelp(turn, 1, 5, 5) {
		t.Error("offered to help while it already has a partner")
	}
}

// Targets everywhere: every robot becomes a waiter on its own target, so nobody
// is free. The waiting robots must help each other, or nothing is ever collected.
func TestTargetsEverywhere(t *testing.T) {
	for name, delay := range map[string]func() int{
		"no delay":    nil,
		"exponential": func() int { return int(rand.ExpFloat64() * 3) },
	} {
		for run := 0; run < 10; run++ {
			m := &Map{size: 10, delay: delay}
			robots, targets := RandomPlacement(10, 4, 96)
			start := len(targets)
			for m.turn < 3000 && len(targets) > start-10 {
				var verdict string
				verdict, targets = Tick(m, robots, targets)
				if illegal(verdict) {
					t.Fatalf("%s, run %d: illegal behaviour: %s", name, run, verdict)
				}
			}
			if got := start - len(targets); got < 10 {
				t.Fatalf("%s, run %d: only %d targets collected in %d ticks", name, run, got, m.turn)
			}
		}
	}
}

// Two proposers on the same target (same cell, same moment): every robot ends
// up with the same pair, and nothing illegal happens.
func TestTwoProposersAgree(t *testing.T) {
	for name, delay := range map[string]func() int{
		"no delay":    nil,
		"exponential": func() int { return int(rand.ExpFloat64() * 3) },
	} {
		for run := 0; run < 50; run++ {
			m := &Map{size: 10, delay: delay}
			robots := []*Robot{
				{id: 1, row: 5, col: 4, facing: "E"},
				{id: 2, row: 5, col: 6, facing: "W"},
				{id: 3, row: 2, col: 5, facing: "S"},
				{id: 4, row: 8, col: 5, facing: "N"},
				{id: 5, row: 0, col: 0, facing: "S"},
			}
			targets, ticks := runUntilDone(t, m, robots, []*Target{{id: 0, row: 5, col: 5}}, 2000)
			if len(targets) != 0 {
				t.Fatalf("%s, run %d: target left after %d ticks", name, run, ticks)
			}
			var first pair
			for _, r := range robots {
				if pr, ok := r.decided[0]; ok {
					if first == (pair{}) {
						first = pr
					} else if pr != first {
						t.Fatalf("%s, run %d: robots decided different pairs %+v and %+v", name, run, first, pr)
					}
				}
			}
		}
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

// A lone wandering robot visits every cell, without ever leaving the grid.
func TestSweepVisitsEveryCell(t *testing.T) {
	for _, facing := range []string{"N", "E", "S", "W"} {
		m := &Map{size: 10}
		r := &Robot{id: 1, row: 4, col: 6, facing: facing}
		seen := map[pos]bool{}
		for i := 0; i < 3*N*N; i++ {
			Tick(m, []*Robot{r}, nil)
			if m.illegal {
				t.Fatalf("facing %s: tried to leave the grid at (%d, %d)", facing, r.row, r.col)
			}
			seen[pos{r.row, r.col}] = true
		}
		if len(seen) != N*N {
			t.Errorf("facing %s: visited %d of %d cells", facing, len(seen), N*N)
		}
	}
}

// A stale sweep stage (the sweep was interrupted by a target or job) must not walk into the wall.
func TestStaleSweepStageStaysInGrid(t *testing.T) {
	m := &Map{size: 10}
	r := &Robot{id: 1, row: 0, col: 5, facing: "N", sweepStage: 1}
	Tick(m, []*Robot{r}, nil)
	if m.illegal {
		t.Fatal("stale sweep stage moved the robot out of the grid")
	}
}

// ------------------------------------------------- communication delay edge cases

// delayShapes are delay functions that stress the protocol: tiny, epoch-long,
// bimodal (most messages fast, a few cross an epoch), uniform, heavy-tailed.
func delayShapes() map[string]func() int {
	return map[string]func() int{
		"exp(3)":     ExponentialDelay(3),
		"exp(8)":     ExponentialDelay(8),
		"const 1":    func() int { return 1 },
		"const 99":   func() int { return epochLen - 1 },
		"uniform 60": func() int { return rand.IntN(61) },
		"bimodal": func() int {
			if rand.Float64() < 0.85 {
				return 0
			}
			return epochLen + rand.IntN(150)
		},
		"heavy tail": func() int { return min(5000, int(math.Pow(1-rand.Float64(), -1/1.2)-1)) },
	}
}

// However late or reordered the messages are, the robots may stall but must
// never attempt an illegal pickup. (Safety does not depend on timing.)
func TestNoIllegalPickupUnderAnyDelay(t *testing.T) {
	for name, delay := range delayShapes() {
		for _, c := range []struct{ robots, targets int }{{2, 1}, {3, 3}, {5, 2}, {12, 20}, {4, 90}} {
			for run := 0; run < 5; run++ {
				m := &Map{size: 10, delay: delay, respawn: c.targets, nextTarget: c.targets}
				robots, targets := RandomPlacement(10, c.robots, c.targets)
				for m.turn < 2500 {
					var verdict string
					verdict, targets = Tick(m, robots, targets)
					if illegal(verdict) {
						t.Fatalf("%s, %d robots, %d targets, run %d: illegal behaviour: %s", name, c.robots, c.targets, run, verdict)
					}
				}
			}
		}
	}
}

// A negative delay must not lose the message: it is read as if there were no delay.
// (Today wait never reaches 0, so the packet stays in the inbox for ever.)
func TestNegativeDelayStillDelivers(t *testing.T) {
	m := &Map{size: 10, delay: func() int { return -3 }}
	sender := &Robot{id: 1, row: 5, col: 5, facing: "N"}
	receiver := &Robot{id: 2, row: 0, col: 0, facing: "N"}

	sender.send(Message{kind: msgPrepare, from: 1, target: 0, n: 101, row: 5, col: 5})
	sender.SendMessage(m, []*Robot{sender, receiver})
	for i := 0; i < 3; i++ {
		receiver.ReceiveMessage(m)
	}
	if len(receiver.inbox) != 0 {
		t.Fatalf("%d packets still in the inbox after 3 iterations, want it delivered", len(receiver.inbox))
	}
}

// A waiter that gives up on a helper that never ACKed tells it, so the helper is
// not left standing at the door for the rest of the epoch.
func TestHelperIsFreedWhenTheWaiterGivesUp(t *testing.T) {
	m := &Map{size: 10, robots: 3, turn: 5}
	inst := instance(0, 5)
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true, partner: 3, jobTarget: inst, propAt: 5, waitingSince: 5}
	helper := &Robot{id: 3, row: 5, col: 4, facing: "E", partner: 1, goalRow: 5, goalCol: 5, jobTarget: inst, ackSent: true}

	m.turn = 5 + giveUpAfter
	waiter.Decide(m, Sensing{turn: m.turn})
	cancel := firstOfKind(waiter.outbox, msgCancel)
	if waiter.onTarget || cancel == nil || cancel.to != 3 {
		t.Fatalf("onTarget=%v outbox=%v, want the waiter to leave and CANCEL r3", waiter.onTarget, waiter.outbox)
	}
	helper.handle(m, *cancel)
	if helper.partner != 0 || helper.jobTarget != 0 {
		t.Fatalf("partner=%d job=%d, want the helper free after CANCEL", helper.partner, helper.jobTarget)
	}
}

// A CANCEL only frees a helper from the waiter and instance it is committed to,
// and never one that already has READY (it may be stepping into the cell).
func TestCancelOnlyFreesTheRightHelper(t *testing.T) {
	m := &Map{size: 10, robots: 3, turn: 5}
	inst := instance(0, 5)
	for name, c := range map[string]struct {
		cancel Message
		ready  bool
	}{
		"other waiter":   {Message{kind: msgCancel, from: 2, to: 3, target: inst}, false},
		"other instance": {Message{kind: msgCancel, from: 1, to: 3, target: instance(1, 5)}, false},
		"already READY":  {Message{kind: msgCancel, from: 1, to: 3, target: inst}, true},
	} {
		helper := &Robot{id: 3, row: 5, col: 4, facing: "E", partner: 1, goalRow: 5, goalCol: 5, jobTarget: inst, ackSent: true, ready: c.ready}
		helper.handle(m, c.cancel)
		if helper.partner != 1 {
			t.Errorf("%s: the helper let go of its waiter", name)
		}
	}
}

// A slow network is allowed to be slow, not to stop the robots altogether.
func TestSlowDelayStillMakesProgress(t *testing.T) {
	collected := 0
	for run := 0; run < 10; run++ {
		m := &Map{size: 10, delay: ExponentialDelay(20), respawn: 1, nextTarget: 1}
		robots, targets := RandomPlacement(10, 4, 1)
		for m.turn < 10000 {
			before := len(targets)
			var verdict string
			verdict, targets = Tick(m, robots, targets)
			if illegal(verdict) {
				t.Fatalf("illegal behaviour: %s", verdict)
			}
			if len(targets) < before {
				collected++
			}
		}
	}
	if collected < 30 {
		t.Fatalf("only %d targets collected in 10 runs of 10000 iterations with a mean delay of 20", collected)
	}
}

// A long run keeps what each robot remembers about old targets within bounds.
func TestOldInstancesAreForgotten(t *testing.T) {
	m := &Map{size: 10, delay: ExponentialDelay(3), respawn: 3, nextTarget: 3}
	robots, targets := RandomPlacement(10, 6, 3)
	for m.turn < 20000 {
		_, targets = Tick(m, robots, targets)
	}
	for _, r := range robots {
		if n := len(r.acc) + len(r.decided) + len(r.tally); n > 300 {
			t.Fatalf("%s keeps %d old Paxos records after %d iterations, want old epochs dropped", r.label(), n, m.turn)
		}
	}
}

// ------------------------------------------------------ adaptive timeouts

// Timeouts keep their sized-for values on a fast network and stretch on a slow one.
func TestTimeoutsStretchWithObservedDelay(t *testing.T) {
	r := &Robot{id: 1}
	if r.ticks(giveUpAfter) != giveUpAfter || r.level() != 0 {
		t.Fatalf("fresh robot: giveUpAfter=%d level=%d, want the base values", r.ticks(giveUpAfter), r.level())
	}
	for i := 0; i < 20; i++ {
		r.observe(0)
	}
	if r.ticks(giveUpAfter) != giveUpAfter {
		t.Errorf("a fast network stretched giveUpAfter to %d", r.ticks(giveUpAfter))
	}
	for i := 0; i < 500; i++ {
		r.observe(30)
	}
	if got := r.ticks(giveUpAfter); got < 5*giveUpAfter {
		t.Errorf("giveUpAfter = %d after a mean delay of ~30, want it stretched at least 5x", got)
	}
	if r.level() < 2 {
		t.Errorf("level = %d after a mean delay of ~30, want longer epochs", r.level())
	}
}

// A robot learns the delay from the packets it receives, by how much later than
// the earliest possible iteration each one is read.
func TestRobotLearnsDelayFromPackets(t *testing.T) {
	m := &Map{size: 10, delay: func() int { return 10 }}
	sender := &Robot{id: 1}
	receiver := &Robot{id: 2}
	for i := 0; i < 200; i++ {
		m.turn++
		sender.send(Message{kind: msgPrepare, from: 1, n: 101})
		sender.SendMessage(m, []*Robot{sender, receiver})
		receiver.ReceiveMessage(m)
	}
	if receiver.meanDelay < 8 || receiver.meanDelay > 12 {
		t.Fatalf("meanDelay = %.1f, want about 10", receiver.meanDelay)
	}
}

// The end of an epoch is part of the instance number, so a waiter and a helper
// that estimate the delay differently still reset on the very same iteration.
func TestEpochEndComesFromTheInstance(t *testing.T) {
	inst := instanceAt(0, 250, 2) // level 2: epochs of 400, this one is [0, 400)
	if got := epochEnd(inst); got != 4*epochLen {
		t.Fatalf("epochEnd = %d, want %d", got, 4*epochLen)
	}

	m := &Map{size: 10, robots: 3, turn: 250}
	helper := &Robot{id: 3, row: 5, col: 4, facing: "E", partner: 1, goalRow: 5, goalCol: 5, jobTarget: inst, ackSent: true, ready: true}
	waiter := &Robot{id: 1, row: 5, col: 5, facing: "N", onTarget: true, partner: 3, acked: true, jobTarget: inst, propAt: 250, waitingSince: 250}
	for m.turn = 250; m.turn < 4*epochLen; m.turn++ {
		helper.Decide(m, Sensing{turn: m.turn})
		waiter.Decide(m, Sensing{turn: m.turn})
		if helper.partner == 0 || waiter.partner == 0 {
			t.Fatalf("pair dropped at iteration %d, before its epoch ended at %d", m.turn, 4*epochLen)
		}
	}
	m.turn = 4 * epochLen
	helper.Decide(m, Sensing{turn: m.turn})
	waiter.Decide(m, Sensing{turn: m.turn})
	if helper.partner != 0 || waiter.partner != 0 {
		t.Fatalf("partners %d and %d at the end of the epoch, want both dropped on the same iteration", helper.partner, waiter.partner)
	}
}

// A GO from an epoch that is over is ignored, whatever level it was sent at.
func TestGoFromAnOverEpochIsIgnored(t *testing.T) {
	m := &Map{size: 10, robots: 3, turn: 250}
	helper := &Robot{id: 3, row: 1, col: 1, facing: "N"}
	inst := instanceAt(0, 250, 1) // level 1: epoch [200, 400)
	m.turn = 400
	helper.handle(m, Message{kind: msgGo, from: 1, to: 3, target: inst, val: pair{1, 3}, row: 5, col: 5})
	if helper.partner != 0 {
		t.Fatal("helper committed to a GO from a finished epoch")
	}
	m.turn = 399
	helper.handle(m, Message{kind: msgGo, from: 1, to: 3, target: inst, val: pair{1, 3}, row: 5, col: 5})
	if helper.partner != 1 {
		t.Fatal("helper ignored a GO from the epoch still running")
	}
}
