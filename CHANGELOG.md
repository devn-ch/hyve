# Changelog

Description see [README](README.md)


## Release v0.1

✅ TestNetworkArgs

✅ TestGuestDHCP

✅ TestGuestPing

✅ QGA with VirtIO-Serial

✅ DHCP with TAP → br0

✅ Guest get an IPv4: 192.168.100.157/24

✅ QGA has connectivity to read the Guest-Netzwerkinterfaces

✅ common QEMU-Test-Startup

✅ KVM-acceleration support in the tests

HYVE scope

```
HYVE x86_64
├── Linux Host
├── QEMU/KVM
├── hyved
├── hyve CLI
├── Bridge Networking
├── DHCP Guest-IP
├── QGA
├── hyve info
├── Port Exposure
└── Boot from USB-Stick
```

HYVE Live-System

```
USB
 │
 ▼
Debian minimal
 │
 ├── hyved.service
 │
 ├── br0
 │
 ├── QEMU/KVM
 │
 └── /var/lib/hyve
       │
       └── VMs
```

Host bootstrap system

```
/etc/hyve/
    hyve.conf

/etc/systemd/system/
    hyved.service

/usr/local/bin/
    hyve
    hyved

/var/lib/hyve/
    vms/

/run/hyve/
    qmp/
    qga/
    console/
```
