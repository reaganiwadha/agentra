package assets

import "embed"

//go:embed analyzer_tests/embedding/samples.json
var embeddingAnalyzerTestAssets embed.FS

func EmbeddingSampleManifest() ([]byte, error) {
	return embeddingAnalyzerTestAssets.ReadFile("analyzer_tests/embedding/samples.json")
}
