package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Scalingo/go-handlers"
	"github.com/Scalingo/go-utils/logger"
)

func main() {
	log := logger.Default()
	log.Info("Initializing app")
	cfg, err := newConfig()
	if err != nil {
		log.WithError(err).Error("Fail to initialize configuration")
		os.Exit(1)
	}

	// start workers
	// this context could handle signal
	// it would be nice to finish the current requests before shutting down
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		go func(i int) {
			log.Infof("worker repo %d started", i)
			defer log.Infof("worker repo %d exited", i)

			startWorkerRepo(ctx)
		}(i)
	}

	for i := 0; i < 10; i++ {
		go func(i int) {
			log.Infof("worker stats %d started", i)
			defer log.Infof("worker stats %d exited", i)

			startWorkerStats(ctx)
		}(i)
	}

	log.Info("Initializing routes")
	router := handlers.NewRouter(log)
	router.HandleFunc("/ping", pongHandler)
	// Initialize web server and configure the following routes:
	// GET /repos
	// GET /stats

	typeRegexp := "{CreateEvent|WatchEvent|PushEvent}"
	reftypeRegexp := "{[a-z]*?}"

	router.HandleFunc("/repos", reposHandlerGet).Methods(http.MethodGet).Queries(
		"type", typeRegexp, "reftype", reftypeRegexp,
	)

	router.HandleFunc("/stats", statsHandlerGet).Methods(http.MethodGet).Queries(
		"type", typeRegexp, "reftype", reftypeRegexp,
	)

	log = log.WithField("port", cfg.Port)
	log.Info("Listening...")
	err = http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), router)
	if err != nil {
		log.WithError(err).Error("Fail to listen to the given port")
		os.Exit(2)
	}
}

func pongHandler(w http.ResponseWriter, r *http.Request, _ map[string]string) error {
	log := logger.Get(r.Context())
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(map[string]string{"status": "pong"})
	if err != nil {
		log.WithError(err).Error("Fail to encode JSON")
	}
	return nil
}

// workers

// WorkerRepository
type WorkerRepository struct {
	Repository GithubRepository
	Err        error
}

// WorkerRepoTask
type WorkerRepoTask struct {
	auth         Authorization
	event        GithubEvent
	filters      url.Values
	repositories chan<- WorkerRepository
}

var workerRepoTasks = make(chan WorkerRepoTask, 1000)

func startWorkerRepo(ctx context.Context) {
	for task := range workerRepoTasks {
		httpRequest := HttpRequest{
			Method: http.MethodGet,
			Url:    task.event.Repo.Url,
			Headers: map[string]string{
				"Accept": "application/vnd.github.v3+json",
			},
		}

		// FIXME how do you handle auth properly ?
		if task.auth.Token != "" {
			httpRequest.Headers["Authorization"] = task.auth.Token
		}

		var repository GithubRepository

		err := httpRequest.Do(ctx, &repository)
		if err == nil && !wantsRepo(ctx, repository, task.filters) {
			err = fmt.Errorf("discarding repository")
		}

		task.repositories <- WorkerRepository{
			Repository: repository,
			Err:        err,
		}
	}
}

// worker stats
type WorkerStats struct {
	Stats Stats
	Err   error
}

type WorkerStatsTask struct {
	auth       Authorization
	repository GithubRepository
	stats      chan<- WorkerStats
}

var workerStatsTasks = make(chan WorkerStatsTask, 1000)

func startWorkerStats(ctx context.Context) {
	for task := range workerStatsTasks {
		httpRequest := HttpRequest{
			Method: http.MethodGet,
			Url:    task.repository.LanguagesUrl,
			Headers: map[string]string{
				"Accept": "application/vnd.github.v3+json",
			},
		}

		if task.auth.Token != "" {
			httpRequest.Headers["Authorization"] = task.auth.Token
		}

		var languages map[string]string

		err := httpRequest.Do(ctx, &languages)

		task.stats <- WorkerStats{
			Stats: Stats{
				Repo: Repo{
					Url:         task.repository.Url,
					Name:        task.repository.Name,
					Owner:       task.repository.Owner.Login,
					Description: task.repository.Description,
				},
				StarCount: task.repository.StargazersCount,
				Languages: languages,
			},
			Err: err,
		}
	}
}

// fetch github events and send them to onEvent until onEvent returns false
func fetchGithubEvents(ctx context.Context, eventCount int) ([]GithubEvent, error) {
	httpRequest := HttpRequest{
		Method: http.MethodGet,
		Url:    "https://api.github.com/events",
		Headers: map[string]string{
			"Accept": "application/vnd.github.v3+json",
		},
		Query: map[string]string{
			"per_page": strconv.Itoa(eventCount),
		},
	}

	if auth, ok := ctx.Value(Authorization{}).(Authorization); ok && auth.Token != "" {
		httpRequest.Headers["Authorization"] = auth.Token
	}

	logger.Get(ctx).Debug("fetching ", httpRequest.Query["per_page"], " events ...")

	var events []GithubEvent

	err := httpRequest.Do(ctx, &events)
	if err != nil {
		return nil, fmt.Errorf("httpRequest.Do failed: %w", err)
	}

	return events, err
}

// repo
func wantsRepo(ctx context.Context, repository GithubRepository, filters url.Values) bool {
	if language := filters.Get("language"); language != "" {
		if repository.Language != language {
			return false
		}
	}

	if license := filters.Get("license"); license != "" {
		if repository.License.Key != license {
			return false
		}
	}

	return true
}

type Repo struct {
	Name        string `json:"name"`
	Url         string `json:"url"`
	Owner       string `json:"owner"`
	Description string `json:"description"`
	// TODO remove
	LanguagesUrl string `json:"languages_url"`
	Language     string `json:"languages"`
}

func dispatchWorkerRepoTasks(
	ctx context.Context,
	auth Authorization,
	params url.Values,
	repositories chan<- WorkerRepository,
) (int, error) {
	typ := params.Get("type")
	reftyp := params.Get("reftype")

	// can't fetch more than 100 events right now
	// use github link API to fetch more
	eventCount := 100

	events, err := fetchGithubEvents(ctx, eventCount)
	if err != nil {
		return 0, fmt.Errorf("fetchGithubEvents failed: %w", err)
	}

	eventWorkerCount := 0

	for _, event := range events {
		if event.Type != typ || event.Payload.RefType != reftyp {
			continue
		}

		eventWorkerCount += 1

		// pass github events to the workers
		workerRepoTasks <- WorkerRepoTask{
			auth:         auth,
			filters:      params,
			event:        event,
			repositories: repositories,
		}
	}

	return eventWorkerCount, nil
}

func fetchRepositories(ctx context.Context, params url.Values) ([]Repo, error) {
	repositories := make(chan WorkerRepository)
	defer close(repositories)

	auth, _ := ctx.Value(Authorization{}).(Authorization)

	eventWorkerCount, err := dispatchWorkerRepoTasks(ctx, auth, params, repositories)
	if err != nil {
		return nil, fmt.Errorf("dispatchWorkerRepoTasks failed: %w", err)
	}

	results := make([]Repo, 0, eventWorkerCount)

	// wait for all events to be processed by the workers so we don't close the channel too soon
	for eventWorkerCount > 0 {
		select {
		case repository := <-repositories:
			eventWorkerCount -= 1

			// aggregates the results of the workers
			if repository.Err == nil {
				results = append(results, Repo{
					Url:          repository.Repository.Url,
					Name:         repository.Repository.Name,
					Description:  repository.Repository.Description,
					Owner:        repository.Repository.Owner.Login,
					LanguagesUrl: repository.Repository.LanguagesUrl,
					Language:     repository.Repository.Language,
				})
			}
		}
	}

	return results, err
}

type Authorization struct {
	Token string
}

func reposHandlerGet(w http.ResponseWriter, r *http.Request, _ map[string]string) error {
	log := logger.Get(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ctx = context.WithValue(ctx, Authorization{}, Authorization{Token: r.Header.Get("Authorization")})

	repos, err := fetchRepositories(ctx, r.URL.Query())
	if err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		err = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		if err != nil {
			log.WithError(err).Error("Fail to encode JSON")
		}
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(repos)
	if err != nil {
		log.WithError(err).Error("Fail to encode JSON")
	}

	return nil
}

type Stats struct {
	Repo
	StarCount int               `json:"stars_count"`
	Languages map[string]string `json:"languages"`
}

func fetchStats(ctx context.Context, params url.Values) ([]Stats, error) {
	repositories := make(chan WorkerRepository)
	defer close(repositories)

	auth, _ := ctx.Value(Authorization{}).(Authorization)

	eventWorkerRepoCount, err := dispatchWorkerRepoTasks(ctx, auth, params, repositories)
	if err != nil {
		return nil, fmt.Errorf("dispatchWorkerRepoTasks failed: %w", err)
	}

	stats := make(chan WorkerStats)
	defer close(stats)

	results := make([]Stats, 0, eventWorkerRepoCount)

	for eventWorkerRepoCount > 0 {
		select {
		case repository := <-repositories:
			if repository.Err != nil {
				eventWorkerRepoCount -= 1
				break
			}

			workerStatsTasks <- WorkerStatsTask{
				auth:       auth,
				repository: repository.Repository,
				stats:      stats,
			}
		case stat := <-stats:
			eventWorkerRepoCount -= 1

			if stat.Err != nil {
				break
			}

			results = append(results, stat.Stats)
		}
	}

	return results, nil
}

func statsHandlerGet(w http.ResponseWriter, r *http.Request, _ map[string]string) error {
	log := logger.Get(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ctx = context.WithValue(ctx, Authorization{}, Authorization{Token: r.Header.Get("Authorization")})

	stats, err := fetchStats(ctx, r.URL.Query())
	if err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		err = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		if err != nil {
			log.WithError(err).Error("Fail to encode JSON")
		}
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(stats)
	if err != nil {
		log.WithError(err).Error("Fail to encode JSON")
	}

	return nil
}
