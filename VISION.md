# NASIJ — Project Vision (v2)

## Mission

NASIJ is an intelligent reconnaissance framework for bug bounty hunters and
penetration testers. It automatically collects, analyzes, correlates, and
visualizes client-side application intelligence for web applications the
user is **explicitly authorized** to assess.

Instead of producing flat lists of URLs or endpoints, NASIJ builds an
interactive knowledge graph of how an application works — and ranks what it
finds by how likely it is to matter — so researchers can skip repetitive
recon and go straight to informed manual testing.

---

## Legal & Ethical Guardrails (new)

This section exists because a recon tool that scales is also a tool that can
scale harm if it's careless. These aren't optional add-ons — they're load-bearing:

- **Scope enforcement, not just scope awareness.** Every workspace requires a
  scope definition (allowed domains/IP ranges, explicit exclusions) loaded
  before the first request. The crawler refuses to touch anything outside it,
  rather than relying on the operator to stay careful.
- **Politeness by default.** Conservative concurrency, request rate limits,
  and respect for `robots.txt` and rate-limit headers (`429`, `Retry-After`)
  out of the box. Aggressive modes require an explicit opt-in flag, not a
  default.
- **Passive-first posture.** No fuzzing, no auth bypass attempts, no payload
  injection, ever. If an analyzer would need to *send* something adversarial
  to get an answer, it's out of scope for this tool.
- **Audit log.** Every request the tool makes is logged with target, timestamp,
  and triggering module — useful both for engagement reports and for proving
  you stayed in scope.

---

## Objectives

Reduce time spent on repetitive recon by:

- Discovering JavaScript assets and their runtime behavior
- Mapping application architecture and client-side routing
- Correlating API usage across static code and live traffic
- Identifying authentication flows and session mechanics
- Detecting framework-specific features automatically
- **Surfacing secrets, sensitive data, and vulnerable dependencies**
- **Ranking findings by likely security relevance**
- Tracking application changes across scans
- Organizing recon into reusable, resumable workspaces

---

## Workflow

```
Define scope & authorization
        ↓
Crawl application (scope-gated, rate-limited)
        ↓
Collect JavaScript (static + dynamically loaded)
        ↓
Observe runtime behavior (Playwright)
        ↓
Build module dependency graph
        ↓
Extract API inventory
        ↓
Map authentication flows
        ↓
Identify frameworks & libraries
        ↓
Scan for secrets, sensitive data, vulnerable dependencies
        ↓
Score & prioritize findings
        ↓
Generate knowledge graph
        ↓
Produce searchable, exportable reports
        ↓
Guide manual security testing
```

---

## Core Capabilities

### Discovery
- Crawl websites within defined scope
- Collect JS files (inline, external, dynamically injected)
- Discover lazily-loaded / code-split chunks
- Identify and fetch source maps where exposed
- Enumerate exposed API docs (OpenAPI, Swagger, GraphQL introspection, Postman collections)
- Inventory all discovered assets with hashes for change detection

### JavaScript Intelligence
- Parse ASTs (Babel/Acorn-based)
- Resolve static and dynamic imports
- Resolve dynamically constructed URLs and template literals
- Build module dependency graphs
- Detect framework and bundler usage
- Identify feature flags and config objects
- Analyze client-side routing tables

### Runtime Intelligence (Playwright)
- Fetch / XHR / WebSocket / SSE traffic
- Service worker registration and behavior
- Dynamic module loading at runtime
- DOM-based storage writes (localStorage/sessionStorage/IndexedDB) tied to the code that wrote them

### API Intelligence
For every discovered REST, GraphQL, or WebSocket endpoint, record:
- HTTP method, route, parameters
- Authentication mechanism observed
- Calling JS file and function
- Whether it was found statically, at runtime, or both (confidence signal)

### Authentication Mapping
- JWT usage (and whether claims are inspectable client-side)
- OAuth flow type
- Cookie-based session mechanics
- Refresh token patterns
- Client-side credential/token storage locations
- Auto-generated auth flow diagrams

### Framework Intelligence
Auto-detect React, Next.js, Angular, Vue, Nuxt, Astro, Svelte, Remix, Vite,
Webpack — and load framework-specific analyzers (e.g., Next.js API route
conventions, Nuxt server middleware patterns).

### Secrets & Sensitive Data Detection (new)
- Regex + entropy-based scanning for API keys, cloud credentials, internal
  hostnames, and leaked tokens in bundled JS and source maps
- PII pattern detection (emails, internal usernames, etc.) surfaced with
  location, not content, in summary views to avoid the report itself becoming
  a liability

### Dependency & Vulnerability Correlation (new)
- Match detected libraries/versions against known-vulnerable-library
  databases (retire.js-style)
- Flag outdated framework versions with known CVEs
- Link flagged dependencies back to the module graph so you know *where*
  they're used, not just that they exist

### Attack Surface Prioritization (new)
- Score each discovered endpoint/flow using heuristics: auth-related routes,
  file upload/download handlers, admin-panel indicators, endpoints taking
  user-controlled params that flow into `fetch`/`redirect`/`innerHTML`-adjacent
  sinks, GraphQL introspection left enabled, etc.
- Present findings sorted by score, not discovery order

---

## Canonical Data Model (new)

All analyzers emit data in a shared schema so plugins can be composed and
findings correlated across categories. Rough shape:

```
JSAsset       { id, url, hash, type, source_map_url?, first_seen, last_seen }
APIRecord     { id, method, route, params[], auth_mechanism?, source_file,
                source_function, discovered_via: [static|runtime], confidence }
AuthFlow      { id, type, storage_location, endpoints[], evidence[] }
Framework     { name, version?, confidence, detected_via[] }
Finding       { id, category, severity_score, evidence[], related_ids[] }
```

This is what makes differential scanning, cross-plugin correlation, and the
knowledge graph actually work together instead of being three separate
outputs bolted side by side.

---

## Workspace Management

Each target gets an isolated workspace:
- Downloaded assets (deduplicated by hash)
- SQLite scan database (enables efficient diffing and querying, vs. flat files)
- Reports, graphs, screenshots, logs
- Historical scan snapshots
- Resumable state — a killed scan picks back up rather than restarting

---

## Differential Reconnaissance

Compare scans over time to highlight:
- New JS files or removed ones
- New/changed/removed API endpoints
- Changed client-side routes
- Modified authentication flows
- Newly observed features, newly exposed secrets, newly vulnerable dependencies

Diffs should be a first-class report type, not an afterthought — "what
changed since last week" is often the highest-signal output for a target
you're monitoring long-term.

---

## Reporting

Formats: HTML, Markdown, JSON.

- **Interactive graph view** (force-directed) for module dependencies and
  API call relationships — this is the knowledge graph the name promises; a flat
  list undersells it
- Findings sorted by priority score, not discovery order
- Export to formats that plug into existing workflow: Burp Suite scope/sitemap,
  Postman collection, OpenAPI spec
- Reports summarize architecture *and* link directly to the underlying
  evidence (source file, line, request/response) for manual follow-up

---

## Architecture & Extensibility

- **Plugin-based analyzers**, each conforming to the canonical data model
  above so outputs compose
- **AST-first parsing** as the core JS analysis strategy
- **Fast asynchronous scanning** (worker pool, configurable concurrency)
- Optional **LLM-assisted correlation layer**: a separate, opt-in phase that
  takes the deterministic graph/findings output and generates plain-language
  testing hypotheses (e.g., "this endpoint takes a user-controlled param that
  flows into a redirect — worth checking for open redirect"). Kept as a
  distinct phase so the deterministic recon output stays reliable and
  reproducible on its own, and the AI layer is clearly labeled as a
  hypothesis generator, not a finding.
- Rich terminal interface (progress, live findings feed)
- Strong test coverage, including fixture SPAs per supported framework so
  detection logic doesn't silently regress

---

## Design Principles

- Modular, plugin-based architecture
- Scope-gated and rate-limited by default; aggressive modes require opt-in
- Framework-aware analysis
- AST-first parsing
- Evidence-based confidence scoring on every finding (not binary yes/no)
- Resumable, diffable workspaces
- Rich terminal interface
- Comprehensive documentation, strong test coverage

---

## Suggested Roadmap

**Phase 1 (MVP):** Scope-gated crawler, JS collection, AST parsing, static
API inventory, basic framework detection, JSON/Markdown reports.

**Phase 2:** Playwright runtime observation, auth flow mapping, secrets
detection, dependency/CVE correlation, SQLite-backed workspaces.

**Phase 3:** Differential recon, interactive graph reports, prioritization
scoring, Burp/Postman/OpenAPI export.

**Phase 4:** Plugin SDK for third-party analyzers, optional LLM correlation
layer.

---

## Scope

NASIJ focuses on reconnaissance, analysis, and visualization. It does not
automate exploitation. Its role is to help researchers understand
applications quickly so they can perform informed manual security testing
within the bounds of authorized engagements.
