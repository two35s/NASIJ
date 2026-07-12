<div align="center">

```
                         ,▄▄▓⌐
                       ▐▀▄▄▀█▄
                       ╙██    █▌
              ¬,╓▄▄▄æR▀▀██▄,,▄█▀█▄
       ▄▀▀▀▀▀φ▄▄L,,,╓   ▄▄▄æφ▄  ▓▀▀╙╙██─   █    ╫███─    ██▄
     ╓▌           ─ ██▄j▌    ██▄█    █▌   j█     ▀▀╙     ████
     █   ,▄▄.       ████▌    ╙█▀▀    ╙                  █████⌐
     ▀██▌▀─        ╙╙╙└─                      ╔      ,▄██████
      █└    ,▄▓█           ,       ç   ,▓▄╓╓▄▓███▓▓█████████▀
     █     ▓▀  ╙▓,       ╓▓██▄▄▄▓█████████████████████████▀─
     ▌     █▄  ▄███████▀▀▀█▀└└╙███████▀██████████▀~╙▀▀▀▀└
     █      ╙╙╙╙└  ╙██▌        ████▌▄▓█████
     ╙▓             ███▓▄▄██▓▓█████████████Γ
       ▀▓▄µ    ,▄▄▓████████████████▌└██▀▀┘
         ▀███████████████ ▀███▀▀▀▀╙
          ┘▀██████████▀▀┘
              ┘┴┘¬
```

</div>

---

## Overview

NASIJ is an intelligent reconnaissance framework designed for **authorized Bug Bounty programs, Penetration Tests, and Application Security assessments**.

Unlike traditional reconnaissance tools that generate flat lists of URLs or endpoints, NASIJ builds an **interactive knowledge graph** describing how a web application works.

It correlates JavaScript, APIs, authentication, runtime behavior, dependencies, and framework intelligence into a unified model that helps researchers quickly understand an application's attack surface.

---

## Vision

Modern web applications are complex.

Traditional recon produces thousands of URLs, endpoints, and JavaScript files, forcing researchers to manually connect everything together.

NASIJ automates that correlation.

```
Application
      │
      ▼
JavaScript
      │
      ▼
APIs
      │
      ▼
Authentication
      │
      ▼
Dependencies
      │
      ▼
Knowledge Graph
      │
      ▼
Prioritized Manual Testing
```

The objective is **not automated exploitation**.

The objective is to dramatically reduce the time required to understand modern applications.

---

# Features

## Discovery

- Scope-aware crawling
- JavaScript discovery
- Dynamic chunk collection
- Source Map discovery
- OpenAPI discovery
- Swagger discovery
- GraphQL discovery
- Asset fingerprinting
- Hash-based tracking

---

## JavaScript Intelligence

- AST parsing
- Import resolution
- Dependency graph generation
- Route extraction
- Framework detection
- Configuration extraction
- Feature flag discovery
- Dynamic URL resolution

---

## Runtime Intelligence

Powered by Playwright.

Collects:

- Fetch requests
- XHR
- WebSockets
- Server-Sent Events
- Service Workers
- localStorage
- sessionStorage
- IndexedDB

---

## API Intelligence

Automatically maps:

- REST APIs
- GraphQL
- WebSockets

Including:

- Methods
- Parameters
- Authentication
- Source files
- Calling functions
- Runtime evidence

---

## Authentication Mapping

Detects:

- JWT
- OAuth
- Cookie sessions
- Refresh tokens
- Storage locations
- Login flows

Automatically generates authentication diagrams.

---

## Security Intelligence

Detects:

- Exposed secrets
- API keys
- Cloud credentials
- Internal hostnames
- Vulnerable dependencies
- Outdated frameworks
- Sensitive client-side data

---

## Knowledge Graph

The heart of NASIJ.

Every discovered object becomes part of a unified graph.

Example relationships:

```
Page

↓

Component

↓

JavaScript Module

↓

API

↓

Authentication

↓

Storage

↓

Finding
```

---

# Architecture

```mermaid
flowchart TB

%% ==========================
%% USER
%% ==========================

U[User]
SCOPE[Scope Definition<br/>Domains / IPs / Exclusions]
AUTH[Authorization]

U --> AUTH
AUTH --> SCOPE

%% ==========================
%% CORE
%% ==========================

subgraph CORE["NASIJ Core Engine"]

ENGINE[Orchestrator]

QUEUE[Task Queue]

WORKERS[Async Worker Pool]

ENGINE --> QUEUE
QUEUE --> WORKERS

end

SCOPE --> ENGINE

%% ==========================
%% CRAWLER
%% ==========================

subgraph DISCOVERY["Discovery Engine"]

CRAWLER[Crawler]

ROBOTS[robots.txt]

RATELIMIT[Rate Limiter]

JSDISC[JS Discovery]

SOURCEMAPS[Source Maps]

API_DOCS[Swagger / OpenAPI / GraphQL]

ASSETS[Asset Inventory]

CRAWLER --> ROBOTS
CRAWLER --> RATELIMIT
CRAWLER --> JSDISC
JSDISC --> SOURCEMAPS
JSDISC --> API_DOCS
JSDISC --> ASSETS

end

WORKERS --> CRAWLER

%% ==========================
%% RUNTIME
%% ==========================

subgraph RUNTIME["Runtime Observation"]

PLAYWRIGHT[Playwright]

XHR[XHR / Fetch]

WS[WebSockets]

SSE[SSE]

STORAGE[Storage Analysis]

SW[Service Workers]

PLAYWRIGHT --> XHR
PLAYWRIGHT --> WS
PLAYWRIGHT --> SSE
PLAYWRIGHT --> STORAGE
PLAYWRIGHT --> SW

end

WORKERS --> PLAYWRIGHT

%% ==========================
%% STATIC ANALYSIS
%% ==========================

subgraph STATIC["JavaScript Intelligence"]

AST[AST Parser]

IMPORTS[Import Resolver]

MODULEGRAPH[Dependency Graph]

ROUTES[Client Routes]

FEATURES[Feature Flags]

FRAMEWORK[Framework Detection]

AST --> IMPORTS
IMPORTS --> MODULEGRAPH

AST --> ROUTES
AST --> FEATURES
AST --> FRAMEWORK

end

ASSETS --> AST

%% ==========================
%% ANALYZERS
%% ==========================

subgraph ANALYZERS["Plugin Analyzer Layer"]

APIINT[API Intelligence]

AUTHMAP[Authentication Mapper]

SECRETSCAN[Secrets Scanner]

DEP[Dependency Scanner]

CVE[CVE Correlation]

SURFACE[Attack Surface Scoring]

APIINT --> SURFACE
AUTHMAP --> SURFACE
SECRETSCAN --> SURFACE
DEP --> CVE
CVE --> SURFACE

end

MODULEGRAPH --> APIINT
MODULEGRAPH --> AUTHMAP
MODULEGRAPH --> SECRETSCAN
MODULEGRAPH --> DEP

XHR --> APIINT
STORAGE --> AUTHMAP

FRAMEWORK --> DEP

%% ==========================
%% CANONICAL MODEL
%% ==========================

subgraph MODEL["Canonical Data Model"]

JSA[JSAsset]
APIREC[APIRecord]
AUTHFLOW[AuthFlow]
FW[Framework]
FINDING[Finding]

end

APIINT --> APIREC
AUTHMAP --> AUTHFLOW
FRAMEWORK --> FW
SECRETSCAN --> FINDING
SURFACE --> FINDING
ASSETS --> JSA

%% ==========================
%% GRAPH
%% ==========================

subgraph GRAPH["Knowledge Graph"]

GRAPHDB[Entity Graph]
REL[Relationships]

GRAPHDB --> REL

end

JSA --> GRAPHDB
APIREC --> GRAPHDB
AUTHFLOW --> GRAPHDB
FW --> GRAPHDB
FINDING --> GRAPHDB

%% ==========================
%% STORAGE
%% ==========================

subgraph STORAGE["Workspace"]

SQLITE[(SQLite)]
FILES[Downloaded Assets]
LOGS[Audit Logs]
SNAPSHOTS[Snapshots]
DIFF[Diff Engine]

SQLITE --> DIFF

end

GRAPHDB --> SQLITE
FILES --> SQLITE

%% ==========================
%% REPORTING
%% ==========================

subgraph REPORT["Reporting"]

HTML[HTML]
MD[Markdown]
JSON[JSON]
BURP[Burp Export]
POSTMAN[Postman]
GRAPHVIEW[Interactive Graph]

end

GRAPHDB --> GRAPHVIEW
DIFF --> HTML
DIFF --> MD
DIFF --> JSON
GRAPHDB --> BURP
GRAPHDB --> POSTMAN

%% ==========================
%% AI
%% ==========================

subgraph AI["Optional LLM Layer"]

LLM[Hypothesis Generator]

end

GRAPHDB --> LLM
FINDING --> LLM

LLM --> HTML
LLM --> MD
```

---

# Design Principles

- AST-first analysis
- Passive-first reconnaissance
- Plugin-based architecture
- Evidence-driven findings
- Knowledge graph correlation
- Resumable workspaces
- Differential reconnaissance
- Framework-aware analysis
- Reproducible results

---

# Roadmap

## Phase 1

- Project foundation
- CLI
- Workspace manager
- Scope manager
- HTTP engine

---

## Phase 2

- Smart crawler
- JavaScript collector
- AST parser
- Framework detection

---

## Phase 3

- Runtime observation
- API intelligence
- Authentication mapping
- Secrets detection

---

## Phase 4

- Dependency correlation
- Knowledge graph
- Interactive reports
- Differential reconnaissance

---

## Phase 5

- Plugin SDK
- Public API
- Optional AI hypothesis engine

---

# Legal & Ethics

NASIJ is designed **only** for systems you are explicitly authorized to assess.

The framework enforces:

- Scope restrictions
- Request rate limiting
- Audit logging
- Passive reconnaissance by default

NASIJ does **not** automate exploitation or vulnerability attacks.

---

# License

MIT License