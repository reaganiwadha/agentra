# Agentra monorepo — Tiltfile
#
# Prerequisites:
#   - Go (1.21+)
#   - Bun
#   - Docker (for backend's postgres + minio dev containers)
#   - ffmpeg (for media thumbnails/processing)
#
# Usage: tilt up

local_resource(
    'backend',
    serve_cmd='cd agentra-backend && go run ./cmd/agentra',
    deps=[
        'agentra-backend/cmd',
        'agentra-backend/internal',
        'agentra-backend/rageditor',
        'agentra-backend/go.mod',
        'agentra-backend/go.sum',
    ],
    labels=['services'],
    links=[
        link('http://localhost:8080', 'API'),
    ],
)

local_resource(
    'client',
    serve_cmd='cd agentra-client && bun run dev',
    deps=[
        'agentra-client/src',
        'agentra-client/static',
        'agentra-client/svelte.config.js',
        'agentra-client/vite.config.ts',
        'agentra-client/tsconfig.json',
    ],
    labels=['services'],
    links=[
        link('http://localhost:5173', 'App'),
    ],
    resource_deps=['backend'],
)
