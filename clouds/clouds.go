package clouds

import (
	"github.com/georgebuckerfield/synca/fs"
)

// Cloud is a cloud interface capable of processing sync requests
type Cloud interface {
	Name() string
	QueueReceive(evt <-chan fs.FsEvent)
	QueueWrite()
	StartWorkers()
	Cancel(chan struct{}) chan struct{}
}

// Registered is an array of registered clouds
var Registered map[string]Cloud

func init() {
	Registered = make(map[string]Cloud)
}
