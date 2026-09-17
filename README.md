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

```
go mod tidy
make build
```