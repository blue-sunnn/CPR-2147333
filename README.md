# CPR: Cyber Physical Robotics project

A simulation of robots on a 10x10 grid. Robots move, turn, and pick up targets,
and a target can only be picked up by exactly two robots at the same time.
It is written in Go, and the window uses [Ebitengine](https://ebitengine.org).

## What the project asks for

### Implementation

A simulation must be developed in any programming language of choice. The
simulation must be of a 2D grid (10x10), where a configurable number of robots
move according to rules. *Targets* in the simulation must show up randomly, and
persist until they are captured by robots. Each robot has a unique ID.

#### Robot operation

At each turn, a robot can perform one of these actions:

- Move forward one square, in the direction it is facing
- Turn left (rotate 90°counter-clockwise)
- Turn right (rotate 90°clockwise)
- Pick up target from its current position

At each turn, a robot can sense:

- If there is a target in the position directly in front
- If there are robots in the position directly in front (and what their IDs are)
- If there are robots in the same position it currently occupies (and what their IDs are)

At each turn, a robot can broadcast a message (meaning, all other robots
receive it). It is up to you what the format and payload of that message is.

#### Simulation behavior

The simulation *must* obey the following procedure:

```text
while (number iterations < max-iterations)
    if (no more targets)
        generate random targets
    for all robots
        read input messages
    for all robots
        sense environment
        decide on action to perform
    Test for illegal behavior here
    for all robots
        perform action
    for all robots
        send output messages
```

The simulation must test for two illegal conditions:

- At least one robot attempts to pick up a target from a position where there is no target
- Exactly one robot attempts to pick up a target from a position where there is a target
- More than two robots attempt to pick up a target from a position where there is a target

**In other words, targets can only be picked up validly by exactly two robots, at exactly the same time**.

#### Communication

Communication must be subject to random delay: specifically, following an
exponential distribution. For each message each robot sends, all other robots
will receive such a message at potentially different times: the random delay is
applied per receiver, not per message. The easiest way to implement this is:

```text
Procedure send-message(msg)
    for all robots
        r = generate-random-number ; exponential distribution
        input-message-queue <- append(msg, r)

Procedure read-messages()
    for all messages in input-message-queue
        if (r == 0)
            read msg
        else
            r = r-1
```

## Milestone

- **Milestone 1** show that all illegal cases are detected
- **Milestone 2** full robot behavior, no delay
- **Milestone 3** communication delay

## Run the simulation

You need Go (see the version in `project/go.mod`).

```bash
cd project
go run .
```

Run the tests:

```bash
cd project
go test ./...
```

## Using the window

The board starts empty. Type a number in each of the three fields on the right
(all three are required), then click **Apply** or press Enter:

| Field | Meaning |
|---|---|
| Robots | How many robots to place, each at a random cell facing a random direction. |
| Targets | How many targets to place at random cells. |
| Iterations | How many ticks the run lasts. |

Click a field to type in it, Tab moves to the next one, Backspace deletes.
Robots and targets together cannot be more than the 100 cells of the grid.

After Apply, the simulation ticks on its own (speed: `tickFrames` in `visualize.go`) until it reaches
the iteration count. The **log** is printed in the terminal: every tick gets a `[N]` header
followed by its events (messages sent, pickups, blocked illegal actions, new
targets); a tick with no events is just the header.

When no run is going, the keyboard steps every robot by hand: Space moves,
Left / Right turn, P picks up.

Robots wander at random until one steps onto a target. It broadcasts a help
request, the closest free robot answers and walks over, and the two pick up
together (`comms.go`). When all targets are gone, a new set appears.

## How the code is organized

All of `project/` is one Go package (`package main`), so files use each other
without imports.

| File | Job |
|---|---|
| `main.go` | Opens the window. The board starts empty; Apply in the panel places robots and targets at random. |
| `simulation.go` | The rules. `Tick` is one iteration: every robot senses and decides, the actions are checked, and only the legal ones are applied. Also checks the starting layout and makes random ones. |
| `robot.go` | The robot: position, facing, which turn it arrived. `Sense` is what it sees and `Decide` picks its action (random for now, the real rules go here). |
| `comms.go` | Robot messages (input/output queues, optional delay) and the help/claim/accept protocol. |
| `target.go`, `map.go` | The target and the grid. |
| `visualize.go` | The window state, the log (printed to the terminal), the settings panel (number fields and Apply) and the keyboard stepper for when no run is going. The picture glides toward the simulation state a little every frame, so the simulation itself can jump instantly. |
| `draw.go` | All drawing: the board, robots, stars, text, colors and sizes. |
| `simulation_test.go` | Tests for the rules, random placement, `Sense` and `Tick`. |
| `go.mod`, `go.sum` | The Go dependencies and their checksums. |

`simulation.go` does not use the window, so the rules can be tested and reused
without it. Only `visualize.go` and `draw.go` use Ebitengine.