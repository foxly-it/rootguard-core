# 🛡 RootGuard Core

![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)
![Go](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go)
![Status](https://img.shields.io/badge/status-active--development-orange)
![Architecture](https://img.shields.io/badge/architecture-engine--layer-purple)

---

## 📌 Overview

RootGuard Core is the authenticated infrastructure control plane of the
RootGuard ecosystem.

It provides deterministic orchestration logic for:

- Docker stacks
- DNS services (AdGuard + Unbound)
- System service management
- Health monitoring
- Configuration generation

RootGuard Core contains **no UI layer**.  
It is designed to be consumed by:

- CLI tools
- HTTP APIs
- Automation systems
- Future GitOps integrations

---

## 🏗 Architecture Role

```
+----------------------+
|   RootGuard WebApp   |
|  (HTTP API + UI)     |
+----------+-----------+
           |
           v
+----------------------+
|   RootGuard Core     |
|  (Engine Layer)      |
+----------+-----------+
           |
           v
+----------------------+
| Docker / Systemd /   |
| AdGuard / Unbound    |
+----------------------+
```

---

## 📂 Repository Structure

```
rootguard-core/
├── cmd/
│   └── rootguard/
│       └── main.go
├── internal/
│   ├── api/
│   ├── configbuilder/
│   ├── docker/
│   ├── health/
│   ├── stack/
│   └── system/
├── go.mod
└── go.sum
```

---

## 🚀 Local Development

### Build

```bash
go build ./...
```

### Run

```bash
ROOTGUARD_API_TOKEN="$(openssl rand -hex 32)" go run ./cmd/rootguard
```

Core listens on port `8081` by default. Except for `/api/health`, every route
requires `Authorization: Bearer <ROOTGUARD_API_TOKEN>`. Core is intended for an
internal container network and must not be exposed directly to a LAN or WAN.

### Unbound settings API

`GET /api/unbound/settings` returns the active RootGuard settings. A validated
`PUT` generates a modular Unbound configuration, checks it inside the resolver
container and restarts Unbound only after successful validation.

Unbound Configuration v2 adds the following authenticated endpoints:

- `POST /api/unbound/preview` renders and compares a proposal without writing.
- `GET /api/unbound/history` returns up to 20 versioned configurations.
- `POST /api/unbound/history/{id}/restore` validates and restores a version.
- `GET /api/unbound/diagnostics` checks syntax, resolution, and DNSSEC rejection.

Every successful change records the previous and active state. If Unbound
cannot restart after a change, Core restores the previous files and restarts
the resolver again before returning an error.

### AdGuard bootstrap API

`GET /api/adguard/status` reports whether AdGuard Home is configured, healthy,
and connected exclusively to the RootGuard Unbound resolver. `POST
/api/adguard/bootstrap` performs the one-time installer flow, generates and
stores credentials with owner-only permissions, validates Unbound through the
official AdGuard API, and applies it without a public fallback resolver.

The AdGuard credentials never leave Core and the API deliberately exposes no
generic AdGuard proxy. Persist `ADGUARD_DATA_DIR` (default:
`/var/lib/rootguard/adguard`) when running the container.

---

## 🧱 Example: Stack Deployment Flow

```go
package main

import (
    "github.com/foxly-it/rootguard-core/internal/stack"
)

func main() {
    err := stack.DeployStack()
    if err != nil {
        panic(err)
    }
}
```

---

## 🎯 Design Principles

RootGuard Core follows strict engineering constraints:

- No UI coupling
- No framework lock-in
- No runtime shell dependency
- Deterministic state transitions
- Minimal external dependencies
- Security-first defaults

---

## 🔮 Project Direction

RootGuard Core is intended to evolve into:

- A full infrastructure control plane engine
- API-consumable orchestration service
- Multi-node DNS management backend
- GitOps-ready stack controller
- Extensible service abstraction layer

---

## 📜 License

Licensed under the Apache License 2.0.

See the LICENSE file for full details.

---

## ⚠ Development Status

This project is under active development.

Breaking changes may occur until the first stable release (v1.0.0).
