package gcp

import (
	"fmt"

	"github.com/georgebuckerfield/s3sync/filesystem"
)

// GCPCloud is an instance of GCP
type GCPCloud struct {
	InboundQueue chan filesystem.FsEvent
}

// Init gets the AWS environment ready
func (gcp *GCPCloud) Init() error {
	fmt.Println("Setting up GCP cloud")
	return nil
}

// Name returns the name of the cloud provider
func (gcp *GCPCloud) Name() string {
	return "GCPCloud"
}

// QueueReceive reads from the event queue
func (gcp *GCPCloud) QueueReceive(queue <-chan filesystem.FsEvent) {
	for evt := range queue {
		fmt.Printf("gcp provider: %s operation recvd for %s\n", evt.Operation, evt.Filename)
	}
}

// QueueWrite writes to the event queue
func (gcp *GCPCloud) QueueWrite() {
	fmt.Println("Writing to queue")
}
