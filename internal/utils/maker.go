package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// CreateFlamingImage calls the API cuki.biz.id to generate a flaming text image
func CreateFlamingImage(text string, style string) ([]byte, error) {
	apiUrl := fmt.Sprintf("https://api.cuki.biz.id/api/flaming/flaming%s?apikey=cuki-x&text=%s", style, url.QueryEscape(text))

	resp, err := http.Get(apiUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from flaming api: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flaming api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read flaming api response: %v", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("flaming api returned empty response")
	}

	return body, nil
}
