# Testing

## Test build script flow

```
build.sh
   │
   ├── Debian Cloud Image
   │
   ├── NoCloud ISO
   │
   └── Provisioning-QEMU
          │
          ├── cloud-init
          ├── install qemu-guest-agent
          ├── activate QGA
          └── ready marker
                 │
                 ▼
          clean Shutdown
                 │
                 ▼
        debian-qga.qcow2
```

```
./testdata/qga/build.sh
        │
        ▼
.build/qga/debian-qga.qcow2
        │
        ▼
internal/qemu/qga_test.go
        │
        ├── start QEMU
        ├── connect virtio-serial/QGA
        ├── wait for Agent
        ├── GuestPing()
        └── QEMU exits gracefully
```

```
Debian boot
    │
    ▼
cloud-init
    │
    ├── apt update
    ├── install qemu-guest-agent
    ├── enable QGA
    ├── start QGA
    │
    ▼
power_state: poweroff
    │
    ▼
QEMU exits
    │
    ▼
built qcow2
```

## Testsuites

### TestGuestPing

```
go test
   │
   ▼
QEMU is started
   │
   ├── debian-qga.qcow2
   └── virtio-serial
          │
          ▼
     QGA socket
          │
          ▼
      GuestPing()
          │
          ▼
       cleanup
```

```
TestGuestPing
     │
     ▼
 QEMU is started
     │
     ▼
 Debian is booted
     │
     ▼
 qemu-guest-agent is started
     │
     ▼
 waitForQGA()
     │
     ▼
 GuestPing()
     │
     ▼
   PASS
     │
     ▼
 QEMU Kill + Wait
```

## Get started with testing

```sh
rm -rf .build/qga
```

build test ISO with QGA
```sh
./testdata/qga/build.sh
```

run a test, e.g. TestGuestPing
```sh
go test ./internal/qemu -run TestGuestPing -v -timeout 90s
```

## Troubleshooting

### change cloud-init/ user-data

problem: Nothing happens after changing something in user-data

solution: rebuild image
```sh
rm -rf .build/qga
./testdata/qga/build.sh
```

### How to run the built debian image to check things

```sh
qemu-system-x86_64 \
  -machine q35 \
  -m 512M \
  -smp 1 \
  -drive file=.build/qga/debian-qga.qcow2,if=virtio,format=qcow2 \
  -chardev socket,id=qga,path=/tmp/hyve-qga.sock,server=on,nowait \
  -device virtio-serial \
  -device virtserialport,chardev=qga,name=org.qemu.guest_agent.0 \
  -nographic
```

exit QEMU with Ctrl+a then x