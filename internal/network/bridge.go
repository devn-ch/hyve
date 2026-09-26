package network

import "fmt"

type Bridge struct {
	name    string
	created bool
	links   LinkManager
}

func CreateBridge(name string) (*Bridge, error) {
	return createBridge(name, newLinkManager())
}

func createBridge(name string, links LinkManager) (*Bridge, error) {
	if name == "" {
		return nil, fmt.Errorf("bridge name is empty")
	}

	if links == nil {
		return nil, fmt.Errorf("link manager is nil")
	}

	exists, err := links.Exists(name)
	if err != nil {
		return nil, err
	}

	if exists {
		isBridge, err := links.IsBridge(name)
		if err != nil {
			return nil, err
		}

		if !isBridge {
			return nil, fmt.Errorf("%q already exists and is not a bridge", name)
		}

		return &Bridge{
			name:  name,
			links: links,
		}, nil
	}

	if err := links.AddBridge(name); err != nil {
		return nil, err
	}

	if err := links.SetUp(name); err != nil {
		_ = links.DeleteLink(name)
		return nil, fmt.Errorf("bring bridge %q up: %w", name, err)
	}

	return &Bridge{
		name:    name,
		created: true,
		links:   links,
	}, nil
}

func (b *Bridge) AddInterface(name string) error {
	if b == nil {
		return fmt.Errorf("bridge is nil")
	}

	if name == "" {
		return fmt.Errorf("interface name is empty")
	}

	exists, err := b.links.Exists(name)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("interface %q does not exist", name)
	}

	if err := b.links.SetMaster(name, b.name); err != nil {
		return fmt.Errorf(
			"attach interface %q to bridge %q: %w",
			name,
			b.name,
			err,
		)
	}

	return nil
}

func (b *Bridge) RemoveInterface(name string) error {
	if b == nil {
		return fmt.Errorf("bridge is nil")
	}

	if name == "" {
		return fmt.Errorf("interface name is empty")
	}

	if err := b.links.SetNoMaster(name); err != nil {
		return fmt.Errorf(
			"detach interface %q from bridge %q: %w",
			name,
			b.name,
			err,
		)
	}

	return nil
}

func (b *Bridge) Close() error {
	if b == nil || !b.created {
		return nil
	}

	if err := b.links.DeleteLink(b.name); err != nil {
		return err
	}

	b.created = false

	return nil
}
