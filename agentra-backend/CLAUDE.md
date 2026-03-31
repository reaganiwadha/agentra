# Agentra Backend Scaffolding Guide

Hello Claude/Codex. Let's scaffold **Agentra**.

Agentra is an always-online video editing agent that runs self-hosted inside a media enterprise environment.

For now, Agentra is in research, with the primary use case:

> **AgenTRIM** — a locally-hosted agentic video processing system that automatically produces domain-relevant highlight reels from unstructured event footage using multimodal analysis and LLM-orchestrated editing.

This repository builds the **backend platform for Agentra**, while the functional context is AgenTRIM.

The goal is a **minimal, deterministic, production-style system** suitable for research and real enterprise deployment.

---

# Technology Stack

Use the following libraries:

* PostgreSQL
* errorx
* null
* goqu
* sqlx
* gin
* goose
* gocron
* fx
* logrus
* go-smb2
* langchaingo
* resty

Architecture:

**Clean Architecture with Hexagonal patterns**

---

# Design Principles

1. Single organization (but explicitly modeled)
2. Self-hosted deployment
3. Simplicity over flexibility
4. Deterministic workflows
5. LLM assists processing but does not control system state
6. All long-running work is database-driven via job loops
7. No premature multi-tenancy or enterprise features

---

# Core Concepts

Agentra has three main subsystems:

1. Identity & Configuration
2. Media Intelligence (Analyzer)
3. Video Generation (Editor)

---

# Bootstrap Behavior (In-Memory)

On application startup:

1. Count admin users
2. If count == 0:

    * Generate random setup token
    * Store token **in memory**
    * Print:

```
=== Agentra First Setup ===
Admin setup token: XXXXX
Visit POST /setup to create first admin
```

Endpoint:

POST /setup

Input:

* token
* email
* password

If valid:

* Create default organization (if none exists)
* Create admin user
* Clear bootstrap token

If any admin exists:
→ `/setup` returns 403

No bootstrap table is used.

---

# Organization & Identity

## organizations

* id (UUID)
* name
* slug
* is_active
* created_at

Only one organization is expected.

---

## users

* id
* organization_id
* email (unique per org)
* password_hash
* role: admin | editor | viewer
* is_active
* created_at
* updated_at

Flat ACL only.

---

# Storage System

Agentra supports two storage types:

* SMB (NAS)
* MinIO (S3-compatible)

Only one active storage per organization.

## storage_configs

* id
* organization_id
* storage_type (smb | minio)
* endpoint
* access_key
* secret_key
* bucket (for MinIO)
* base_path — root path where source footage is discovered
* output_base_path — root path where rendered highlight reels are written
* is_active
* created_at
* updated_at

All media access must go through a storage interface.

`base_path` and `output_base_path` are both configurable by the admin.
They may point to different directories on the same storage backend.

---

# Media Serving

Agentra proxies media files directly through the HTTP server using range requests
(HTTP 206 Partial Content), making it compatible with HTML `<video>` players and
enabling direct file downloads. Both source assets and rendered outputs use this pattern.

MinIO presigned URL redirects are a future optimization — not implemented now.

## Thumbnails

Generated on-demand using ffmpeg (must be available on the server).
A single frame is extracted at the 5-second mark and served as JPEG.
No persistent thumbnail storage. No caching for now (can be added later).

ffmpeg is a hard system dependency.

## Endpoints

### Thumbnail (on-demand)

GET /media-assets/:id/thumbnail   → JPEG, extracted at ~5s mark
GET /renders/:id/thumbnail        → JPEG, extracted at ~5s mark

### Stream / Download

GET /media-assets/:id/stream      → range-request compatible, serves source file
GET /renders/:id/stream           → range-request compatible, serves rendered file

The `stream` endpoints set appropriate Content-Type and Content-Disposition headers
so browsers can both play inline (video player) and download.

---

# Projects

## projects

* id
* organization_id
* name
* description
* status (active | archived)
* created_by
* created_at
* updated_at

Each project represents a media domain.

---

# Media Assets

## media_assets

Represents files discovered in storage.

* id
* project_id
* filename
* storage_path
* sha256
* duration_sec
* file_size_bytes
* status:

    * pending
    * analyzing
    * ready
    * failed
* created_at
* updated_at

Unique: (project_id, sha256)

---

# Media Analysis

## media_analysis

* id
* media_id
* transcript (JSONB)
* vision_tags (JSONB)
* embedding (JSONB or vector)
* analysis_model
* analyzed_at

---

# Model Configuration

Models are admin-configured and stored in plaintext.

## model_configs

* id
* organization_id
* name
* base_url
* api_key
* model_name
* usage_type:

    * chat_editing
    * vision_analysis
    * transcription
    * embedding
* is_active
* created_at
* updated_at

Unique: (organization_id, usage_type)

All external models must be treated as **OpenAI-compatible HTTP endpoints**.

---

# Editor Configuration

## editor_configs

* id
* organization_id
* base_prompt (TEXT)
* max_duration_sec
* is_autonomous_enabled
* created_at
* updated_at

Only one active config per organization.

Base prompt is used as SYSTEM message for all editing jobs.

---

# Highlight Jobs

## highlight_jobs

* id
* project_id
* requested_by
* prompt
* status:

    * queued
    * running
    * completed
    * failed
* error_message
* started_at
* finished_at
* created_at

---

## timelines

* id
* job_id
* otio_json
* created_at

---

## renders

* id
* job_id
* output_path
* duration_sec
* file_size_bytes
* created_at

---

# Event Loops

Agentra runs two independent background loops.

---

## 1. Analyzer Loop

Cron: every 30 seconds

Purpose:
Ensure all media assets are analyzed.

Process:

Select up to 5 assets at a time:

```
media_assets.status = 'pending'
ORDER BY created_at ASC
FOR UPDATE SKIP LOCKED
```

Pipeline per asset (all ffmpeg calls use os/exec, temp files in os.TempDir()):

1. Mark `analyzing`
2. Fetch video file from storage adapter → temp file
3. Compute SHA256 of the downloaded file, store on media_asset
4. `ffmpeg -vn` → extract audio as mono 16kHz MP3
5. POST audio to transcription model → `verbose_json` transcript (text + segments with timestamps)
6. `ffmpeg select='gt(scene,0.4)'` → extract keyframes as JPEGs, cap at 10 frames
7. POST keyframes + prompt to vision model → `{ "tags": [...], "description": "..." }` (JSON only)
8. Combine: `"Transcript: <text>\n\nTags: <tags>"` → POST to embeddings model → float64 vector
9. Insert `media_analysis` (transcript JSONB, vision_tags JSONB, embedding JSONB)
10. Mark `ready`
11. Clean up all temp files

On any step failure: mark `failed`, continue to next asset.

HTTP client: **resty** for all three API calls (transcription, vision, embedding).
All model endpoints are OpenAI-compatible. langchaingo is NOT used in the analyzer.
langchaingo is reserved for the rageditor/editor pipeline.

ffmpeg is a hard system dependency (also used for thumbnails).

Important:

* Analyzer is NOT a RAG system
* It is a deterministic data preparation pipeline
* Large backlogs are processed incrementally (5 assets per tick)

---

## 2. Editor Loop

Cron: every 30 seconds

Purpose:
Process highlight_jobs.

Process:

Select:

```
highlight_jobs.status = 'queued'
FOR UPDATE SKIP LOCKED
```

Steps:

1. Mark running
2. Retrieve relevant clips using:

    * transcript search
    * embeddings
3. Build context
4. Call ragEDITOR pipeline
5. Save timeline
6. Save render metadata
7. Mark completed

Important:

* Editor does NOT wait for full archive analysis
* It only uses media where status = ready
* Works with partial datasets

---

# Editor Modes

There is **one editor engine**.

Two trigger types:

### Interactive (primary)

User submits prompt → creates highlight_job

### Autonomous (optional)

If enabled in editor_config:
Cron may generate scheduled jobs

Do NOT implement multiple editor systems.

---

# Authentication

Session-based auth. Sessions are stored in a `sessions` database table.

## sessions

* id (UUID, also used as the session token)
* user_id
* expires_at
* created_at

Clients pass the session token via `Authorization: Bearer <token>` header.
Session tokens do not expire automatically for now (no TTL enforcement needed yet).

## Auth Endpoints

POST /login

Input:
* email
* password

Output:
* session token

POST /logout (authenticated)

---

# HTTP API (Minimal)

### Setup

POST /setup

### Auth

POST /login
POST /logout

### Users

POST /users

### Projects

GET /projects
POST /projects

### Media

GET /projects/:id/media
GET /media-assets/:id/thumbnail
GET /media-assets/:id/stream

### Highlight Jobs

POST /projects/:id/highlights
GET /jobs/:id

### Renders

GET /renders/:id/thumbnail
GET /renders/:id/stream

### Model Config

GET /models
PUT /models/:usage_type

### Storage Config

GET /storage
PUT /storage

---

# Background Jobs Summary

| Job                | Interval |
| ------------------ | -------- |
| Analyzer           | 30s      |
| Editor             | 30s      |
| Storage scanner    | 5m       |
| Stuck job recovery | daily    |

Storage scanner walks the active storage backend (SMB or MinIO), discovers video files, and upserts `media_assets` rows (keyed on sha256 per project). It does NOT analyze — it only creates `pending` records for the analyzer loop to pick up.

---

# Clean Architecture Layout

```
/cmd/agentra

/internal
    /domain
    /usecase
    /repository/postgres
    /adapter/http
    /adapter/storage
    /adapter/llm
    /adapter/scheduler
    /config
    
/rageditor
```

Rules:

* Domain has no external dependencies
* Usecases depend on interfaces
* Adapters implement interfaces
* fx wires dependencies

---

# Guardrails (Important)

Do NOT implement:

* Multi-organization logic
* RBAC
* WebSockets
* Real-time editing
* Distributed workers
* Social media publishing
* Autonomous content generation loops
* Workflow builders
* Model training

Agentra is:

> A deterministic self-hosted video processing backend with LLM-assisted editing.

---

# Success Criteria

System should:

1. Start with empty DB
2. Print setup token
3. Create admin
4. Configure storage and models
5. Ingest media
6. Analyze media
7. Accept prompt
8. Produce highlight job
9. Store timeline and render metadata

FOR ANY EDITING, LET's just output an empty 30 second black video. User wants to be hands on when making the editing system, so right now just focus on building the backend.
ragEDITOR is a local Go package at /rageditor (imported as agentra/rageditor). For now, it only exposes a function that generates a 30 second black video. Do not build out ragEDITOR internals — user will do that themselves.

# Code Style

* Minimal comments — code should tell its own story
* Only comment where logic is genuinely non-obvious
* No doc comments on every function/type
* Self-documenting naming over explanatory comments

# Build Approach

Incremental. Phases:
1. Foundation — config, DB connection, domain models, goose migrations, fx bootstrap
2. Identity — org, users, sessions, setup endpoint, login/logout
3. Configuration — storage_configs, model_configs, editor_configs
4. Media — projects, media_assets, storage adapter, NAS scanner loop
5. Analysis — media_analysis, analyzer loop, LLM adapters
6. Editing — highlight_jobs, timelines, renders, editor loop, rageditor stub

Present each phase for review before proceeding to the next.
