# AGENT.md

## Purpose
This repo is the Agentra frontend (SvelteKit + TypeScript). It is currently a client-rendered app (`ssr = false`) that talks directly to the backend HTTP API.

## Prerequisites
- Node.js (LTS recommended)
- npm (default in this repo)

## Run Locally
1. Install dependencies:
   - `npm install`
2. Configure env:
   - API base is read from `PUBLIC_API_URL` in `src/lib/api.ts`
   - Default fallback is `http://localhost:8080`
3. Start dev server:
   - `npm run dev`
4. Build check:
   - `npm run build`
   - `npm run check`

## Key Runtime Behavior
- Auth token is stored in `localStorage` key: `agentra_session`
- API helper: `src/lib/api.ts`
  - Adds `Authorization: Bearer <token>` when present
  - Use `api.postForm(...)` for multipart uploads
- Global auth state: `src/lib/auth.svelte.ts`

## Route Map (High Value)
- `src/routes/media/+page.svelte`: org media library + upload modal
- `src/routes/projects/[id]/+page.svelte`: project detail and scoped media view
- `src/routes/admin/*`: admin settings pages

## Media UX Contract (Current)
- Upload endpoint: `POST /media/upload`
- Upload modal is single-file only.
- Accepted file types in UI:
  - `.mov .mp4 .jpg .jpeg .png .wav .mp3`
- Media page groups assets chronologically by month/day.
- Project association is optional on media rows:
  - when `project_id` is null, label as global media.

## Styling and Component Notes
- Tailwind utility classes are used inline in Svelte components.
- Reusable media/auth helpers:
  - `src/lib/components/AuthenticatedImage.svelte`
  - `src/lib/components/VideoPlayer.svelte`
  - `src/lib/components/Modal.svelte`

## Coding Rules for Changes
- Keep this frontend thin: no backend business logic in client code.
- Prefer changing `src/lib/api.ts` once over ad-hoc fetch patterns.
- When backend request/response shapes change, update:
  - route-local types
  - API calls
  - empty/loading/error states
- Preserve media layout behavior:
  - consistent card row height
  - variable width by aspect ratio
  - chronological grouping retained

