# Agentra monorepo - Tiltfile
#
# Prerequisites:
#   - Go (1.21+)
#   - Bun
#   - Docker
#   - ffmpeg (for media thumbnails/processing)
#
# Usage: tilt up

POSTGRES_USER = 'postgres'
POSTGRES_PASSWORD = 'postgres'
POSTGRES_DB = 'agentra'
POSTGRES_PORT = '5432'
POSTGRES_ENV = 'POSTGRES_HOST=localhost POSTGRES_PORT=%s POSTGRES_USER=%s POSTGRES_PASSWORD=%s POSTGRES_DB=%s POSTGRES_SSLMODE=disable' % (
    POSTGRES_PORT,
    POSTGRES_USER,
    POSTGRES_PASSWORD,
    POSTGRES_DB,
)
POSTGRES_ENV_BAT = 'set "POSTGRES_HOST=localhost" && set "POSTGRES_PORT=%s" && set "POSTGRES_USER=%s" && set "POSTGRES_PASSWORD=%s" && set "POSTGRES_DB=%s" && set "POSTGRES_SSLMODE=disable"' % (
    POSTGRES_PORT,
    POSTGRES_USER,
    POSTGRES_PASSWORD,
    POSTGRES_DB,
)
CLIENT_ENV_VARS = {
    'PUBLIC_API_URL': 'http://localhost:8080',
    'PUBLIC_APP_NAME': 'Agentra',
}

docker_compose(encode_yaml({
    'services': {
        'postgres': {
            'image': 'pgvector/pgvector:pg16',
            'environment': {
                'POSTGRES_USER': POSTGRES_USER,
                'POSTGRES_PASSWORD': POSTGRES_PASSWORD,
                'POSTGRES_DB': POSTGRES_DB,
            },
            'ports': ['%s:5432' % POSTGRES_PORT],
            'volumes': ['agentra-postgres-data:/var/lib/postgresql/data'],
            'healthcheck': {
                'test': ['CMD-SHELL', 'pg_isready -U %s -d %s' % (POSTGRES_USER, POSTGRES_DB)],
                'interval': '2s',
                'timeout': '5s',
                'retries': 20,
            },
        },
    },
    'volumes': {
        'agentra-postgres-data': {},
    },
}))

dc_resource('postgres', labels=['infra'])

local_resource(
    'client-deps',
    cmd='cd agentra-client && bun install --frozen-lockfile',
    cmd_bat='cd agentra-client && bun install --frozen-lockfile',
    deps=[
        'agentra-client/package.json',
        'agentra-client/bun.lock',
    ],
    labels=['setup'],
)

local_resource(
    'backend',
    serve_cmd='cd agentra-backend && DOCKERTEST=0 %s PORT=8080 CORS_ORIGINS="http://localhost:5173" go run ./cmd/agentra' % POSTGRES_ENV,
    serve_cmd_bat='cd agentra-backend && set "DOCKERTEST=0" && %s && set "PORT=8080" && set "CORS_ORIGINS=http://localhost:5173" && go run ./cmd/agentra' % POSTGRES_ENV_BAT,
    deps=[
        'agentra-backend/cmd',
        'agentra-backend/internal',
        'agentra-backend/rageditor',
        'agentra-backend/go.mod',
        'agentra-backend/go.sum',
    ],
    resource_deps=['postgres'],
    labels=['services'],
    links=[
        link('http://localhost:8080', 'API'),
    ],
)

local_resource(
    'client',
    serve_cmd='cd agentra-client && bun run dev',
    serve_cmd_bat='cd agentra-client && bun run dev',
    serve_env=CLIENT_ENV_VARS,
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
    resource_deps=['client-deps', 'backend'],
)
