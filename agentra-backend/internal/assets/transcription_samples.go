package assets

import "embed"

//go:embed analyzer_tests/transcription/samples.json
var analyzerTestAssets embed.FS

func TranscriptionSampleManifest() ([]byte, error) {
	return analyzerTestAssets.ReadFile("analyzer_tests/transcription/samples.json")
}
