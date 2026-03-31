Transcription analyzer test assets are defined in `samples.json`.

Each sample points at a public GitHub-hosted audio file (`.wav` or `.mp3`) so
tests can run without committing binary media into this repository.

Schema:
- `id`: stable identifier used in logs and summaries.
- `name`: label shown in test progress events.
- `url`: raw audio file URL.
- `expected_contains`: optional phrases used for basic result sanity checks.
