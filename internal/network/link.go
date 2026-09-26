package network

type LinkManager interface {
	Exists(name string) (bool, error)
	IsBridge(name string) (bool, error)

	AddBridge(name string) error
	DeleteLink(name string) error

	SetUp(name string) error
	SetMaster(link, master string) error
	SetNoMaster(link string) error
}
