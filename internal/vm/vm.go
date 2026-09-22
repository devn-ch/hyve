package vm

type State string
type NetworkMode string

const (
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateExited   State = "exited"

	NetworkNAT    NetworkMode = "nat"
	NetworkTAP    NetworkMode = "tap"
	NetworkBridge NetworkMode = "bridge"
	NetworkNone   NetworkMode = "none"
)

type VM struct {
	Name   string
	State  State
	CPUs   int
	Memory string
}

type NetworkConfig struct {
	Mode      NetworkMode `json:"mode,omitempty"`
	Interface string      `json:"interface,omitempty"`
	MAC       string      `json:"mac,omitempty"`
}
