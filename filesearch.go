package main

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	log "github.com/sirupsen/logrus"
)

var (
	fileWG            sync.WaitGroup
	fileSearchResults []fileSearchRes
	userDirButtons    map[string]*gtk.Box
)

type fileSearchRes struct {
	path string
	dir  string
}

func searchFs(ctx context.Context) {
	fileSearchResultFlowBoxLock.Lock()
	defer fileSearchResultFlowBoxLock.Unlock()

	// check if a new search was made
	select {
	case <-ctx.Done():
		return
	default:

		fileWG.Add(1)
		glib.IdleAdd(func() {
			defer fileWG.Done()

			if fileSearchResultFlowBox != nil {
				fileSearchResultFlowBox.Destroy()
				fileSearchResultFlowBox = nil
			}

			fileSearchResultFlowBox = setUpFileSearchResultContainer()
		})

		fileWG.Wait()

		fileSearchResults = nil
		userDirButtons = make(map[string]*gtk.Box)

		for key := range userDirsMap {
			if key != "home" {
				ignore = userDirsMap[key]
				filepath.WalkDir(userDirsMap[key], getWalkFun(key))
			}
		}

		if len(fileSearchResults) == 0 {
			return
		}

		for _, searchRes := range fileSearchResults {
			log.Debugf("Path: %s", searchRes)

			fileWG.Add(1)
		}

		glib.IdleAdd(createFileSearchResultButtonFunc(ctx, 0))

		fileWG.Wait()

		fileWG.Add(1)
		glib.IdleAdd(func() {
			defer fileWG.Done()

			select {
			case <-ctx.Done():
				return
			default:
				if len(fileSearchResultFlowBox.Children()) == 0 {
					fileSearchResultWrapper.Hide()
					statusLabel.SetText("0 results")
					return
				}

				fileSearchResultFlowBox.Hide()

				statusLabel.SetText(fmt.Sprintf("%v results | LMB: xdg-open | RMB: file manager",
					len(fileSearchResultFlowBox.Children())))
				num := uint(len(fileSearchResultFlowBox.Children())) / *fsColumns
				fileSearchResultFlowBox.SetMinChildrenPerLine(num + 1)
				fileSearchResultFlowBox.SetMaxChildrenPerLine(num + 1)

				fileSearchResultFlowBox.ShowAll()
			}
		})

		// wait for all operations to complete before unlocking the mutex for next search
		fileWG.Wait()
	}
}

func createFileSearchResultButtonFunc(ctx context.Context, index int) func() {
	return func() {
		defer fileWG.Done()

		select {
		case <-ctx.Done():
			// this safe cause there is at most one scheduled operation
			for range len(fileSearchResults) - index - 1 {
				fileWG.Done()
			}
			return
		default:
			if index+1 < len(fileSearchResults) {
				// schedule createButton Operations one by one to not Block the Event Loop
				defer glib.IdleAdd(createFileSearchResultButtonFunc(ctx, index+1))
			}

			searchRes := fileSearchResults[index]

			_, ok := userDirButtons[searchRes.dir]
			if !ok {
				btn := setUpUserDirButton(fmt.Sprintf("folder-%s", searchRes.dir), "", searchRes.dir, userDirsMap)
				fileSearchResultFlowBox.Add(btn)
				btn.Parent().(*gtk.FlowBoxChild).SetCanFocus(false)
				userDirButtons[searchRes.dir] = btn
			}

			partOfPathToShow := strings.Split(searchRes.path, userDirsMap[searchRes.dir])[1]
			if partOfPathToShow != "" {
				if !(strings.HasPrefix(searchRes.path, "#is_dir#") && isExcluded(searchRes.path)) {
					button := setUpUserFileSearchResultButton(partOfPathToShow, searchRes.path)
					if button != nil {
						fileSearchResultFlowBox.Add(button)
					}
					button.Parent().(*gtk.FlowBoxChild).SetCanFocus(false)
				}
			}
		}
	}
}

func getWalkFun(key string) func(path string, d fs.DirEntry, e error) error {
	return func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}

		splitPath := strings.Split(path, ignore)

		if len(splitPath) < 2 {
			fmt.Println(splitPath)
			return nil
		}
		// don't search leading part of the path, as e.g. '/home/user/Pictures'
		toSearch := splitPath[1]

		// Remaining part of the path (w/o file name) must be checked against being present in excluded dirs
		doSearch := true
		parts := strings.Split(toSearch, "/")
		remainingPart := ""
		if len(parts) > 1 {
			remainingPart = strings.Join(parts[:len(parts)-1], "/")
		}
		if remainingPart != "" && isExcluded(remainingPart) {
			doSearch = false
		}

		if doSearch && strings.Contains(strings.ToLower(toSearch), strings.ToLower(phrase)) {
			// mark directories
			if d.IsDir() {
				fileSearchResults = append(fileSearchResults, fileSearchRes{
					path: fmt.Sprintf("#is_dir#%s", path),
					dir:  key,
				})
			} else {
				fileSearchResults = append(fileSearchResults, fileSearchRes{
					path: path,
					dir:  key,
				})
			}
		}

		return nil
	}
}
