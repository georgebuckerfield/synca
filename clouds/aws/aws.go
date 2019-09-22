package aws

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/georgebuckerfield/synca"
	"github.com/georgebuckerfield/synca/clouds"
	"github.com/georgebuckerfield/synca/fs"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
)

const (
	// DeleteMaxBatchSize controls the batch size for deletions
	DeleteMaxBatchSize = 10
	// DeleteWorkers controls the number of workers processing deletes
	DeleteWorkers = 5
)

// AWSCloud is an instance of AWS for us to use
type cloud struct {
	bucket      string
	CancelChan  chan struct{}
	Done        chan struct{}
	DeleteQueue chan string
	UploadQueue chan string
	s3Svc       s3iface.S3API
	s3Deleter   s3manageriface.BatchDelete
	s3Uploader  s3manageriface.UploaderAPI
	workers     workerGroup
}

type workerGroup struct {
	wg sync.WaitGroup
}

type deleteBatch struct {
	files []string
	mux   sync.Mutex
}

// Init gets the AWS environment ready
func init() {
	fmt.Println("Setting up AWS cloud...")

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(synca.Config.BucketRegion)},
	)
	if err != nil {
		fmt.Printf("Error creating AWS cloud: %v", err)
		panic(err)
	}

	// Configure S3 objects and object queues
	var AWS cloud
	AWS.bucket = synca.Config.BucketName
	AWS.s3Svc = s3.New(sess)
	AWS.s3Uploader = s3manager.NewUploaderWithClient(AWS.s3Svc)
	AWS.DeleteQueue = make(chan string, 100)
	AWS.UploadQueue = make(chan string, 100)

	cancel := make(chan struct{})
	AWS.CancelChan = cancel

	// Register with the list of clouds
	clouds.Registered["aws"] = &AWS
}

// Name returns the name of the cloud provider
func (a *cloud) Name() string {
	return "AWSCloud"
}

func (a *cloud) Cancel(done chan struct{}) chan struct{} {
	a.Done = done
	return a.CancelChan
}

// StartWorkers starts the goroutines that will do the work
func (a *cloud) StartWorkers() {
	defer func() {
		fmt.Printf("send signal that we're done!\n")
		a.Done <- struct{}{}
	}()

	workerPool := workerGroup{
		wg: sync.WaitGroup{},
	}

	a.workers = workerPool
	// Create a channel so we can signal cancellation
	cancel := make(chan struct{})
	// Start the queue workers
	for i := 1; i < DeleteWorkers; i++ {
		fmt.Printf("starting delete queue worker %d\n", i)
		a.workers.wg.Add(1)
		go a.deleteQueueWorker(a.DeleteQueue, cancel)
	}
	a.workers.wg.Add(1)
	go a.uploadQueueWorker(a.UploadQueue, cancel)
	// Wait for cancellation signal
	for range a.CancelChan {
		fmt.Println("closing channel to signal cancellation to workers\n")
		close(cancel)
		break
	}
	fmt.Printf("sent cancellation to all workers\n")
	// Now wait for the workers to signal they're done
	a.workers.wg.Wait()
}

// QueueReceive reads from the event queue and directs them to the appropriate
// work queue to be processed
func (a *cloud) QueueReceive(queue <-chan fs.FsEvent) {
	for evt := range queue {
		fmt.Printf("aws provider: %s operation recvd for %s\n", evt.Operation, evt.Filename)
		switch evt.Operation {
		case "delete":
			a.DeleteQueue <- evt.Filename
		default:
			fmt.Printf("aws provider: pushing object to upload queue\n")
			a.UploadQueue <- evt.Filename
		}
	}
}

// QueueWrite writes to the event queue
func (a *cloud) QueueWrite() {
	fmt.Println("Writing to queue")
}

func (a *cloud) deleteQueueWorker(queue <-chan string, cancel <-chan struct{}) {
	toDelete := &deleteBatch{}
	for {
		select {
		case file := <-queue:
			toDelete.files = append(toDelete.files, file)
			// If there are still items to take off the queue, and we have still have
			// capacity in the batch, then loop again and add another item
			if len(queue) > 0 && len(toDelete.files) < DeleteMaxBatchSize {
				continue
			} else {
				// Otherwise, send the items to be deleted
				if err := a.deleteObjects(a.bucket, toDelete.files); err != nil {
					fmt.Printf("error deleting %s: %v\n", toDelete.files, err)
				}
				// Empty the batch ready for the next loop
				toDelete.files = []string{}
			}
		case <-cancel:
			fmt.Printf("delete worker recvd cancel, signal wg.Done()\n")
			a.workers.wg.Done()
			return
		}
	}

}

func (a *cloud) uploadQueueWorker(queue <-chan string, cancel <-chan struct{}) {
	for {
		select {
		case file := <-queue:
			fmt.Printf("uploadQueue received object\n")
			if err := a.uploadObject(a.bucket, file); err != nil {
				fmt.Printf("error uploading %s: %v\n", file, err)
			}
		case <-cancel:
			fmt.Printf("upload worker recvd cancel, signal wg.Done()\n")
			a.workers.wg.Done()
			return
		}
	}
}

func (a *cloud) deleteObjects(bucket string, filenames []string) error {
	// fmt.Printf("Starting delete for %s\n", filenames)
	batchSize := len(filenames)

	size := func(opts *s3manager.BatchDelete) {
		opts.BatchSize = batchSize
	}
	s3Deleter := s3manager.NewBatchDeleteWithClient(a.s3Svc, size)
	objects := []s3manager.BatchDeleteObject{}
	for _, f := range filenames {
		object := s3manager.BatchDeleteObject{
			Object: &s3.DeleteObjectInput{
				Key:    aws.String(strings.TrimLeft(f, "/")),
				Bucket: aws.String(bucket),
			},
		}
		objects = append(objects, object)
	}
	// fmt.Printf("running deleter for %v\n", objects)
	err := s3Deleter.Delete(aws.BackgroundContext(), &s3manager.DeleteObjectsIterator{
		Objects: objects,
	})
	if err != nil {
		fmt.Printf("error deleting %s: %v\n", filenames, err)
		return err
	}
	fmt.Printf("deleted %d objects\n", batchSize)
	return nil
}

func (a *cloud) uploadObject(bucket string, filename string) error {
	fmt.Printf("Starting upload for %s\n", filename)

	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Error opening file: %v", err)
	}

	defer f.Close()
	input := &s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
		Body:   f,
	}
	output, err := a.s3Uploader.Upload(input)
	if err != nil {
		return fmt.Errorf("Failed to upload file %s, %v", filename, err)
	}
	fmt.Printf("%v\n", output)
	return nil
}
