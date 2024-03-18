package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Scalingo/go-utils/logger"
	"github.com/Scalingo/sclng-backend-test-v1/github"
)

func initStatsWorkers(ctx context.Context, count int) {
	log := logger.Get(ctx)

	// TODO workerCount in conf
	// and some thinking about how many workers we need
	for i := 0; i < 16; i++ {
		go func(i int) {
			log.Debugf("worker stats %d started", i)

			// you'd want to know if one of thoses stops unexpectedly
			defer log.Errorf("worker stats %d exited", i)

			startWorkerStats(ctx)
		}(i)
	}
}

type WorkerStats struct {
	Stats Stats
	Err   error
}

type WorkerStatsTask struct {
	auth       Authorization
	params     url.Values
	repository github.Repository
	stats      chan<- WorkerStats
}

// tasks to pool from for the workers
// limit to 1000 just to set a limit but the actual limit would require some thinking
var workerStatsTasks = make(chan WorkerStatsTask, 1000)

type WorkerDiscardRepository struct{}

func (w WorkerDiscardRepository) Error() string {
	return "discard repository"
}

func startWorkerStats(ctx context.Context) {
	for task := range workerStatsTasks {
		httpRequest := HttpRequest{
			Method: http.MethodGet,
			Headers: map[string]string{
				"Accept": "application/vnd.github.v3+json",
			},
		}

		if task.auth.Token != "" {
			httpRequest.Headers["Authorization"] = task.auth.Token
		}

		// fetch repository
		// we do so to get the repository's license, stars count
		httpRequest.Url = task.repository.Url

		var repository github.Repository

		_, err := httpRequest.Do(ctx, &repository)
		if err != nil {
			task.stats <- WorkerStats{
				Err: fmt.Errorf("failed to fetch repository: %w", err),
			}

			continue
		}

		// fetch languages
		httpRequest.Url = task.repository.LanguagesUrl

		var languages map[string]int

		_, err = httpRequest.Do(ctx, &languages)
		if err != nil {
			task.stats <- WorkerStats{
				Err: fmt.Errorf("failed to fetch languages: %w", err),
			}

			continue
		}

		// filters out repositories based on the query parameters
		license := task.params.Get("license")
		if license != "" && repository.License.Key != license {
			task.stats <- WorkerStats{
				Err: fmt.Errorf("wrong license `%s`: %w", repository.License.Key, WorkerDiscardRepository{}),
			}

			continue
		}

		language := task.params.Get("language")
		if language != "" {
			if _, ok := languages[language]; !ok {
				task.stats <- WorkerStats{
					Err: fmt.Errorf("wrong language `%v`: %w", languages, WorkerDiscardRepository{}),
				}

				continue
			}
		}

		// send back the repository stats
		task.stats <- WorkerStats{
			Stats: Stats{
				Repo: Repo{
					Url:         task.repository.Url,
					Name:        task.repository.Name,
					Owner:       task.repository.Owner.Login,
					Description: task.repository.Description,
				},
				StarCount: repository.StargazersCount,
				Languages: languages,
			},
		}
	}
}

// fetch the repositories stats
type Stats struct {
	Repo
	StarCount int            `json:"stars_count"`
	Languages map[string]int `json:"languages"`
	License   string         `json:"license"`
}

func fetchStats(ctx context.Context, params url.Values) ([]Stats, error) {
	log := logger.Get(ctx)

	stats := make(chan WorkerStats)
	defer close(stats)

	repositories, err := fetchGithubRepositories(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetchGithubRepositories failed: %w", err)
	}

	auth, _ := ctx.Value(Authorization{}).(Authorization)

	for _, repository := range repositories {
		workerStatsTasks <- WorkerStatsTask{
			auth:       auth,
			params:     params,
			repository: repository,
			stats:      stats,
		}
	}

	results := make([]Stats, 0, len(repositories))

	eventCount := len(repositories)

	for eventCount > 0 {
		select {
		case stat := <-stats:
			eventCount -= 1

			if stat.Err != nil {
				if errors.Is(stat.Err, WorkerDiscardRepository{}) {
					log.Debug(stat.Err.Error())
				} else {
					log.Warnf("error fetching stats: %w", stat.Err)
				}
				break
			}

			results = append(results, stat.Stats)
		}
	}

	return results, nil
}
