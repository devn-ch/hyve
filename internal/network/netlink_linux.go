//go:build linux

package network

import (
	"errors"
	"fmt"

	"github.com/vishvananda/netlink"
)

type netlinkManager struct{}

func newLinkManager() LinkManager {
	return &netlinkManager{}
}

func (m *netlinkManager) Exists(name string) (bool, error) {
	_, err := netlink.LinkByName(name)
	if err == nil {
		return true, nil
	}

	var notFound netlink.LinkNotFoundError
	if errors.As(err, &notFound) {
		return false, nil
	}

	return false, fmt.Errorf("find link %q: %w", name, err)
}

func (m *netlinkManager) IsBridge(name string) (bool, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return false, fmt.Errorf("find link %q: %w", name, err)
	}

	_, ok := link.(*netlink.Bridge)
	return ok, nil
}

func (m *netlinkManager) AddBridge(name string) error {
	bridge := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: name,
		},
	}

	if err := netlink.LinkAdd(bridge); err != nil {
		return fmt.Errorf("create bridge %q: %w", name, err)
	}

	return nil
}

func (m *netlinkManager) DeleteLink(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find link %q: %w", name, err)
	}

	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("delete link %q: %w", name, err)
	}

	return nil
}

func (m *netlinkManager) SetUp(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find link %q: %w", name, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring link %q up: %w", name, err)
	}

	return nil
}

func (m *netlinkManager) SetMaster(link, master string) error {
	iface, err := netlink.LinkByName(link)
	if err != nil {
		return fmt.Errorf("find link %q: %w", link, err)
	}

	bridge, err := netlink.LinkByName(master)
	if err != nil {
		return fmt.Errorf("find master %q: %w", master, err)
	}

	if err := netlink.LinkSetMaster(iface, bridge); err != nil {
		return fmt.Errorf("set master %q for link %q: %w", master, link, err)
	}

	return nil
}

func (m *netlinkManager) SetNoMaster(link string) error {
	iface, err := netlink.LinkByName(link)
	if err != nil {
		return fmt.Errorf("find link %q: %w", link, err)
	}

	if err := netlink.LinkSetNoMaster(iface); err != nil {
		return fmt.Errorf("remove master from link %q: %w", link, err)
	}

	return nil
}
