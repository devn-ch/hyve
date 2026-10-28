package network

import "fmt"

type Mode string

const (
	ModeNAT    Mode = "nat"
	ModeTAP    Mode = "tap"
	ModeBridge Mode = "bridge"
	ModeNone   Mode = "none"
)

type Config struct {
	Mode      Mode
	Interface string // existing TAP on ModeTAP
	Bridge    string // Bridge-Name on ModeBridge
	TAPName   string // Optional, managed by HYVE
}

type Network struct {
	mode          Mode
	bridge        *Bridge
	tap           *TAP
	interfaceName string
	bridgeOwned   bool
}

func CreateNetwork(config Config) (*Network, error) {
	switch config.Mode {
	case "", ModeNAT, ModeNone:
		return &Network{
			mode: config.Mode,
		}, nil

	case ModeTAP:
		return createTAPNetwork(config)

	case ModeBridge:
		return createBridgeNetwork(config)

	default:
		return nil, fmt.Errorf("unsupported network mode %q", config.Mode)
	}
}

func createTAPNetwork(config Config) (*Network, error) {
	if config.Interface == "" {
		return nil, fmt.Errorf("TAP networking requires an interface")
	}

	return &Network{
		mode:          ModeTAP,
		interfaceName: config.Interface,
	}, nil
}

func createBridgeNetwork(config Config) (*Network, error) {
	if config.Bridge == "" {
		return nil, fmt.Errorf("bridge networking requires a bridge")
	}

	bridge, err := CreateBridge(config.Bridge)
	if err != nil {
		return nil, fmt.Errorf("create bridge: %w", err)
	}

	network := &Network{
		mode:        ModeBridge,
		bridge:      bridge,
		bridgeOwned: bridge.created,
	}

	// No TAP requested: use the bridge directly.
	if config.TAPName == "" {
		return network, nil
	}

	tap, err := CreateTAP(config.TAPName)
	if err != nil {
		_ = network.Close()
		return nil, fmt.Errorf("create TAP: %w", err)
	}

	if err := bridge.AddInterface(tap.name); err != nil {
		_ = tap.Close()
		_ = network.Close()

		return nil, fmt.Errorf(
			"attach TAP %q to bridge %q: %w",
			tap.name,
			bridge.name,
			err,
		)
	}

	network.tap = tap

	return network, nil
}

func (n *Network) Interface() string {
	if n == nil {
		return ""
	}

	if n.tap != nil {
		return n.tap.name
	}

	if n.interfaceName != "" {
		return n.interfaceName
	}

	if n.bridge != nil {
		return n.bridge.name
	}

	return ""
}

func (n *Network) Close() error {
	if n == nil {
		return nil
	}

	var firstErr error

	if n.bridge != nil && n.tap != nil {
		if err := n.bridge.RemoveInterface(n.tap.name); err != nil {
			firstErr = err
		}
	}

	if n.tap != nil {
		if err := n.tap.Close(); err != nil && firstErr == nil {
			firstErr = err
		}

		n.tap = nil
	}

	if n.bridge != nil && n.bridgeOwned {
		if err := n.bridge.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	n.bridge = nil

	return firstErr
}
