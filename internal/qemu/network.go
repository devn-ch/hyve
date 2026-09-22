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
	var netdev string

	switch cfg.Mode {
	case "", NetworkNAT:
		netdev = "user,id=net0"

	case NetworkTAP:
		if cfg.Interface == "" {
			return nil, fmt.Errorf(
				"tap networking requires an interface",
			)
		}

		netdev = fmt.Sprintf(
			"tap,id=net0,ifname=%s,script=no,downscript=no",
			cfg.Interface,
		)

	case NetworkBridge:
		if cfg.Interface == "" {
			return nil, fmt.Errorf(
				"bridge networking requires an interface",
			)
		}

		netdev = fmt.Sprintf(
			"bridge,id=net0,br=%s",
			cfg.Interface,
		)

	case NetworkNone:
		return nil, nil

	default:
		return nil, fmt.Errorf(
			"unsupported network mode %q",
			cfg.Mode,
		)
	}

	device := "virtio-net-pci,netdev=net0"

	if cfg.MAC != "" {
		device += fmt.Sprintf(",mac=%s", cfg.MAC)
	}

	return []string{
		"-netdev",
		netdev,
		"-device",
		device,
	}, nil
}
