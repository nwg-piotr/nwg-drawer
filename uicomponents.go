package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

func setUpPinnedFlowBox() *gtk.FlowBox {
	if pinnedFlowBox != nil {
		pinnedFlowBox.Destroy()
	}
	flowBox, _ := gtk.FlowBoxNew()
	if uint(len(pinned)) >= *columnsNumber {
		flowBox.SetMaxChildrenPerLine(*columnsNumber)
	} else if len(pinned) > 0 {
		flowBox.SetMaxChildrenPerLine(uint(len(pinned)))
	}

	flowBox.SetColumnSpacing(*itemSpacing)
	flowBox.SetRowSpacing(*itemSpacing)
	flowBox.SetHomogeneous(true)
	flowBox.SetProperty("name", "pinned-box")
	flowBox.SetSelectionMode(gtk.SELECTION_NONE)

	if len(pinned) > 0 {
		for _, desktopID := range pinned {
			entry := id2entry[desktopID]

			btn, _ := gtk.ButtonNew()

			var img *gtk.Image
			if entry.Icon != "" {
				pixbuf, _ := createPixbuf(entry.Icon, *iconSize)
				img, _ = gtk.ImageNewFromPixbuf(pixbuf)
			} else {
				img, _ = gtk.ImageNewFromIconName("image-missing", gtk.ICON_SIZE_INVALID)
			}

			btn.SetImage(img)
			btn.SetAlwaysShowImage(true)
			btn.SetImagePosition(gtk.POS_TOP)

			name := ""
			if entry.NameLoc != "" {
				name = entry.NameLoc
			} else {
				name = entry.Name
			}
			if len(name) > 20 {
				r := []rune(name)
				name = string(r[:17])
				name = fmt.Sprintf("%s…", name)
			}
			btn.SetLabel(name)

			btn.Connect("button-release-event", func(row *gtk.Button, e *gdk.Event) bool {
				btnEvent := gdk.EventButtonNewFromEvent(e)
				if btnEvent.Button() == 1 {
					launch(entry.Exec, entry.Terminal)
					return true
				} else if btnEvent.Button() == 3 {
					unpinItem(entry.DesktopID)
					return true
				}
				return false
			})
			btn.Connect("activate", func() {
				launch(entry.Exec, entry.Terminal)
			})
			btn.Connect("enter-notify-event", func() {
				statusLabel.SetText(entry.CommentLoc)
			})
			flowBox.Add(btn)
		}
		pinnedFlowBoxWrapper.PackStart(flowBox, true, false, 0)

		//While moving focus with arrow keys we want buttons to get focus directly
		flowBox.GetChildren().Foreach(func(item interface{}) {
			item.(*gtk.Widget).SetCanFocus(false)
		})
	}
	flowBox.ShowAll()

	return flowBox
}

func setUpCategoriesButtonBox() *gtk.EventBox {
	lists := map[string][]string{
		"utility":              listUtility,
		"development":          listDevelopment,
		"game":                 listGame,
		"graphics":             listGraphics,
		"internet-and-network": listInternetAndNetwork,
		"office":               listOffice,
		"audio-video":          listAudioVideo,
		"system-tools":         listSystemTools,
		"other":                listOther,
	}

	eventBox, _ := gtk.EventBoxNew()

	hBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	eventBox.Add(hBox)
	button, _ := gtk.ButtonNewWithLabel("All")
	button.SetProperty("name", "category-button")
	button.Connect("clicked", func(item *gtk.Button) {
		searchEntry.SetText("")
		appFlowBox = setUpAppsFlowBox(nil, "")
		for _, btn := range catButtons {
			btn.SetImagePosition(gtk.POS_LEFT)
			btn.SetSizeRequest(0, 0)
		}
	})
	hBox.PackStart(button, false, false, 0)

	for _, cat := range categories {
		if isSupposedToShowUp(cat.Name) {
			button, _ = gtk.ButtonNewFromIconName(cat.Icon, gtk.ICON_SIZE_MENU)
			button.SetProperty("name", "category-button")
			catButtons = append(catButtons, button)
			button.SetLabel(cat.DisplayName)
			button.SetAlwaysShowImage(true)
			hBox.PackStart(button, false, false, 0)
			name := cat.Name
			b := *button
			button.Connect("clicked", func(item *gtk.Button) {
				searchEntry.SetText("")
				// !!! since gotk3 FlowBox type does not implement set_filter_func, we need to rebuild appFlowBox
				appFlowBox = setUpAppsFlowBox(lists[name], "")
				for _, btn := range catButtons {
					btn.SetImagePosition(gtk.POS_LEFT)
				}
				w := b.GetAllocatedWidth()
				b.SetImagePosition(gtk.POS_TOP)
				b.SetSizeRequest(w, 0)
				if fileSearchResultWrapper != nil {
					fileSearchResultWrapper.Hide()
				}
			})
		}
	}
	return eventBox
}

func isSupposedToShowUp(catName string) bool {
	result := catName == "utility" && notEmpty(listUtility) ||
		catName == "development" && notEmpty(listDevelopment) ||
		catName == "game" && notEmpty(listGame) ||
		catName == "graphics" && notEmpty(listGraphics) ||
		catName == "internet-and-network" && notEmpty(listInternetAndNetwork) ||
		catName == "office" && notEmpty(listOffice) ||
		catName == "audio-video" && notEmpty(listAudioVideo) ||
		catName == "system-tools" && notEmpty(listSystemTools) ||
		catName == "other" && notEmpty(listOther)

	return result
}

func notEmpty(listCategory []string) bool {
	if len(listCategory) == 0 {
		return false
	}
	for _, desktopID := range listCategory {
		entry := id2entry[desktopID]
		if !entry.NoDisplay {
			return true
		}
	}
	return false
}

func setUpAppsFlowBox(categoryList []string, searchPhrase string) *gtk.FlowBox {
	if appFlowBox != nil {
		appFlowBox.Destroy()
	}
	flowBox, _ := gtk.FlowBoxNew()
	flowBox.SetMinChildrenPerLine(*columnsNumber)
	flowBox.SetMaxChildrenPerLine(*columnsNumber)
	flowBox.SetColumnSpacing(*itemSpacing)
	flowBox.SetRowSpacing(*itemSpacing)
	flowBox.SetHomogeneous(true)
	flowBox.SetSelectionMode(gtk.SELECTION_NONE)

	for _, entry := range desktopEntries {
		if searchPhrase == "" {
			if !entry.NoDisplay {
				if categoryList != nil {
					if isIn(categoryList, entry.DesktopID) {
						button := flowBoxButton(entry)
						flowBox.Add(button)
					}
				} else {
					button := flowBoxButton(entry)
					flowBox.Add(button)
				}
			}
		} else {
			if !entry.NoDisplay && (strings.Contains(strings.ToLower(entry.NameLoc), strings.ToLower(searchPhrase)) ||
				strings.Contains(strings.ToLower(entry.CommentLoc), strings.ToLower(searchPhrase)) ||
				strings.Contains(strings.ToLower(entry.Comment), strings.ToLower(searchPhrase)) ||
				strings.Contains(strings.ToLower(entry.Exec), strings.ToLower(searchPhrase))) {
				button := flowBoxButton(entry)
				flowBox.Add(button)
			}
		}
	}
	hWrapper, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	appSearchResultWrapper.PackStart(hWrapper, false, false, 0)
	hWrapper.PackStart(flowBox, true, false, 0)
	// While moving focus with arrow keys we want buttons to get focus directly
	flowBox.GetChildren().Foreach(func(item interface{}) {
		item.(*gtk.Widget).SetCanFocus(false)
	})
	resultWindow.ShowAll()

	return flowBox
}

func flowBoxButton(entry desktopEntry) *gtk.Button {
	button, _ := gtk.ButtonNew()
	button.SetAlwaysShowImage(true)

	var pixbuf *gdk.Pixbuf
	var img *gtk.Image
	var err error
	if entry.Icon != "" {
		pixbuf, err = createPixbuf(entry.Icon, *iconSize)
	} else {
		log.Warnf("Undefined icon for %s", entry.Name)
		pixbuf, err = createPixbuf("image-missing", *iconSize)
	}
	if err != nil {
		pixbuf, _ = createPixbuf("unknown", *iconSize)
	}
	img, _ = gtk.ImageNewFromPixbuf(pixbuf)

	button.SetImage(img)
	button.SetImagePosition(gtk.POS_TOP)
	name := entry.NameLoc
	if len(name) > 20 {
		r := []rune(name)
		name = string(r[:17])
		name = fmt.Sprintf("%s…", name)
	}
	button.SetLabel(name)

	ID := entry.DesktopID
	exec := entry.Exec
	terminal := entry.Terminal
	desc := entry.CommentLoc
	button.Connect("button-release-event", func(btn *gtk.Button, e *gdk.Event) bool {
		btnEvent := gdk.EventButtonNewFromEvent(e)
		if btnEvent.Button() == 1 {
			launch(exec, terminal)
			return true
		} else if btnEvent.Button() == 3 {
			pinItem(ID)
			return true
		}
		return false
	})
	button.Connect("activate", func() {
		launch(exec, terminal)
	})
	button.Connect("enter-notify-event", func() {
		statusLabel.SetText(desc)
	})
	return button
}

func setUpFileSearchResultContainer() *gtk.FlowBox {
	if fileSearchResultFlowBox != nil {
		fileSearchResultFlowBox.Destroy()
	}
	flowBox, _ := gtk.FlowBoxNew()
	flowBox.SetProperty("orientation", gtk.ORIENTATION_VERTICAL)
	fileSearchResultWrapper.PackStart(flowBox, false, false, 10)

	return flowBox
}

func walk(path string, d fs.DirEntry, e error) error {
	if e != nil {
		return e
	}
	// don't search leading part of the path, as e.g. '/home/user/Pictures'
	toSearch := strings.Split(path, ignore)[1]

	// Remaing part of the path (w/o file name) must be checked against being present in excluded dirs
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
			fileSearchResults = append(fileSearchResults, fmt.Sprintf("#is_dir#%s", path))
		} else {
			fileSearchResults = append(fileSearchResults, path)
		}
	}

	return nil
}

func setUpSearchEntry() *gtk.SearchEntry {
	searchEntry, _ := gtk.SearchEntryNew()
	searchEntry.SetPlaceholderText("Type to search")
	/*searchEntry.Connect("enter-notify-event", func() {
		cancelClose()
	})*/
	searchEntry.Connect("search-changed", func() {
		for _, btn := range catButtons {
			btn.SetImagePosition(gtk.POS_LEFT)
			btn.SetSizeRequest(0, 0)
		}

		phrase, _ = searchEntry.GetText()
		if len(phrase) > 0 {

			// search apps
			appFlowBox = setUpAppsFlowBox(nil, phrase)

			// search files
			if !*noFS && len(phrase) > 2 {
				if fileSearchResultFlowBox != nil {
					fileSearchResultFlowBox.Destroy()
				}

				fileSearchResultFlowBox = setUpFileSearchResultContainer()

				for key := range userDirsMap {
					if key != "home" {
						fileSearchResults = nil
						searchUserDir(key)
					}
				}
				if fileSearchResultFlowBox.GetChildren().Length() == 0 {
					fileSearchResultWrapper.Hide()
					statusLabel.SetText("0 results")
				}
			} else {
				// search phrase too short
				if fileSearchResultFlowBox != nil {
					fileSearchResultFlowBox.Destroy()
				}
				if fileSearchResultWrapper != nil {
					fileSearchResultWrapper.Hide()
				}
			}
			// focus 1st search result #17
			var w *gtk.Widget
			if appFlowBox != nil {
				b := appFlowBox.GetChildAtIndex(0)
				if b != nil {
					button, err := b.GetChild()
					if err == nil {
						button.ToWidget().GrabFocus()
						w = button.ToWidget()
					}
				}
			}
			if w == nil && fileSearchResultFlowBox != nil {
				f := fileSearchResultFlowBox.GetChildAtIndex(0)
				if f != nil {
					button, err := f.GetChild()
					if err == nil {
						button.ToWidget().SetCanFocus(true)
						button.ToWidget().GrabFocus()
					}
				}
			}
		} else {
			// clear search results
			appFlowBox = setUpAppsFlowBox(nil, "")

			if fileSearchResultFlowBox != nil {
				fileSearchResultFlowBox.Destroy()
			}

			if fileSearchResultWrapper != nil {
				fileSearchResultWrapper.Hide()
			}
		}
	})

	return searchEntry
}

func isExcluded(dir string) bool {
	for _, exclusion := range exclusions {
		if strings.Contains(dir, exclusion) {
			return true
		}
	}
	return false
}

func searchUserDir(dir string) {
	fileSearchResults = nil
	ignore = userDirsMap[dir]
	filepath.WalkDir(userDirsMap[dir], walk)

	if len(fileSearchResults) > 0 {
		btn := setUpUserDirButton(fmt.Sprintf("folder-%s", dir), "", dir, userDirsMap)
		fileSearchResultFlowBox.Add(btn)

		for _, path := range fileSearchResults {
			partOfPathToShow := strings.Split(path, userDirsMap[dir])[1]
			if partOfPathToShow != "" {
				if !(strings.HasPrefix(path, "#is_dir#") && isExcluded(path)) {
					btn := setUpUserFileSearchResultButton(partOfPathToShow, path)
					fileSearchResultFlowBox.Add(btn)
				}

			}
		}
		fileSearchResultFlowBox.Hide()

		statusLabel.SetText(fmt.Sprintf("%v results | LMB: xdg-open | RMB: file manager",
			fileSearchResultFlowBox.GetChildren().Length()))
		num := uint(fileSearchResultFlowBox.GetChildren().Length() / *fsColumns)
		fileSearchResultFlowBox.SetMinChildrenPerLine(num + 1)
		fileSearchResultFlowBox.SetMaxChildrenPerLine(num + 1)
		//While moving focus with arrow keys we want buttons to get focus directly
		fileSearchResultFlowBox.GetChildren().Foreach(func(item interface{}) {
			item.(*gtk.Widget).SetCanFocus(false)
		})
		fileSearchResultFlowBox.ShowAll()
	}
}

func setUpUserDirButton(iconName, displayName, entryName string, userDirsMap map[string]string) *gtk.Box {
	if displayName == "" {
		parts := strings.Split(userDirsMap[entryName], "/")
		displayName = parts[(len(parts) - 1)]
	}
	box, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	button, _ := gtk.ButtonNew()
	button.SetAlwaysShowImage(true)
	img, _ := gtk.ImageNewFromIconName(iconName, gtk.ICON_SIZE_MENU)
	button.SetImage(img)

	if len(displayName) > *nameLimit {
		displayName = fmt.Sprintf("%s…", displayName[:*nameLimit-3])
	}
	button.SetLabel(displayName)

	button.Connect("button-release-event", func(btn *gtk.Button, e *gdk.Event) bool {
		btnEvent := gdk.EventButtonNewFromEvent(e)
		if btnEvent.Button() == 1 {
			open(userDirsMap[entryName], true)
			return true
		} else if btnEvent.Button() == 3 {
			open(userDirsMap[entryName], false)
			return true
		}
		return false
	})

	box.PackStart(button, false, true, 0)
	return box
}

func setUpUserFileSearchResultButton(fileName, filePath string) *gtk.Box {
	box, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	button, _ := gtk.ButtonNew()

	// in the walk function we've marked directories with the '#is_dir#' prefix
	if strings.HasPrefix(filePath, "#is_dir#") {
		filePath = filePath[8:]
		img, _ := gtk.ImageNewFromIconName("folder", gtk.ICON_SIZE_MENU)
		button.SetAlwaysShowImage(true)
		button.SetImage(img)
	}

	tooltipText := ""
	if len(fileName) > *nameLimit {
		tooltipText = fileName
		fileName = fmt.Sprintf("%s…", fileName[:*nameLimit-3])
	}
	button.SetLabel(fileName)
	if tooltipText != "" {
		button.SetTooltipText(tooltipText)
	}

	button.Connect("button-release-event", func(btn *gtk.Button, e *gdk.Event) bool {
		btnEvent := gdk.EventButtonNewFromEvent(e)
		if btnEvent.Button() == 1 {
			open(filePath, true)
			return true
		} else if btnEvent.Button() == 3 {
			open(filePath, false)
			return true
		}
		return false
	})

	button.Connect("activate", func() {
		open(filePath, true)
	})

	box.PackStart(button, false, true, 0)
	return box
}
