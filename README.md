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
