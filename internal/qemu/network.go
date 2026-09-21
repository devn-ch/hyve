package qemu

import "fmt"

type NetworkMode string

const (
	NetworkNAT    NetworkMode = "nat"
	NetworkTAP    NetworkMode = "tap"
	NetworkBridge NetworkMode = "bridge"
	NetworkNone   NetworkMode = "none"
)

type NetworkConfig struct {
	Mode      NetworkMode
	Interface string
	MAC       string
}

func networkArgs(cfg NetworkConfig) ([]string, error) {
	switch cfg.Mode {
	case "", NetworkNAT:
		return []string{
			"-netdev",
			"user,id=net0",
			"-device",
			"virtio-net-pci,netdev=net0",
		}, nil

	case NetworkTAP:
		if cfg.Interface == "" {
			return nil, fmt.Errorf(
				"tap networking requires an interface",
			)
		}

		return []string{
			"-netdev",
			fmt.Sprintf(
				"tap,id=net0,ifname=%s,script=no,downscript=no",
				cfg.Interface,
			),
			"-device",
			"virtio-net-pci,netdev=net0",
		}, nil

	case NetworkBridge:
		if cfg.Interface == "" {
			return nil, fmt.Errorf(
				"bridge networking requires an interface",
			)
		}

		return []string{
			"-netdev",
			fmt.Sprintf(
				"bridge,id=net0,br=%s",
				cfg.Interface,
			),
			"-device",
			"virtio-net-pci,netdev=net0",
		}, nil

	case NetworkNone:
		return nil, nil

	default:
		return nil, fmt.Errorf(
			"unsupported network mode %q",
			cfg.Mode,
		)
	}
}
