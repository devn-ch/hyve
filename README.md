# HYVE

**Lightweight virtualization. Anywhere.**

HYVE is a lightweight, API-first virtualization platform designed to run and manage virtual machines on x86_64 and ARM64 Linux hosts.

Built on [QEMU](https://www.qemu.org/)/ [KVM](https://linux-kvm.org/), HYVE aims to make on-premises virtualization simpler, more secure, and easier to operate — from small business servers and edge devices to developer environments and homelabs.

## Vision

HYVE focuses on reducing operational complexity through centralized management, controlled remote access, and reliable VM lifecycle automation.

Our goal is to provide a practical foundation for organizations that need flexible, locally operated infrastructure without the overhead of traditional virtualization platforms.

## Key Features

* Lightweight VM management powered by [QEMU](https://www.qemu.org/)/ [KVM](https://linux-kvm.org/)
* x86_64 and ARM64 support
* API-first architecture
* Integrated VM console
* QEMU Guest Agent integration
* Recovery and health monitoring
* Secure, centralized administration

HYVE is currently under active development.

## Architecture

<details>
 <summary><h3>HYVE daemon (hyved)</h3></summary>

```
      HYVE
       │
   ┌───┴────┐
   │ hyved  │
   │ daemon │
   └───┬────┘
       │
  VM lifecycle
       │
   ┌───▼───┐
   │ QEMU  │
   │ + KVM │
   └───────┘
       │
   ┌───▼───┐
   │ Guest │
   │  VM   │
   └───────┘
```
</details>


<details>
 <summary><h3>HYVE CLI (hyve)</h3></summary>

```
   ┌──────────────┐
   │     CLI      │
   └──────┬───────┘
         │
   ┌──────▼───────┐
   │   VM Manager │
   └──────┬───────┘
         │
   ┌──────▼───────┐
   │ QEMU Driver  │
   └──────┬───────┘
         │
   ┌──────▼───────┐
   │    QEMU      │
   └──────────────┘
```
</details>


<details>
 <summary><h3>HYVE - VM</h3></summary>

```
            VM
            │
   ┌───────┼────────┐
   │       │        │
   State    QMP      QGA
                     │
                  Guest
```
</details>


<details>
 <summary><h3>HYVE - QMP and QGA</h3></summary>

```
         HYVE
           │
      ┌────┴────┐
      │         │
      QMP       QGA
      │         │
lifecycle  guest interaction
      │         │
      └────┬───┘
           │
          QEMU
           │
      ┌────┴─────┐
      │          │
   disk      network
      │          │
      ▼          ▼
   Guest OS     NIC
```

```
QMP socket  → HYVE ↔ QEMU
QGA socket  → HYVE ↔ Guest Agent
Console     → VNC/Serial
```

```
QEMU
 ├── QMP  → Host-/VM-control
 │
 └── QGA  → Guest-control
              ├── ping
              ├── shutdown
              └── network interfaces
```

HYVE stop with QGA and QMP v1
```
hyve stop <VM>
      │
      ▼
     QGA
      │
      ├── guest-shutdown
      │
      ▼
     wait
      │
      ├── successfully → done
      │
      └── Timeout
             │
             ▼
            QMP
       system_powerdown
             │
             ▼
          Timeout
             │
             ▼
          kill VM
```

HYVE stop with QGA and QMP v2
```
hyve stop <VM>
   │
   ▼
  QGA
   │
   └── guest-shutdown
           │
           ▼
        Guest is shutting down
        sauber herunter
           │
           ▼
       QEMU exits
```

Activate the QEMU guest agent feature in your VM.
```sh
sudo systemctl enable --now qemu-guest-agent
```
</details>


<details>
 <summary><h3>HYPE Guest ping</h3></summary>

```
HYVE
 │
 │ Start()
 ▼
QEMU
 │
 │ virtio-serial
 ▼
Guest
 │
 │ qemu-ga
 ▼
QGA socket
 │
 ▼
HYVE.GuestPing()
 │
 └── {"execute":"guest-ping"}
             │
             ▼
        {"return":{}}
```
</details>


<details>
 <summary><h3>HYVE - Socket lifecycle</h3></summary>

```
hyve run test
      │
      ▼
   QEMU is started
      │
      ├── /run/hyve/qmp/test.sock
      └── /run/hyve/console/test.sock
      │
      ▼
   QEMU is running
      │
      ▼
   QEMU is stopped
      │
      ├── QMP-Socket is deleted
      └── Console-Socket is deleted
```
</details>


<details>
 <summary><h3>HYVE - Console lifecycle</h3></summary>

```
hyve console test
       │
       ├── TCP listener is started
       ├── remote-viewer is started
       ├── proxy connection is opened
       │
       └── remote-viewer is exited
                │
                ├── Listener is closed
                └── Proxy-connection is closed
                       │
                       ▼
                 hyve console is exited
```
</details>


<details>
 <summary><h3>HYVE - Frontend concept</h3></summary>

Web UI:
```
Browser
   │
   │ HTTPS
   ▼
HYVE Web UI
   │
   ├── VM management
   ├── console access
   └── API
          │
          ▼
        hyved
          │
          ▼
       QEMU
```

FE console:
```
Browser
   │
   │ HTTPS / WebSocket
   ▼
HYVE Web Frontend
   │
   │ Unix socket
   ▼
QEMU VNC
```

Authentication und Authorization
```
User
  ↓
HTTPS
  ↓
HYVE
  ├── is allowed to view VM "test"?
  ├── is allowed to open Console?
  ├── is allowed to stop VM?
  └── is allowed to destroy VM?
```
That would keep QEMU independend.
</details>

<details>
 <summary><h3>HYVE - Bridge networking</h3></summary>

```
NetworkConfig
    │
    ├── Mode: NetworkBridge
    ├── Interface: br0
    └── MAC: 52:54:00:12:34:56
             │
             ▼
        QEMU virtio-net
             │
             ▼
           br0
             │
             ▼
          DHCP
             │
             ▼
     192.168.0.42/24
```
</details>


## How to build

The target OS is linux with KVM acceleration support. For development you can also run on MacOS in a dev container but w/o KVM feature.

```
go mod tidy
make build
```

## How to test a QEMU configuration

```sh
mkdir tmp && cd tmp
```

place a test ISO, e.g. talos linux which requires graphical output instead of serial
```sh
wget https://factory.talos.dev/?arch=amd64&platform=nocloud&schematic-id=df23a85b80a3e6e1c04b009713cea469ddf98f3046d8892e7991c33dcb519fa2&target=cloud&version=1.14.1
cd ..
```

```sh
sudo ./bin/hyve create test --cpus 2 --memory 2048 --drive disk:10G --drive cdrom:$(pwd)/tmp/nocloud-amd64.iso
```

shows the files in the directory /var/lib/hyve/vms/test an, which was created by Hyve-VM 
```sh
sudo find /var/lib/hyve/vms/test -maxdepth 2 -type f -ls
```

test your QEMU configuration
```sh
sudo qemu-system-x86_64 \
  -enable-kvm \
  -cpu host \
  -m 2048 \
  -smp 2 \
  -cdrom $(pwd)/tmp/nocloud-amd64.iso \
  -drive file=/var/lib/hyve/vms/test/disk-0.qcow2,format=qcow2,if=virtio \
  -display none \
  -vnc unix:/tmp/test-vnc.sock
```

expose the VNC socket to VNC TCP for e.g. a remote-viewer app
```sh
sudo socat TCP-LISTEN:5900,reuseaddr,fork UNIX-CONNECT:/tmp/test-vnc.sock
```


## Testing

<details>
 <summary><h3>Testsuite for QEMU features</h3></summary>

Testsuite for all QEMU features
```sh
go test ./internal/qemu -v -timeout 120s
```

Check QEMU-Network arguments
```sh
go test ./internal/qemu -run TestNetworkArgs -v
```

Check QEMU-Guest DCHP bridge
```sh
go test ./internal/qemu -run TestGuestDHCPBridge -v
```

Check QEMU-QGA Ping
```sh
go test ./internal/qemu -run TestGuestPing -v
```
</details>