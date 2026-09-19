# HYVE

Micro-Hypervisor for virtual environments. As the alternative with less resource requirements than hypvervisors like Proxmox.

## Architecture

### HYVE daemon (hyved)
```
             HYVE
              │
        ┌─────┴────┐
        │  hyved   │
        │  daemon  │
        └─────┬────┘
              │
         VM lifecycle
              │
       ┌──────▼──────┐
       │     QEMU    │
       │    + KVM    │
       └─────────────┘
              │
         ┌────▼────┐
         │  Guest  │
         │   VM    │
         └─────────┘
```

### HYVE CLI

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

### HYVE - VM

```
              VM
              │
      ┌───────┼────────┐
      │       │        │
    State    QMP      QGA
                       │
                     Guest
```

### HYVE - QMP and QGA
```
            HYVE
             │
        ┌────┴────┐
        │         │
       QMP       QGA
        │         │
   lifecycle  guest interaction
        │         │
        └─────┬───┘
              │
             QEMU
              │
        ┌─────┴─────┐
        │           │
      disk       network
        │           │
        ▼           ▼
     Guest OS      NIC
```

### HYVE - Socket lifecycle

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

### Console lifecycle

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

## build

The target OS is linux with activated KVM acceleration. For development you can run on MacOS in a dev container w/o KVM.

```
go mod tidy
make build
```

## How to test a QEMU configuration

```sh
mkdir tmp && cd tmp
```

place a test ISO, here talos linux which requires graphical output instead of serial
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

test QEMU configuration
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