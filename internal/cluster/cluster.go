package cluster

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/c.mueller/auto-cluster-sync-demo/internal/database"
	"github.com/c.mueller/auto-cluster-sync-demo/internal/models"
	"github.com/hashicorp/serf/serf"
)

// Cluster manages the Serf cluster and synchronization
type Cluster struct {
	serf     *serf.Serf
	db       *database.DB
	nodeID   string
	eventCh  chan serf.Event
	shutdown chan struct{}
	ready    bool
	readyCh  chan struct{}
	stopped  bool
}

// New creates a new Cluster instance
func New(nodeID string, bindAddr string, db *database.DB) (*Cluster, error) {
	// Parse bind address (format: "IP:Port")
	host, portStr, err := net.SplitHostPort(bindAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid bind address %q: %w", bindAddr, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port in bind address %q: %w", bindAddr, err)
	}

	// Create Serf configuration
	config := serf.DefaultConfig()
	config.NodeName = nodeID
	config.MemberlistConfig.BindAddr = host
	config.MemberlistConfig.BindPort = port

	// Create event channel
	eventCh := make(chan serf.Event, 256)
	config.EventCh = eventCh

	// Create cluster instance
	cluster := &Cluster{
		db:       db,
		nodeID:   nodeID,
		eventCh:  eventCh,
		shutdown: make(chan struct{}),
		ready:    false,
		readyCh:  make(chan struct{}),
		stopped:  false,
	}

	// Create Serf instance
	serfInstance, err := serf.Create(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create serf: %w", err)
	}

	cluster.serf = serfInstance

	return cluster, nil
}

// Start starts the cluster and joins the seed nodes
func (c *Cluster) Start(seeds []string, joinTimeout time.Duration) error {
	// Start event handler
	go c.handleEvents()

	// Join cluster via seeds
	if len(seeds) > 0 {
		log.Printf("🔍 Attempting to join cluster via seeds: %v", seeds)

		// Retry logic
		maxRetries := 3
		var lastErr error
		joined := false

		for i := 0; i < maxRetries; i++ {
			if i > 0 {
				backoff := time.Duration(i) * 2 * time.Second
				log.Printf("⏳ Retry %d/%d in %v...", i+1, maxRetries, backoff)
				time.Sleep(backoff)
			}

			numJoined, err := c.serf.Join(seeds, true)
			if err != nil {
				lastErr = err
				log.Printf("⚠️  Join attempt %d failed: %v", i+1, err)
				continue
			}

			if numJoined > 0 {
				log.Printf("✅ Successfully joined %d nodes", numJoined)
				joined = true
				break
			}
		}

		// If we couldn't join but didn't error, we might be the first node
		if !joined && lastErr == nil {
			log.Println("ℹ️  No seeds responded, starting as first node")
			c.markReady()
			return nil
		}

		if !joined {
			log.Printf("⚠️  Failed to join after %d attempts: %v", maxRetries, lastErr)
			log.Println("ℹ️  Continuing as standalone node")
			c.markReady()
			return nil
		}

		// Wait for full sync to complete (with timeout)
		log.Println("⏳ Waiting for full sync to complete...")
		syncTimeout := 30 * time.Second
		select {
		case <-c.readyCh:
			log.Println("✅ Node is ready")
			return nil
		case <-time.After(syncTimeout):
			log.Printf("⚠️  Full sync timeout after %v, continuing anyway", syncTimeout)
			c.markReady()
			return nil
		}
	} else {
		log.Println("ℹ️  No seeds configured, starting as first node")
		c.markReady()
	}

	return nil
}

// Stop gracefully shuts down the cluster
func (c *Cluster) Stop() error {
	// Check if already stopped (idempotent)
	if c.stopped {
		return nil
	}
	c.stopped = true

	log.Println("🛑 Shutting down cluster...")

	// Signal shutdown to event handler
	close(c.shutdown)

	// Leave the cluster gracefully
	if err := c.serf.Leave(); err != nil {
		log.Printf("⚠️  Error leaving cluster: %v", err)
	}

	// Shutdown Serf
	if err := c.serf.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown serf: %w", err)
	}

	log.Println("✅ Cluster shutdown complete")
	return nil
}

// Members returns the current cluster members
func (c *Cluster) Members() []serf.Member {
	return c.serf.Members()
}

// LocalNode returns the local node name
func (c *Cluster) LocalNode() string {
	return c.nodeID
}

// markReady marks the cluster as ready and signals waiting goroutines
func (c *Cluster) markReady() {
	if !c.ready {
		c.ready = true
		close(c.readyCh)
	}
}

// IsReady returns true if the cluster is ready to serve requests
func (c *Cluster) IsReady() bool {
	return c.ready
}

// GetMemberInfo returns information about all cluster members
func (c *Cluster) GetMemberInfo() []models.ClusterMemberInfo {
	members := c.serf.Members()
	info := make([]models.ClusterMemberInfo, len(members))

	for i, member := range members {
		info[i] = models.ClusterMemberInfo{
			Name:   member.Name,
			Addr:   member.Addr.String(),
			Status: member.Status.String(),
		}
	}

	return info
}

// MemberCount returns the number of cluster members
func (c *Cluster) MemberCount() int {
	return len(c.serf.Members())
}
