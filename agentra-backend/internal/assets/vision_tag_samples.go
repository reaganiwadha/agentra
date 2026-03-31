package assets

import "embed"

//go:embed analyzer_tests/vision_tags/samples.json
var visionTagAnalyzerTestAssets embed.FS

func VisionTagSampleManifest() ([]byte, error) {
	return visionTagAnalyzerTestAssets.ReadFile("analyzer_tests/vision_tags/samples.json")
}
