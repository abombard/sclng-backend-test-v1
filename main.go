package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/Scalingo/go-handlers"
	"github.com/Scalingo/go-utils/logger"
)

type Authorization struct {
	Token string
}

func main() {
	log := logger.Default()
	log.Info("Initializing app")
	cfg, err := newConfig()
	if err != nil {
		log.WithError(err).Error("Fail to initialize configuration")
		os.Exit(1)
	}

	// start workers
	ctx := context.Background()

	// TODO handle SIGINT so we finish the requests being processed
	initStatsWorkers(ctx, 16)

	log.Info("Initializing routes")
	router := handlers.NewRouter(log)
	router.HandleFunc("/ping", pongHandler)
	// Initialize web server and configure the following routes:
	// GET /repos
	// GET /stats

	router.HandleFunc("/repos", reposHandlerGet).Methods(http.MethodGet)
	router.HandleFunc("/stats", statsHandlerGet).Methods(http.MethodGet)

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

func reposHandlerGet(w http.ResponseWriter, r *http.Request, _ map[string]string) error {
	log := logger.Get(r.Context())

	ctx := r.Context()
	ctx = context.WithValue(ctx, Authorization{}, Authorization{Token: r.Header.Get("Authorization")})

	repos, err := fetchRepositories(ctx, r.URL.Query())
	if err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		err = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		if err != nil {
			log.WithError(err).Error("Fail to encode JSON")
		}

		return nil
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(repos)
	if err != nil {
		log.WithError(err).Error("Fail to encode JSON")
	}

	return nil
}

func statsHandlerGet(w http.ResponseWriter, r *http.Request, _ map[string]string) error {
	log := logger.Get(r.Context())

	ctx := r.Context()
	ctx = context.WithValue(ctx, Authorization{}, Authorization{Token: r.Header.Get("Authorization")})

	stats, err := fetchStats(ctx, r.URL.Query())
	if err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		err = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		if err != nil {
			log.WithError(err).Error("Fail to encode JSON")
		}

		return nil
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(stats)
	if err != nil {
		log.WithError(err).Error("Fail to encode JSON")
	}

	return nil
}
