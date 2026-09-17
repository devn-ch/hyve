package vm

type State string

const (
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateExited   State = "exited"
)

type VM struct {
	Name   string
	State  State
	CPUs   int
	Memory string
}