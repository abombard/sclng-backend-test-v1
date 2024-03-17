package github

import (
	"net/http"
	"strings"
)

// Github Pagination
func fetchResponseLinks(res *http.Response) map[string]string {
	header := res.Header.Get("link")
	if header == "" {
		return map[string]string{}
	}

	links := map[string]string{}

	for _, link := range strings.Split(header, ",") {
		link = strings.TrimSpace(link)

		parts := strings.Split(link, ";")

		key := parts[1][6 : len(parts[1])-1]
		val := parts[0][1 : len(parts[0])-1]

		links[key] = val
	}

	return links
}
