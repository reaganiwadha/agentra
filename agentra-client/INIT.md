# Agentra Web App — SvelteKit Requirements

## Goals

The SvelteKit app must be:

* Thin client over Agentra HTTP API
* Server-first (use SvelteKit server routes where possible)
* Minimal client state
* No business logic
* No media processing
* No direct LLM calls
* Replaceable without affecting backend

---

# Architecture Principles (SvelteKit)

## 1. Use Server Load Functions First

Prefer:

* `+page.server.ts`
* `+layout.server.ts`
* `+server.ts`

Avoid client fetches unless necessary.

Reasons:

* Centralized API calls
* Automatic auth handling
* No CORS complexity
* Cleaner code

---

## 2. Session Handling

### Storage

Session token stored in:

**HTTP-only cookie**

Name:

```
agentra_session
```

Why:

* Secure
* Automatic inclusion
* No localStorage

---

## 3. Global API Client

Create:

```
src/lib/server/api.ts
```

Responsibilities:

* Read session cookie
* Add header:

```
Authorization: Bearer <token>
```

* Proxy requests to backend

Environment variable:

```
AGENTRA_API_URL=http://localhost:8080
```

All server load functions must use this helper.

---

# Project Structure

```
src/
  routes/
    +layout.server.ts
    +layout.svelte

    setup/
    login/
    logout/

    dashboard/

    projects/
      +page.svelte
      [id]/

    jobs/
      [id]/

    admin/
      models/
      storage/
      editor/

  lib/
    server/api.ts
    components/
      VideoPlayer.svelte
      StatusBadge.svelte
```

---

# Root Layout

## +layout.server.ts

Responsibilities:

* Read session cookie
* Call `/me` (or equivalent)
* Provide user info to all pages

If unauthorized:

* Redirect to `/login`

Except:

* `/login`
* `/setup`

---

## +layout.svelte

Display:

* Top navigation:

  * Dashboard
  * Projects
  * Admin (if role=admin)
  * Logout

---

# Routes & Views

---

# 1. Setup

### Route

`/setup`

Files:

```
setup/+page.svelte
setup/+page.server.ts
```

Behavior:

Form:

* token
* email
* password

Action:
POST `/setup`

If success:
→ redirect `/login`

If any admin exists:
Backend returns 403 → show message

No client-side logic.

---

# 2. Login

### Route

`/login`

Files:

```
login/+page.svelte
login/+page.server.ts
```

Form:

* email
* password

Action:

1. POST `/login`
2. Get session token
3. Set cookie:

```
agentra_session
httpOnly: true
secure: true (if HTTPS)
sameSite: lax
```

Redirect → `/dashboard`

---

# 3. Logout

### Route

`/logout/+server.ts`

Behavior:

* Delete cookie
* POST `/logout`
* Redirect `/login`

---

# 4. Dashboard

### Route

`/dashboard`

Files:

```
dashboard/+page.server.ts
dashboard/+page.svelte
```

Load data:

* GET /projects
* (or summary endpoint if added)

Display:

* Project count
* Asset counts by status
* Job counts by status

No interactivity required.

---

# 5. Projects

---

## 5.1 Project List

Route:
`/projects`

Load:
GET /projects

Display:

* Name
* Description
* Status

Button:
Create Project

Form action:
POST /projects

---

## 5.2 Project Detail

Route:
`/projects/[id]`

Files:

```
+page.server.ts
+page.svelte
```

Load:

* GET /projects/:id/media
* GET recent jobs (optional)

Layout:

### Left Panel — Media List

For each asset:

* Thumbnail
  `/media-assets/:id/thumbnail`
* Filename
* Status badge

Click:
Open modal

Modal contains:

* `<VideoPlayer>`
  `/media-assets/:id/stream`
* Transcript
* Vision tags

---

### Right Panel — Highlight Creator

Form:

Textarea: prompt

Action:
POST `/projects/:id/highlights`

After submit:
Invalidate page data

---

### Jobs List

Show:

* Status
* Created time

Click → `/jobs/:id`

---

# 6. Job Detail

Route:
`/jobs/[id]`

Load:
GET `/jobs/:id`

If status != completed:

Client-side polling:
Every **5 seconds**:

```
invalidate('job')
```

Display:

* Prompt
* Status
* Error (if failed)

If completed:

Video:
`/renders/:id/stream`

Thumbnail:
`/renders/:id/thumbnail`

---

# 7. Admin Section

Visible only if:

```
data.user.role === 'admin'
```

---

## 7.1 Models

Route:
`/admin/models`

Load:
GET /models

Form per usage_type:

PUT /models/:usage_type

---

## 7.2 Storage

Route:
`/admin/storage`

Load:
GET /storage

Submit:
PUT /storage

Show warning:
“Changing storage does not migrate existing media”

---

## 7.3 Editor Config

Route:
`/admin/editor`

Load:
GET editor config

Submit:
PUT editor config

Fields:

* base_prompt
* max_duration
* autonomous_enabled

---

# Components

---

## VideoPlayer.svelte

Simple wrapper:

```
<video controls preload="metadata">
  <source src={url} />
</video>
```

Must rely on HTTP range.

---

## StatusBadge.svelte

Inputs:

* status

Colors:

pending → gray
analyzing → blue
ready → green
failed → red
queued → gray
running → blue
completed → green

---

# Polling Strategy

Only for:

* Job detail page

Interval: 5 seconds

Use:

```
import { invalidate } from '$app/navigation'
```

No WebSockets.

---

# Error Handling

Server actions must:

* Catch backend errors
* Return:

```
fail(status, { message })
```

UI shows message.

---

# Environment Variables

Required:

```
AGENTRA_API_URL
```

Optional:

```
PUBLIC_APP_NAME=Agentra
```

---

# What NOT To Implement

Do NOT implement:

* Global state stores
* WebSockets
* Real-time editing
* Timeline UI
* File uploads
* Client-side caching layer
* API SDK abstraction layer

Keep it thin.

---

# MVP Navigation Flow

```
/setup (first run)
→ /login
→ /dashboard
→ /projects
→ /projects/[id]
→ create highlight
→ /jobs/[id]
→ watch render
```

---

# Complexity Target

The entire UI should be achievable in:

**~2–3k lines of code**

If it grows beyond that, scope is drifting.

---

If you want the **highest-leverage next step**, I’d suggest:

* Exact **SvelteKit folder tree (copy-paste ready)**
  or
* A **minimal API contract (JSON shapes)**
  or
* A **Phase-2 plan** (interactive editor without breaking this architecture)

You’re very close to a clean, thesis-quality full stack system now.
