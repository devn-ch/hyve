# HYVE

Micro-Hypervisor for virtual environments. As the alternative with less resource requirements than hypvervisors like Proxmox.

## Architecture

```
                    HYVE
                     │
             ┌───────┴───────┐
             │     hyved     │
             │     daemon    │
             └───────┬───────┘
                     │
              VM lifecycle
                     │
             ┌───────▼───────┐
             │     QEMU      │
             │    + KVM      │
             └───────────────┘
                     │
                ┌────▼────┐
                │  Guest  │
                │   VM    │
                └─────────┘
```

## CLI

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

HYVE VM
```
              VM
              │
      ┌───────┼────────┐
      │       │        │
    State    QMP      QGA
                       │
                     Guest
```

## build

The target OS is linux with activated KVM acceleration. For development you can run on MacOS in a dev container w/o KVM.

```
go mod tidy
make build
```