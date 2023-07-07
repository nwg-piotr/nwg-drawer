package main

import (
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/fsnotify/fsnotify"
)

// Thanks to Steve Domino https://medium.com/@skdomino/watch-this-file-watching-in-go-5b5a247cf71f
var watcher *fsnotify.Watcher

func watchFiles() {

	// creates a new file watcher
	watcher, _ = fsnotify.NewWatcher()
	defer watcher.Close()

	if err := watcher.Add(pinnedFile); err != nil {
		log.Errorf("ERROR: %s", err)
	}

	for _, fp := range appDirs {
		if err := filepath.Walk(fp, watchDir); err != nil {
			log.Errorf("ERROR: %s", err)
		}
	}

	done := make(chan bool)

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if strings.HasSuffix(event.Name, ".desktop") &&
					(event.Op.String() == "CREATE" ||
						event.Op.String() == "REMOVE" ||
						event.Op.String() == "RENAME") {
					desktopTrigger = true
				} else if event.Name == pinnedFile {
					// TODO: This can be used to propagate information about the changed file to the
					//       GUI to avoid recreating everything
					pinnedItemsChanged <- struct{}{}
				}

			case err := <-watcher.Errors:
				log.Errorf("ERROR: %s", err)
			}
		}
	}()

	<-done
}

func watchDir(path string, fi os.FileInfo, err error) error {
	if fi.Mode().IsDir() {
		return watcher.Add(path)
	}

	return nil
}
