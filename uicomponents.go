package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

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
	} else {
		flowBox.SetMaxChildrenPerLine(uint(len(pinned)))
	}

	flowBox.SetColumnSpacing(20)
	flowBox.SetHomogeneous(true)
	flowBox.SetRowSpacing(20)
	flowBox.SetProperty("name", "pinned-box")
	flowBox.SetSelectionMode(gtk.SELECTION_NONE)

	if len(pinned) > 0 {
		for _, desktopID := range pinned {
			entry := id2entry[desktopID]

			btn, _ := gtk.ButtonNew()
			pixbuf, _ := createPixbuf(entry.Icon, *iconSizeLarge)
			img, err := gtk.ImageNewFromPixbuf(pixbuf)
			if err != nil {
				println(err, entry.Icon)
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
				name = fmt.Sprintf("%s ...", name[:17])
			}
			btn.SetLabel(name)

			btn.Connect("button-release-event", func(row *gtk.Button, e *gdk.Event) bool {
				btnEvent := gdk.EventButtonNewFromEvent(e)
				if btnEvent.Button() == 1 {
					launch(entry.Exec, entry.Terminal)
					return true
				} else if btnEvent.Button() == 3 {
					unpinItem(entry.DesktopID)
					pinnedFlowBox = setUpPinnedFlowBox()
					return true
				}
				return false
			})
			btn.Connect("activate", func() {
				launch(entry.Exec, entry.Terminal)
			})

			flowBox.Add(btn)
		}
	}

	flowBox.Connect("enter-notify-event", func() {
		cancelClose()
	})

	pinnedFlowBoxWrapper.PackStart(flowBox, true, false, 0)
	//While moving focus with arrow keys we want buttons to get focus directly
	flowBox.GetChildren().Foreach(func(item interface{}) {
		item.(*gtk.Widget).SetCanFocus(false)
	})

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
	eventBox.Connect("enter-notify-event", func() {
		cancelClose()
	})
	hBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	eventBox.Add(hBox)
	button, _ := gtk.ButtonNewWithLabel("All")
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
			catButtons = append(catButtons, button)
			button.SetLabel(cat.DisplayName)
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
		if entry.NoDisplay == false {
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
	flowBox.SetColumnSpacing(20)
	flowBox.SetRowSpacing(20)
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

	pixbuf, _ := createPixbuf(entry.Icon, *iconSizeLarge)
	img, _ := gtk.ImageNewFromPixbuf(pixbuf)
	button.SetImage(img)
	button.SetImagePosition(gtk.POS_TOP)
	name := entry.NameLoc
	if len(name) > 20 {
		name = fmt.Sprintf("%s ...", name[:17])
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
			pinnedFlowBox = setUpPinnedFlowBox()
		}
		return false
	})
	button.Connect("activate", func() {
		launch(exec, terminal)
	})
	button.Connect("enter-notify-event", func() {
		statusLabel.SetText(desc)
	})
	button.Connect("leave-notify-event", func() {
		statusLabel.SetText(status)
	})
	return button
}

func setUpFileSearchResult() *gtk.FlowBox {
	if fileSearchResultFlowBox != nil {
		fileSearchResultFlowBox.Destroy()
	}
	flowBox, _ := gtk.FlowBoxNew()
	flowBox.SetProperty("orientation", gtk.ORIENTATION_VERTICAL)
	flowBox.Connect("enter-notify-event", func() {
		cancelClose()
	})
	fileSearchResultWrapper.PackStart(flowBox, false, false, 10)
	flowBox.ShowAll() // TODO: check if necessary here

	return flowBox
}

func walk(path string, d fs.DirEntry, e error) error {
	if e != nil {
		return e
	}
	//if !d.IsDir() {
	// don't search leading part of the path, as e.g. '/home/user/Pictures'
	toSearch := strings.Split(path, ignore)[1]
	if strings.Contains(strings.ToLower(toSearch), strings.ToLower(phrase)) {
		// mark directories
		if d.IsDir() {
			fileSearchResults = append(fileSearchResults, fmt.Sprintf("#is_dir#%s", path))
		} else {
			fileSearchResults = append(fileSearchResults, path)
		}
	}

	//}
	return nil
}

func setUpSearchEntry() *gtk.SearchEntry {
	searchEntry, _ := gtk.SearchEntryNew()
	searchEntry.SetPlaceholderText("Type to search")
	searchEntry.Connect("enter-notify-event", func() {
		cancelClose()
	})
	searchEntry.Connect("search-changed", func() {
		for _, btn := range catButtons {
			btn.SetImagePosition(gtk.POS_LEFT)
			btn.SetSizeRequest(0, 0)
		}

		phrase, _ = searchEntry.GetText()
		if len(phrase) > 0 {

			appFlowBox = setUpAppsFlowBox(nil, phrase)

			if len(phrase) > 2 {
				if fileSearchResultFlowBox != nil {
					fileSearchResultFlowBox.Destroy()
				}
				fileSearchResultFlowBox = setUpFileSearchResult()
				for key := range userDirsMap {
					if key != "home" {
						fileSearchResults = nil
						if len(fileSearchResults) == 0 {
							fileSearchResultFlowBox.Show()
						}
						searchUserDir(key)
					}
				}
				if fileSearchResultFlowBox.GetChildren().Length() == 0 {
					fileSearchResultFlowBox.Hide()
				}
			} else {
				if fileSearchResultFlowBox != nil {
					fileSearchResultFlowBox.Destroy()
				}
			}
		} else {
			if fileSearchResultFlowBox != nil {
				fileSearchResultFlowBox.Destroy()
			}
			appFlowBox = setUpAppsFlowBox(nil, "")
		}
	})
	searchEntry.Connect("focus-in-event", func() {
		searchEntry.SetText("")
	})

	return searchEntry
}

func searchUserDir(dir string) {
	fileSearchResults = nil
	ignore = userDirsMap[dir]
	filepath.WalkDir(userDirsMap[dir], walk)

	if fileSearchResults != nil && len(fileSearchResults) > 0 {
		btn := setUpUserDirButton(fmt.Sprintf("folder-%s", dir), "", dir, userDirsMap)
		fileSearchResultFlowBox.Add(btn)

		for _, path := range fileSearchResults {
			partOfPathToShow := strings.Split(path, userDirsMap[dir])[1]
			if partOfPathToShow != "" {
				btn := setUpUserFileSearchResultButton(partOfPathToShow, path)
				fileSearchResultFlowBox.Add(btn)
			}
		}
		fileSearchResultFlowBox.Hide()

		statusLabel.SetText(fmt.Sprintf("%v results", fileSearchResultFlowBox.GetChildren().Length()))
		num := uint(fileSearchResultFlowBox.GetChildren().Length() / 3)
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
	img, _ := gtk.ImageNewFromIconName(iconName, gtk.ICON_SIZE_MENU)
	button.SetImage(img)

	if len(displayName) > *nameLimit {
		displayName = fmt.Sprintf("%s...", displayName[:*nameLimit-3])
	}
	button.SetLabel(displayName)

	button.Connect("clicked", func() {
		launch(fmt.Sprintf("%s %s", *fileManager, userDirsMap[entryName]), false)
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
		button.SetImage(img)
	}

	tooltipText := ""
	if len(fileName) > *nameLimit {
		tooltipText = fileName
		fileName = fmt.Sprintf("%s...", fileName[:*nameLimit-3])
	}
	button.SetLabel(fileName)
	if tooltipText != "" {
		button.SetTooltipText(tooltipText)
	}

	button.Connect("clicked", func() {
		open(filePath)
	})

	box.PackStart(button, false, true, 0)
	return box
}
