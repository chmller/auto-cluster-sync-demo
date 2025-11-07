package worker

import (
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/c.mueller/auto-cluster-sync-demo/internal/cluster"
	"github.com/c.mueller/auto-cluster-sync-demo/internal/database"
	"github.com/c.mueller/auto-cluster-sync-demo/internal/models"
)

// Worker manages background job processing
type Worker struct {
	db                *database.DB
	cluster           *cluster.Cluster
	nodeID            string
	shutdown          chan struct{}
	ticker            *time.Ticker
	staleTicker       *time.Ticker
	processing        atomic.Bool
	stopped           bool
	staleJobTimeout   time.Duration
	heartbeatInterval time.Duration
}

// New creates a new worker instance
func New(db *database.DB, cluster *cluster.Cluster, nodeID string) *Worker {
	return &Worker{
		db:                db,
		cluster:           cluster,
		nodeID:            nodeID,
		shutdown:          make(chan struct{}),
		stopped:           false,
		staleJobTimeout:   30 * time.Second,
		heartbeatInterval: 5 * time.Second,
	}
}

// Start begins the worker loop
func (w *Worker) Start() {
	log.Printf("[INFO] Worker starting...")

	w.ticker = time.NewTicker(1 * time.Second)
	w.staleTicker = time.NewTicker(30 * time.Second)

	go w.workerLoop()
	log.Printf("[INFO] Worker started successfully")
}

// Stop gracefully shuts down the worker
func (w *Worker) Stop() {
	// Check if already stopped (idempotent)
	if w.stopped {
		return
	}
	w.stopped = true

	log.Printf("[INFO] Worker stopping...")
	close(w.shutdown)

	if w.ticker != nil {
		w.ticker.Stop()
	}
	if w.staleTicker != nil {
		w.staleTicker.Stop()
	}

	log.Printf("[INFO] Worker stopped")
}

// workerLoop is the main worker loop
func (w *Worker) workerLoop() {
	for {
		select {
		case <-w.ticker.C:
			// Try to claim and process a job if not currently processing
			if !w.processing.Load() {
				w.tryClaimAndProcess()
			}

		case <-w.staleTicker.C:
			// Check for stale jobs
			w.checkStaleJobs()

		case <-w.shutdown:
			log.Printf("[INFO] Worker loop exiting")
			return
		}
	}
}

// tryClaimAndProcess attempts to claim and process the next pending job
func (w *Worker) tryClaimAndProcess() {
	// 1. Claim next job
	todo, err := w.db.ClaimNextPendingTodo(w.nodeID)
	if err != nil {
		log.Printf("[ERROR] Failed to claim job: %v", err)
		return
	}

	if todo == nil {
		// No jobs available
		return
	}

	w.processing.Store(true)
	defer w.processing.Store(false)

	log.Printf("[INFO] Claimed job: %s", todo.ExternID)

	// 2. Broadcast claim
	if err := w.cluster.BroadcastJobClaimed(todo); err != nil {
		log.Printf("[WARN] Failed to broadcast job claimed: %v", err)
	}

	// 3. Mark as processing and broadcast
	if err := w.db.MarkJobProcessing(todo.ExternID); err != nil {
		log.Printf("[ERROR] Failed to mark job as processing: %v", err)
		w.releaseJob(todo.ExternID)
		return
	}

	if err := w.cluster.BroadcastJobStarted(todo); err != nil {
		log.Printf("[WARN] Failed to broadcast job started: %v", err)
	}

	// 4. Start heartbeat goroutine
	heartbeatDone := make(chan struct{})
	go w.heartbeatLoop(todo.ExternID, heartbeatDone)

	// 5. Process job
	log.Printf("[INFO] Processing job: %s - %s", todo.ExternID, todo.Todo)
	err = w.processJob(todo)
	close(heartbeatDone) // Stop heartbeat

	// 6. Mark completed or failed
	if err != nil {
		log.Printf("[ERROR] Job failed: %s - %v", todo.ExternID, err)
		if err := w.db.UpdateJobStatus(todo.ExternID, models.StatusFailed); err != nil {
			log.Printf("[ERROR] Failed to update job status: %v", err)
		}
		if err := w.cluster.BroadcastJobFailed(todo); err != nil {
			log.Printf("[WARN] Failed to broadcast job failed: %v", err)
		}
	} else {
		log.Printf("[INFO] Job completed: %s", todo.ExternID)

		// Mark todo as completed in database
		completed := true
		_, err := w.db.UpdateTodo(todo.ID, nil, &completed)
		if err != nil {
			log.Printf("[ERROR] Failed to mark todo as completed: %v", err)
		}

		if err := w.db.UpdateJobStatus(todo.ExternID, models.StatusCompleted); err != nil {
			log.Printf("[ERROR] Failed to update job status: %v", err)
		}
		if err := w.cluster.BroadcastJobCompleted(todo); err != nil {
			log.Printf("[WARN] Failed to broadcast job completed: %v", err)
		}
	}
}

// processJob performs the actual work (simulated)
func (w *Worker) processJob(todo *models.Todo) error {
	// Simulate work by sleeping for 5-10 seconds
	duration := time.Duration(5+rand.Intn(6)) * time.Second
	log.Printf("[INFO] Job %s will take %v", todo.ExternID, duration)

	// Sleep in small increments to allow for graceful shutdown
	sleepInterval := 500 * time.Millisecond
	elapsed := time.Duration(0)

	for elapsed < duration {
		select {
		case <-w.shutdown:
			log.Printf("[INFO] Job %s interrupted by shutdown", todo.ExternID)
			return nil
		case <-time.After(sleepInterval):
			elapsed += sleepInterval
		}
	}

	log.Printf("[INFO] Job %s finished processing", todo.ExternID)
	return nil
}

// heartbeatLoop sends periodic heartbeats while a job is being processed
func (w *Worker) heartbeatLoop(externID string, done chan struct{}) {
	ticker := time.NewTicker(w.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.db.SendHeartbeat(externID, w.nodeID); err != nil {
				log.Printf("[WARN] Failed to send heartbeat for job %s: %v", externID, err)
				return
			}

			// Broadcast heartbeat (optional, for monitoring)
			if err := w.cluster.BroadcastJobHeartbeat(externID); err != nil {
				log.Printf("[WARN] Failed to broadcast heartbeat: %v", err)
			}

		case <-done:
			return

		case <-w.shutdown:
			return
		}
	}
}

// checkStaleJobs checks for and reclaims stale jobs
func (w *Worker) checkStaleJobs() {
	staleJobs, err := w.db.GetStaleJobs(w.staleJobTimeout)
	if err != nil {
		log.Printf("[ERROR] Failed to get stale jobs: %v", err)
		return
	}

	if len(staleJobs) == 0 {
		return
	}

	log.Printf("[WARN] Found %d stale job(s)", len(staleJobs))

	for _, job := range staleJobs {
		log.Printf("[INFO] Reclaiming stale job: %s (was on node %s)", job.ExternID, *job.ClaimedBy)

		if err := w.db.ReleaseJob(job.ExternID); err != nil {
			log.Printf("[ERROR] Failed to release stale job %s: %v", job.ExternID, err)
			continue
		}

		log.Printf("[INFO] Released stale job %s back to pending", job.ExternID)

		if err := w.cluster.BroadcastJobReleased(&job); err != nil {
			log.Printf("[WARN] Failed to broadcast job released: %v", err)
		}
	}
}

// releaseJob releases a job back to pending status
func (w *Worker) releaseJob(externID string) {
	if err := w.db.ReleaseJob(externID); err != nil {
		log.Printf("[ERROR] Failed to release job %s: %v", externID, err)
	}
}
