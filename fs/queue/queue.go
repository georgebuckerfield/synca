package queue

import (
	"fmt"

	"github.com/georgebuckerfield/synca/clouds"
	"github.com/georgebuckerfield/synca/fs"
)

// Queue manages the filesystem events
type Queue struct {
	InQueue   chan fs.FsEvent
	OutQueues map[string]chan fs.FsEvent
}

// FsQueueCapacity sets the capacity of the inbound queue
const Capacity = 3

// Manager looks after the filesystem events queue and is responsible for
// implementing fan-out of events to cloud consumers
func (queue Queue) Manager() {
	// Setup a map of outbound queues for the cloud consumers to receive from
	queue.OutQueues = make(map[string]chan fs.FsEvent)
	// For each registered cloud provider, create an outbound queue and a
	// goroutine to receive from the channel
	for _, consumer := range clouds.Registered {
		q := make(chan fs.FsEvent, Capacity)
		queue.OutQueues[consumer.Name()] = q
		fmt.Printf("Starting %s consumer queues\n", consumer.Name())
		go consumer.QueueReceive(queue.OutQueues[consumer.Name()])
	}
	// Receive on the main inbound queue and duplicate each event across all
	// available consumer channels
	for {
		evt := <-queue.InQueue
		for _, out := range queue.OutQueues {
			out <- evt
		}
	}
}
