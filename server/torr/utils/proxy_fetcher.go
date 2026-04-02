package utils

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"server/log"
)

// FetchRandomProxy loads a TXT list from the given URL and selects one random non-empty line.
// Lines starting with '#' are treated as comments and ignored.
func FetchRandomProxy(urlStr string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch proxy list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("proxy list URL returned HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read proxy list body: %w", err)
	}

	lines := strings.Split(string(bodyBytes), "\n")
	var validProxies []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		// Only keep non-empty, non-comment lines that look like valid URIs
		if l != "" && !strings.HasPrefix(l, "#") && strings.Contains(l, "://") && !strings.Contains(l, "<") {
			validProxies = append(validProxies, l)
		}
	}

	if len(validProxies) == 0 {
		return "", fmt.Errorf("no valid proxies found in list from %s", urlStr)
	}

	selected := validProxies[rand.IntN(len(validProxies))]

	log.TLogln("Fetched proxy list:", len(validProxies), "proxies available. Selected new proxy.")

	return selected, nil
}
