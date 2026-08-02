# RootGuard Core API

RootGuard Core exposes an internal JSON API for the RootGuard WebApp. It is not
a public administration API and must remain on RootGuard's private controller
network.

This reference follows the route registrations in `internal/api/routes.go`.
Response structures may gain fields between pre-1.0 releases; clients should
ignore unknown JSON fields.

## Authentication and errors

`GET /api/health` is the **only unauthenticated endpoint**. Every other route,
including the AdGuard Home proxy, requires the internal token:

```http
Authorization: Bearer <ROOTGUARD_API_TOKEN>
```

Missing or invalid credentials return `401 Unauthorized`. JSON errors use a
bounded response such as:

```json
{"error":"request could not be completed"}
```

The examples below contain placeholders only. Never put a real token,
credential, host address, or production response in documentation or reports.

## Health and runtime

| Method and path | Purpose | Representative response |
| --- | --- | --- |
| `GET /api/health` | Process health; no authentication | `{"status":"ok"}` |
| `GET /api/system` | Core operating system and architecture | `{"os":"linux","arch":"amd64"}` |
| `GET /api/docker/status` | Docker API reachability | Status object |
| `GET /api/stack/status` | Managed container state and release provenance | Stack status object |
| `GET /api/dashboard` | Aggregated installation, DNS-chain, resource, and privacy-preserving DNS metrics | Dashboard object |
| `GET /api/services` | Bounded runtime metadata for allowlisted services | Array of service objects |
| `GET /api/services/{name}/logs` | Recent bounded and redacted diagnostic logs | Log window object |
| `POST /api/services/{name}/{action}` | Run an allowlisted lifecycle action | Updated service status |

`{name}` and `{action}` are validated server-side. Browser callers cannot
provide an image, container name, command, mount, or configuration path.

## Installation

| Method and path | Purpose | Representative response |
| --- | --- | --- |
| `GET /api/installation` | Read persistent deployment progress | Installation status |
| `POST /api/installation/preflight` | Validate a proposed local DNS endpoint without deploying | Preflight report |
| `POST /api/installation/deploy` | Start the asynchronous managed deployment | `202 Accepted` with installation status |

Representative request:

```json
{
  "dns_bind_address": "192.0.2.10",
  "dns_port": 53,
  "adguard_channel": "stable"
}
```

Unknown fields and malformed bodies are rejected. Addresses and ports above are
documentation-only examples.

## Data-plane updates

| Method and path | Purpose | Representative response |
| --- | --- | --- |
| `GET /api/updates` | Read check, update, rollback, backup, and cleanup state | Update status and history |
| `POST /api/updates/check` | Start an asynchronous image check | `202 Accepted` with update status |
| `POST /api/updates/{name}` | Update one allowlisted DNS service | `202 Accepted` with update status |

Only the configured AdGuard Home and Unbound services are accepted. Images are
owned by Core configuration, not by the request.

## Control-plane updates

| Method and path | Purpose | Representative response |
| --- | --- | --- |
| `GET /api/control-plane-updates` | Read paired Core/WebApp update state | Control-plane status |
| `POST /api/control-plane-updates/check` | Ask the isolated updater to check both images | `202 Accepted` with updater result |
| `POST /api/control-plane-updates/install` | Start an atomic paired update | `202 Accepted` with updater result |

The independent updater owns the exact services, image references, Compose
arguments, health checks, and rollback procedure.

## Unbound settings and configuration

| Method and path | Purpose | Representative response |
| --- | --- | --- |
| `GET /api/unbound/settings` | Read active guided settings | Settings object |
| `GET /api/unbound/config` | Read the active base and managed configuration files | Configuration object |
| `PUT /api/unbound/settings` | Validate and activate guided settings | Updated settings and version |
| `POST /api/unbound/preview` | Render a side-effect-free change preview | Diff and generated configuration |
| `GET /api/unbound/history` | Read the bounded configuration history | Array of version entries |
| `POST /api/unbound/history/{id}/restore` | Validate and restore a prior version | Restored version entry |
| `GET /api/unbound/diagnostics` | Run bounded resolver and DNSSEC diagnostics | Diagnostic checks |
| `GET /api/unbound/presets` | List safe guided profiles | Array of preset objects |
| `POST /api/unbound/advice` | Evaluate a draft deterministically | Array of advice entries |
| `POST /api/unbound/forward-check` | Probe configured forwarding targets | Bounded probe report |
| `GET /api/unbound/network-capabilities` | Report verified IPv4/IPv6 capability | Capability object |

Representative partial settings payload:

```json
{
  "qname_minimisation": true,
  "prefetch": true,
  "serve_expired": true,
  "cache_min_ttl": 0,
  "cache_max_ttl": 86400,
  "threads": 2,
  "network_mode": "ipv4"
}
```

Activation performs candidate and effective `unbound-checkconf` validation,
atomic replacement, restart health checks, versioning, and rollback.

## Diagnostic logging

| Method and path | Purpose | Representative response |
| --- | --- | --- |
| `GET /api/unbound/diagnostic-logging` | Read temporary logging state | Logging status |
| `POST /api/unbound/diagnostic-logging` | Start bounded temporary diagnostic logging | Logging status |
| `DELETE /api/unbound/diagnostic-logging` | Stop temporary diagnostic logging | Logging status |

## Expert configuration

| Method and path | Purpose | Representative response |
| --- | --- | --- |
| `GET /api/unbound/custom` | Read the guarded custom configuration | Custom configuration object |
| `POST /api/unbound/custom/preview` | Preview and validate a custom draft | Diff, advice, and validation result |
| `PUT /api/unbound/custom` | Validate and activate custom content | Updated configuration and version |
| `GET /api/unbound/directives` | Read completion and documentation metadata | Directive metadata array |

The expert API rejects directives that could change listeners, remote control,
trust anchors, file inclusion, DNSSEC enforcement, or guided-setting ownership.

## AdGuard Home

| Method and path | Purpose | Representative response |
| --- | --- | --- |
| `GET /api/adguard/status` | Read bootstrap and protected-upstream state | AdGuard status |
| `GET /api/adguard/filter-report` | Run bounded local filter probes | Filter report |
| `POST /api/adguard/bootstrap` | Reconcile the private installer and DNS baseline | AdGuard status |
| `/api/adguard/ui/*` | Reverse proxy to the private native administration UI | Native AdGuard response |

The proxy injects Core-owned private credentials and never publishes AdGuard's
administration port. The WebApp applies its own authenticated session and
same-origin boundary before forwarding browser traffic to Core.
