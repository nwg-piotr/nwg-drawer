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
	flowBox.SetMaxChildrenPerLine(6)
	flowBox.SetColumnSpacing(20)
	flowBox.SetHomogeneous(true)
	flowBox.SetRowSpacing(20)

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
				name = fmt.Sprintf("%s...", name[:17])
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

			flowBox.Add(btn)
		}
	}

	flowBox.Connect("enter-notify-event", func() {
		cancelClose()
	})

	pinnedFlowBoxWrapper.PackStart(flowBox, true, true, 0)
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
	eventBox.Connect("enter-notify-event", func() {
		cancelClose()
	})
	hBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	eventBox.Add(hBox)
	button, _ := gtk.ButtonNewWithLabel("All")
	button.Connect("clicked", func(item *gtk.Button) {
		//clearSearchResult()
		searchEntry.GrabFocus()
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
				//clearSearchResult()
				searchEntry.GrabFocus()
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

func setUpBackButton() *gtk.Box {
	vBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	hBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	vBox.PackStart(hBox, false, false, 0)
	button, _ := gtk.ButtonNew()
	button.SetCanFocus(false)
	pixbuf, _ := createPixbuf("arrow-left", *iconSizeLarge)
	image, _ := gtk.ImageNewFromPixbuf(pixbuf)
	button.SetImage(image)
	button.SetAlwaysShowImage(true)
	button.Connect("enter-notify-event", func() {
		cancelClose()
	})
	button.Connect("clicked", func(btn *gtk.Button) {
		clearSearchResult()
		searchEntry.GrabFocus()
		searchEntry.SetText("")
	})
	hBox.PackEnd(button, false, true, 0)

	return vBox
}

func setUpCategoryListBox(listCategory []string) *gtk.ListBox {
	listBox, _ := gtk.ListBoxNew()

	for _, desktopID := range listCategory {
		entry := id2entry[desktopID]
		name := entry.NameLoc
		if name == "" {
			name = entry.Name
		}
		if len(name) > 30 {
			name = fmt.Sprintf("%s...", name[:27])
		}
		if !entry.NoDisplay {
			row, _ := gtk.ListBoxRowNew()
			row.SetSelectable(false)
			vBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
			eventBox, _ := gtk.EventBoxNew()
			hBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
			eventBox.Add(hBox)
			vBox.PackStart(eventBox, false, false, *itemPadding)

			ID := entry.DesktopID
			eventBox.Connect("button-release-event", func(row *gtk.ListBoxRow, e *gdk.Event) bool {
				btnEvent := gdk.EventButtonNewFromEvent(e)
				if btnEvent.Button() == 1 {
					launch(entry.Exec, entry.Terminal)
					return true
				} else if btnEvent.Button() == 3 {
					pinItem(ID)
				}
				return false
			})

			pixbuf, _ := createPixbuf(entry.Icon, *iconSizeLarge)
			img, _ := gtk.ImageNewFromPixbuf(pixbuf)
			hBox.PackStart(img, false, false, 0)

			lbl, _ := gtk.LabelNew(name)
			hBox.PackStart(lbl, false, false, 0)

			row.Add(vBox)
			listBox.Add(row)
		}
	}
	backButton.Show()
	return listBox
}

func setUpCategorySearchResult(searchPhrase string) *gtk.ListBox {
	listBox, _ := gtk.ListBoxNew()

	resultWindow, _ = gtk.ScrolledWindowNew(nil, nil)
	resultWindow.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	resultWindow.Connect("enter-notify-event", func() {
		cancelClose()
	})
	resultWrapper.PackStart(resultWindow, true, true, 0)

	counter := 0
	for _, entry := range desktopEntries {
		if len(searchPhrase) == 1 && counter > 9 {
			break
		} else if len(searchPhrase) == 2 && counter > 14 {
			break
		}
		if !entry.NoDisplay && (strings.Contains(strings.ToLower(entry.NameLoc), strings.ToLower(searchPhrase)) ||
			strings.Contains(strings.ToLower(entry.CommentLoc), strings.ToLower(searchPhrase)) ||
			strings.Contains(strings.ToLower(entry.Comment), strings.ToLower(searchPhrase))) {

			counter++

			row, _ := gtk.ListBoxRowNew()
			row.SetSelectable(false)
			vBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
			eventBox, _ := gtk.EventBoxNew()
			hBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
			eventBox.Add(hBox)
			vBox.PackStart(eventBox, false, false, *itemPadding)

			exec := entry.Exec
			term := entry.Terminal
			ID := entry.DesktopID
			row.Connect("activate", func() {
				launch(exec, term)
			})
			eventBox.Connect("button-release-event", func(row *gtk.EventBox, e *gdk.Event) bool {
				btnEvent := gdk.EventButtonNewFromEvent(e)
				if btnEvent.Button() == 1 {
					launch(exec, term)
					return true
				} else if btnEvent.Button() == 3 {
					pinItem(ID)
				}
				return false
			})

			pixbuf, _ := createPixbuf(entry.Icon, *iconSizeLarge)
			img, _ := gtk.ImageNewFromPixbuf(pixbuf)
			hBox.PackStart(img, false, false, 0)

			name := entry.NameLoc
			if len(name) > 45 {
				name = fmt.Sprintf("%s...", name[:42])
			}

			lbl, _ := gtk.LabelNew(name)
			hBox.PackStart(lbl, false, false, 0)

			row.Add(vBox)
			listBox.Add(row)

		}
	}
	resultWindow.Add(listBox)
	resultWindow.ShowAll()
	return listBox
}

func setUpAppsFlowBox(categoryList []string, searchPhrase string) *gtk.FlowBox {
	if appFlowBox != nil {
		appFlowBox.Destroy()
	}
	flowBox, _ := gtk.FlowBoxNew()
	flowBox.SetMinChildrenPerLine(6)
	flowBox.SetColumnSpacing(20)
	flowBox.SetRowSpacing(20)
	for _, entry := range desktopEntries {
		if categoryList != nil {
			if !entry.NoDisplay && isIn(categoryList, entry.DesktopID) {
				button := flowBoxButton(entry)
				flowBox.Add(button)
			}
		} else {
			if !entry.NoDisplay {
				button := flowBoxButton(entry)
				flowBox.Add(button)
			}
		}
	}
	appFlowBoxWrapper.PackStart(flowBox, false, false, 0)
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
		name = fmt.Sprintf("%s...", name[:17])
	}
	button.SetLabel(name)

	ID := entry.DesktopID
	exec := entry.Exec
	terminal := entry.Terminal
	button.Connect("button-release-event", func(row *gtk.Button, e *gdk.Event) bool {
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
	return button
}

func setUpFileSearchResult() *gtk.ListBox {
	listBox, _ := gtk.ListBoxNew()
	if fileSearchResultWindow != nil {
		fileSearchResultWindow.Destroy()
	}
	fileSearchResultWindow, _ = gtk.ScrolledWindowNew(nil, nil)
	fileSearchResultWindow.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	fileSearchResultWindow.Connect("enter-notify-event", func() {
		cancelClose()
	})
	resultWrapper.PackStart(fileSearchResultWindow, true, true, 0)

	fileSearchResultWindow.Add(listBox)
	fileSearchResultWindow.ShowAll()

	return listBox
}

func walk(path string, d fs.DirEntry, e error) error {
	if e != nil {
		return e
	}
	if !d.IsDir() {
		parts := strings.Split(path, "/")
		fileName := parts[len(parts)-1]
		if strings.Contains(strings.ToLower(fileName), strings.ToLower(phrase)) {
			fileSearchResults[fileName] = path
		}
	}
	return nil
}

func setUpSearchEntry() *gtk.SearchEntry {
	searchEntry, _ := gtk.SearchEntryNew()
	searchEntry.Connect("enter-notify-event", func() {
		cancelClose()
	})
	searchEntry.Connect("search-changed", func() {
		phrase, _ = searchEntry.GetText()
		if len(phrase) > 0 {
			userDirsListBox.Hide()
			backButton.Show()

			if resultWindow != nil {
				resultWindow.Destroy()
			}
			resultListBox = setUpCategorySearchResult(phrase)
			if resultListBox.GetChildren().Length() == 0 {
				resultWindow.Hide()
			}

			if len(phrase) > 2 {
				if fileSearchResultWindow != nil {
					fileSearchResultWindow.Destroy()
				}
				fileSearchResultListBox = setUpFileSearchResult()
				for key := range userDirsMap {
					if key != "home" {
						fileSearchResults = make(map[string]string)
						if len(fileSearchResults) == 0 {
							fileSearchResultListBox.Show()
						}
						filepath.WalkDir(userDirsMap[key], walk)
						searchUserDir(key)
					}
				}
				if fileSearchResultListBox.GetChildren().Length() == 0 {
					fileSearchResultWindow.Hide()
				}
			} else {
				if fileSearchResultWindow != nil {
					fileSearchResultWindow.Destroy()
				}
			}

		} else {
			clearSearchResult()
			userDirsListBox.ShowAll()
		}

	})
	searchEntry.Connect("focus-in-event", func() {
		searchEntry.SetText("")
	})

	return searchEntry
}

func searchUserDir(dir string) {
	fileSearchResults = make(map[string]string)
	filepath.WalkDir(userDirsMap[dir], walk)
	if len(fileSearchResults) > 0 {
		/*row := setUpUserDirsListRow(fmt.Sprintf("folder-%s", dir), "", dir, userDirsMap)
		fileSearchResultListBox.Add(row)
		fileSearchResultListBox.ShowAll()*/

		for _, path := range fileSearchResults {
			row := setUpUserFileSearchResultRow(path, path)
			fileSearchResultListBox.Add(row)
		}
		fileSearchResultListBox.ShowAll()
	}
}

func setUpUserFileSearchResultRow(fileName, filePath string) *gtk.ListBoxRow {
	row, _ := gtk.ListBoxRowNew()
	//row.SetCanFocus(false)
	row.SetSelectable(false)
	vBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	eventBox, _ := gtk.EventBoxNew()
	hBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	eventBox.Add(hBox)
	vBox.PackStart(eventBox, false, false, *itemPadding)

	/*if len(fileName) > 45 {
		fileName = fmt.Sprintf("%s...", fileName[:42])
	}*/
	lbl, _ := gtk.LabelNew(fileName)
	hBox.PackStart(lbl, false, false, 0)
	row.Add(vBox)

	row.Connect("activate", func() {
		open(filePath)
	})

	eventBox.Connect("button-release-event", func(row *gtk.ListBoxRow, e *gdk.Event) bool {
		btnEvent := gdk.EventButtonNewFromEvent(e)
		if btnEvent.Button() == 1 {
			open(filePath)
			return true
		}
		return false
	})

	return row
}

func clearSearchResult() {
	if resultWindow != nil {
		resultWindow.Destroy()
	}
	if fileSearchResultWindow != nil {
		fileSearchResultWindow.Destroy()
	}
	if userDirsListBox != nil {
		userDirsListBox.ShowAll()
	}
	if categoriesListBox != nil {
		sr := categoriesListBox.GetSelectedRow()
		if sr != nil {
			categoriesListBox.GetSelectedRow().SetSelectable(false)
		}
		categoriesListBox.UnselectAll()
	}
	backButton.Hide()
	//searchEntry.SetText("")
	//searchEntry.GrabFocus()
}
