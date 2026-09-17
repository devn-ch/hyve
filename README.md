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

## build

The target OS is linux with activated KVM acceleration. For development you can run on MacOS in a dev container w/o KVM.

```
go mod tidy
make build
```