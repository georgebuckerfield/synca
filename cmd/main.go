package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/georgebuckerfield/synca"
	"github.com/georgebuckerfield/synca/clouds"
	"github.com/georgebuckerfield/synca/fs"
	"github.com/georgebuckerfield/synca/fs/queue"
	// Cloud provider imports:
	_ "github.com/georgebuckerfield/synca/clouds/aws"
)

type workers struct {
	Wg sync.WaitGroup
}

func main() {

	path := os.Args[1]
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatalf("path %s does not exist\n", path)
	}

	bucket := os.Args[2]
	synca.Config.BucketName = bucket

	region := os.Args[3]
	synca.Config.BucketRegion = region

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Printf("received %v, shutting down processes...\n", c)
		// For each cloud
		for _, c := range clouds.Registered {
			// Setup a channel for the cloud to tell us when they're done
			done := make(chan struct{})
			// Retrieve the cancellation channel from the cloud and give them our
			// done channel
			cancel := c.Cancel(done)
			// Send the signal
			cancel <- struct{}{}
			// Wait for confirmation they're done
			<-done
		}
		os.Exit(1)
	}()

	for _, c := range clouds.Registered {
		go c.StartWorkers()
	}

	q := queue.Queue{}
	q.InQueue = make(chan fs.FsEvent, queue.Capacity)

	err := fs.WatchFsEvents(path, q.InQueue)
	if err != nil {
		panic(err)
	}
	q.Manager()
}
