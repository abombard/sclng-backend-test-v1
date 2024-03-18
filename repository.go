package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Scalingo/go-utils/logger"
	"github.com/Scalingo/sclng-backend-test-v1/github"
)

func fetchGithubRepositories(ctx context.Context, params url.Values) ([]github.Repository, error) {
	log := logger.Get(ctx)

	httpRequest := HttpRequest{
		Method: http.MethodGet,
		Url:    "https://api.github.com/repositories",
		Headers: map[string]string{
			"Accept": "application/vnd.github.v3+json",
		},
	}

	if auth, ok := ctx.Value(Authorization{}).(Authorization); ok && auth.Token != "" {
		httpRequest.Headers["Authorization"] = auth.Token
	}

	var repositories []github.Repository

	// just a quick param to fetch 100 repository that are not the last created
	if sinceIdParam := params.Get("since"); sinceIdParam != "" {
		httpRequest.Query = map[string]string{"since": sinceIdParam}

		_, err := httpRequest.Do(ctx, &repositories)

		return repositories, err
	}

	// Find the last 100 repositories created
	//
	// Since the API won't tell us if we have the last page
	// we look for a call that returns 99 repositories
	// then decrement until we find that last one repository missing
	//
	// the algorithm is basically: find a low and a high bound
	// (low: 100 repositories returned, high: less than 100 repositories returned)
	// then get closer to the correct id by increments of (high - low) / 2
	sinceIdPrev := 1
	sinceId := 10000000000

	var tryCount int
	for tryCount = 0; ; tryCount++ {
		httpRequest.Query = map[string]string{"since": strconv.Itoa(sinceId)}

		repositories = []github.Repository{}

		_, err := httpRequest.Do(ctx, &repositories)
		if err != nil {
			return nil, err
		}

		diff := sinceIdPrev - sinceId
		if diff < 0 {
			diff = -diff
		} else if diff == 0 {
			// just so we don't loop infinitely on the same id
			diff = 2
		}

		sinceIdPrev = sinceId

		if len(repositories) < 99 {
			sinceId = sinceId - (diff / 2)
		} else if len(repositories) > 99 {
			sinceId = sinceId + (diff / 2)
		} else {
			break
		}

		log.Debugf("tryCount %d sinceIdPrev %d sinceId %d len(results) %d", tryCount, sinceIdPrev, sinceId, len(repositories))
	}

	// at this point we just have to find that one more repository missing
	for ; len(repositories) < 100; tryCount++ {
		sinceId -= 1

		httpRequest.Query = map[string]string{"since": strconv.Itoa(sinceId)}

		_, err := httpRequest.Do(ctx, &repositories)
		if err != nil {
			return nil, fmt.Errorf("list github repositories failed: %w", err)
		}
	}

	log.Infof("took %d calls to find the last 100 repositories", tryCount)

	return repositories, nil
}

type Repo struct {
	Name        string `json:"name"`
	Url         string `json:"url"`
	Owner       string `json:"owner"`
	Description string `json:"description"`
}

func fetchRepositories(ctx context.Context, params url.Values) ([]Repo, error) {
	repositories, err := fetchGithubRepositories(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetchGithubRepositories failed: %w", err)
	}

	results := make([]Repo, 0, len(repositories))

	for _, repository := range repositories {
		results = append(results, Repo{
			Url:         repository.Url,
			Name:        repository.Name,
			Owner:       repository.Owner.Login,
			Description: repository.Description,
		})
	}

	return results, err
}
