package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arruko/torvyn-sentientd/internal/argo"
	"github.com/arruko/torvyn-sentientd/internal/config"
	"github.com/arruko/torvyn-sentientd/internal/correlation"
	"github.com/arruko/torvyn-sentientd/internal/database"
	"github.com/arruko/torvyn-sentientd/internal/domain"
	"github.com/arruko/torvyn-sentientd/internal/httpserver"
	"github.com/arruko/torvyn-sentientd/internal/ingest"
	"github.com/arruko/torvyn-sentientd/internal/investigation"
	"github.com/arruko/torvyn-sentientd/internal/kagent"
	"github.com/arruko/torvyn-sentientd/internal/logging"
	"github.com/arruko/torvyn-sentientd/internal/policy"
	"github.com/arruko/torvyn-sentientd/internal/queue"
	"github.com/arruko/torvyn-sentientd/internal/slack"
	"github.com/arruko/torvyn-sentientd/internal/store"
	"github.com/arruko/torvyn-sentientd/internal/topology"
)

type simpleIDSource struct{}

func (s simpleIDSource) NewIncidentID() string {
	return time.Now().Format("20060102-150405.000000")
}

type emptyGraph struct{}

func (emptyGraph) SubgraphForService(service string) (*domain.TopologySlice, error) {
	return nil, nil
}

func main() {
	cfg := config.FromEnv()
	ctx := context.Background()

	// Initialize incident store (Postgres or In-Memory based on config)
	var incidentStore store.IncidentStore
	if cfg.UseInMemoryStore || cfg.DatabaseURL == "" {
		logging.Info.Println("Using in-memory incident store")
		incidentStore = store.NewInMemoryIncidentStore()
	} else {
		logging.Info.Println("Connecting to Postgres...")
		dbConfig := database.DefaultConfig(cfg.DatabaseURL)
		pool, err := database.Connect(ctx, dbConfig)
		if err != nil {
			logging.Error.Printf("Failed to connect to database: %v", err)
			fmt.Fprintf(os.Stderr, "FATAL: database connection failed: %v\n", err)
			os.Exit(1)
		}
		defer database.Close(pool)

		incidentStore = store.NewPostgresIncidentStore(pool)
		logging.Info.Println("Using Postgres incident store")
	}

	// Initialize ServiceGraph topology client
	topologyClient := topology.NewClient(cfg.ServiceName) // Use same namespace as sentientd
	if err := topologyClient.Initialize(); err != nil {
		logging.Info.Printf("ServiceGraph client not initialized (running in local mode): %v", err)
		// Fall back to empty graph for local dev
		topologyClient = nil
	}

	// Use topology client if available, otherwise use empty graph
	var graph topology.Graph
	if topologyClient != nil {
		graph = topologyClient
	} else {
		graph = emptyGraph{}
	}

	idSource := simpleIDSource{}

	corr := correlation.New(correlation.Config{
		InitialWindow:   cfg.InitialWindow,
		ExpansionWindow: cfg.ExpansionWindow,
	}, incidentStore, graph, idSource)

	kagentClient := kagent.New(cfg.KagentURL)
	argoClient := argo.New(cfg.ArgoNamespace)
	slackClient := slack.New(cfg.SlackWebhookURL)

	// Initialize Argo client (in-cluster mode)
	// This will fail gracefully in local dev without kubeconfig
	if err := argoClient.Initialize(); err != nil {
		logging.Info.Printf("Argo client not initialized (running in local mode): %v", err)
	}

	// Initialize policy engine
	var policyEngine *policy.Engine
	if cfg.PolicyDir != "" {
		pe, err := policy.NewEngine(cfg.PolicyDir)
		if err != nil {
			logging.Error.Printf("Failed to initialize policy engine: %v", err)
			logging.Info.Printf("Continuing without policy validation")
		} else {
			policyEngine = pe
			logging.Info.Printf("Policy engine initialized with policies from %s", cfg.PolicyDir)
		}
	} else {
		logging.Info.Printf("Policy directory not configured, skipping policy validation")
	}

	invEngine := investigation.NewEngine(incidentStore, kagentClient, argoClient, slackClient, policyEngine)

	// Initialize NATS queue if enabled
	var natsQueue queue.Queue
	var ingestWorker *ingest.Worker
	ingestCtx, cancelIngest := context.WithCancel(context.Background())

	if cfg.NATSEnabled && cfg.NATSURL != "" {
		logging.Info.Printf("NATS enabled, connecting to %s", cfg.NATSURL)
		nq, err := queue.NewNATSQueue(cfg.NATSURL)
		if err != nil {
			logging.Error.Printf("Failed to connect to NATS: %v", err)
			logging.Info.Printf("Continuing without NATS")
		} else {
			natsQueue = nq
			logging.Info.Printf("NATS queue initialized")

			// Start ingest worker
			ingestWorker = ingest.NewWorker(natsQueue)
			go func() {
				if err := ingestWorker.Start(ingestCtx); err != nil {
					logging.Error.Printf("Ingest worker error: %v", err)
				}
			}()
		}
	} else {
		logging.Info.Printf("NATS disabled (NATS_ENABLED=%v, NATS_URL=%s)", cfg.NATSEnabled, cfg.NATSURL)
	}

	// Setup graceful shutdown
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	// For v1: simple background loop that periodically scans for open incidents
	// and triggers investigation once. Later: event-driven.
	investigationCtx, cancelInvestigation := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-investigationCtx.Done():
				logging.Info.Println("Investigation loop shutting down...")
				return
			case <-time.After(30 * time.Second):
				incs, err := incidentStore.List(investigationCtx)
				if err != nil {
					logging.Error.Printf("list incidents failed: %v", err)
					continue
				}
				for _, inc := range incs {
					// naive: investigate all open incidents
					if inc.State == domain.IncidentStateOpen {
						if err := invEngine.InvestigateIncident(investigationCtx, inc.ID); err != nil {
							logging.Error.Printf("investigate %s failed: %v", inc.ID, err)
						}
					}
				}
			}
		}
	}()

	// Start HTTP server in a goroutine
	srv := httpserver.New(cfg, corr)
	if natsQueue != nil {
		srv.SetQueue(natsQueue)
	}
	serverErrors := make(chan error, 1)
	go func() {
		logging.Info.Printf("Starting HTTP server on %s", cfg.ListenAddr)
		serverErrors <- srv.ListenAndServe()
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErrors:
		logging.Error.Printf("HTTP server error: %v", err)
	case sig := <-shutdownChan:
		logging.Info.Printf("Received signal %v, initiating graceful shutdown...", sig)

		// Cancel investigation loop
		cancelInvestigation()

		// Cancel ingest worker
		cancelIngest()

		// Close NATS connection
		if natsQueue != nil {
			if err := natsQueue.Close(); err != nil {
				logging.Error.Printf("NATS queue close error: %v", err)
			}
		}

		// Shutdown HTTP server with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logging.Error.Printf("HTTP server shutdown error: %v", err)
		}

		logging.Info.Println("Shutdown complete")
	}
}
