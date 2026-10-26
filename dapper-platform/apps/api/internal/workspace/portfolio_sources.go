package workspace

import (
	"embed"
	"encoding/json"
	"fmt"
)

// Existing client-site media, retained for an authorized publisher's reviewed
// migration. These files are never mounted in the anonymous public asset tree.
//
//go:embed portfolio_sources/*.jpg portfolio_sources/manifest.json
var portfolioSources embed.FS

type portfolioSourceImage struct {
	File         string `json:"file"`
	Title        string `json:"title"`
	Alt          string `json:"alt"`
	Source       string `json:"source"`
	SourceSHA256 string `json:"sourceSha256"`
	LocalSHA256  string `json:"localSha256"`
}
type portfolioSourceManifest struct {
	Images []portfolioSourceImage `json:"images"`
}

func sourcePortfolioManifest() (portfolioSourceManifest, error) {
	data, err := portfolioSources.ReadFile("portfolio_sources/manifest.json")
	if err != nil {
		return portfolioSourceManifest{}, err
	}
	var manifest portfolioSourceManifest
	err = json.Unmarshal(data, &manifest)
	return manifest, err
}
func sourcePortfolioImage(name string) ([]byte, error) {
	manifest, err := sourcePortfolioManifest()
	if err != nil {
		return nil, err
	}
	for _, image := range manifest.Images {
		if image.File == name {
			return portfolioSources.ReadFile("portfolio_sources/" + image.File)
		}
	}
	return nil, fmt.Errorf("unknown portfolio source")
}
