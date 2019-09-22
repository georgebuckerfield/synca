package fs

import (
	"fmt"
	"golang.org/x/sys/unix"
	"strings"
	"unsafe"
)

// FsEvent defines a fs event
type FsEvent struct {
	Filename  string
	Operation string
}

// WatchFsEvents creates the watcher and reads fileystem events from it
func WatchFsEvents(path string, queue chan FsEvent) error {
	fd, err := createWatcher(path)
	if err != nil {
		return err
	}
	go readFsEvents(path, queue, fd)
	return nil
}

func createWatcher(path string) (int, error) {

	// Create an inotify file descriptor
	fd, errno := unix.InotifyInit1(unix.IN_CLOEXEC)
	if fd == -1 {
		err := fmt.Errorf("error creating inotify file descriptor: %s", errno)
		return -1, err
	}

	// What flags are we looking for?
	flags := uint32(unix.IN_CREATE | unix.IN_CLOSE_WRITE | unix.IN_DELETE)

	// Create a watcher
	wd, errno := unix.InotifyAddWatch(fd, path, flags)
	if wd == -1 {
		err := fmt.Errorf("error creating watcher: %s", errno)
		return -1, err
	}

	// Return the file descriptor for the inotify instance
	return fd, nil
}

func readFsEvents(path string, queue chan FsEvent, fd int) {

	// Create a buffer
	var buf [unix.SizeofInotifyEvent * 4096]byte

	for {
		n, _ := unix.Read(fd, buf[:])
		var offset uint32

		for offset <= uint32(n-unix.SizeofInotifyEvent) {
			event := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			mask := uint32(event.Mask)
			nameLen := uint32(event.Len)
			bytes := (*[unix.PathMax]byte)(unsafe.Pointer(&buf[offset+unix.SizeofInotifyEvent]))
			name := path + "/" + strings.TrimRight(string(bytes[0:nameLen]), "\000")

			var op string
			if mask&unix.IN_DELETE == unix.IN_DELETE {
				op = "delete"
			}
			if mask&unix.IN_CLOSE_WRITE == unix.IN_CLOSE_WRITE {
				op = "close_write"
			}

			evt := FsEvent{Filename: name, Operation: op}
			queue <- evt

			offset += (unix.SizeofInotifyEvent + nameLen)
		}
	}
}
