package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Thanks to Steve Domino https://medium.com/@skdomino/watch-this-file-watching-in-go-5b5a247cf71f
var watcher *fsnotify.Watcher

func watchFiles() {

	// creates a new file watcher
	watcher, _ = fsnotify.NewWatcher()
	defer watcher.Close()

	if err := watcher.Add(pinnedFile); err != nil {
		fmt.Println("ERROR", err)
	}

	for _, fp := range appDirs {
		if err := filepath.Walk(fp, watchDir); err != nil {
			fmt.Println("ERROR", err)
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
					pinnedTrigger = true
				}

			case err := <-watcher.Errors:
				fmt.Println("ERROR", err)
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
