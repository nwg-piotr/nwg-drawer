package main

import (
	"fmt"
	"github.com/diamondburned/gotk4-layer-shell/pkg/gtklayershell"
	"io/fs"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
)

func setUpPinnedFlowBox() *gtk.FlowBox {
	if pinnedFlowBox != nil {
		pinnedFlowBox.Destroy()
	}
	flowBox := gtk.NewFlowBox()
	if uint(len(pinned)) >= *columnsNumber {
		flowBox.SetMaxChildrenPerLine(*columnsNumber)
	} else if len(pinned) > 0 {
		flowBox.SetMaxChildrenPerLine(uint(len(pinned)))
	}

	flowBox.SetColumnSpacing(*itemSpacing)
	flowBox.SetRowSpacing(*itemSpacing)
	flowBox.SetHomogeneous(true)
	flowBox.SetObjectProperty("name", "pinned-box")
	flowBox.SetSelectionMode(gtk.SelectionNone)

	if len(pinned) > 0 {
		for _, desktopID := range pinned {
			entry := id2entry[desktopID]
			if entry.DesktopID == "" {
				log.Debugf("Pinned item doesn't seem to exist: %s", desktopID)
				continue
			}

			btn := gtk.NewButton()

			var img *gtk.Image
			if entry.Icon != "" {
				pixbuf, _ := createPixbuf(entry.Icon, *iconSize)
				img = gtk.NewImageFromPixbuf(pixbuf)
			} else {
				img = gtk.NewImageFromIconName("image-missing", int(gtk.IconSizeInvalid))
			}

			btn.SetImage(img)
			btn.SetAlwaysShowImage(true)
			btn.SetImagePosition(gtk.PosTop)

			name := ""
			if entry.NameLoc != "" {
				name = entry.NameLoc
			} else {
				name = entry.Name
			}
			if len(name) > 20 {
				r := substring(name, 0, 17)
				name = fmt.Sprintf("%s…", r)
			}
			btn.SetLabel(name)

			btn.Connect("button-release-event", func(row *gtk.Button, event *gdk.Event) bool {
				btnEvent := event.AsButton()
				if btnEvent.Button() == 1 {
					launch(entry.Exec, entry.Terminal, true)
					return true
				} else if btnEvent.Button() == 3 {
					unpinItem(entry.DesktopID)
					return true
				}
				return false
			})
			btn.Connect("activate", func() {
				launch(entry.Exec, entry.Terminal, true)
			})
			btn.Connect("enter-notify-event", func() {
				statusLabel.SetText(entry.CommentLoc)
			})
			btn.Connect("focus-in-event", func() {
				statusLabel.SetText(entry.CommentLoc)
			})
			flowBox.Add(btn)
			btn.Parent().(*gtk.FlowBoxChild).SetCanFocus(false)
		}
		pinnedFlowBoxWrapper.PackStart(flowBox, true, false, 0)
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

	eventBox := gtk.NewEventBox()

	hBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	eventBox.Add(hBox)
	button := gtk.NewButtonWithLabel("All")
	button.SetObjectProperty("name", "category-button")
	button.Connect("clicked", func(item *gtk.Button) {
		searchEntry.SetText("")
		appFlowBox = setUpAppsFlowBox(nil, "")
		for _, btn := range catButtons {
			btn.SetImagePosition(gtk.PosLeft)
			btn.SetSizeRequest(0, 0)
		}
	})
	hBox.PackStart(button, false, false, 0)

	for _, cat := range categories {
		if isSupposedToShowUp(cat.Name) {
			button = gtk.NewButtonFromIconName(cat.Icon, int(gtk.IconSizeMenu))
			button.SetObjectProperty("name", "category-button")
			catButtons = append(catButtons, button)
			button.SetLabel(cat.DisplayName)
			button.SetAlwaysShowImage(true)
			hBox.PackStart(button, false, false, 0)
			name := cat.Name
			b := *button
			button.Connect("clicked", func(item *gtk.Button) {
				searchEntry.SetText("")
				// One day or another we'll add SetFilterFunction here; it was impossible on the gotk3 library
				appFlowBox = setUpAppsFlowBox(lists[name], "")
				for _, btn := range catButtons {
					btn.SetImagePosition(gtk.PosLeft)
				}
				w := b.AllocatedWidth()
				b.SetImagePosition(gtk.PosTop)
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
	flowBox := gtk.NewFlowBox()
	flowBox.SetMinChildrenPerLine(*columnsNumber)
	flowBox.SetMaxChildrenPerLine(*columnsNumber)
	flowBox.SetColumnSpacing(*itemSpacing)
	flowBox.SetRowSpacing(*itemSpacing)
	flowBox.SetHomogeneous(true)
	flowBox.SetSelectionMode(gtk.SelectionNone)

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
					button.Parent().(*gtk.FlowBoxChild).SetCanFocus(false)
				}
			}
		} else {
			if !entry.NoDisplay && (strings.Contains(strings.ToLower(entry.NameLoc), strings.ToLower(searchPhrase)) ||
				strings.Contains(strings.ToLower(entry.CommentLoc), strings.ToLower(searchPhrase)) ||
				strings.Contains(strings.ToLower(entry.Comment), strings.ToLower(searchPhrase)) ||
				strings.Contains(strings.ToLower(entry.Exec), strings.ToLower(searchPhrase))) {
				button := flowBoxButton(entry)
				flowBox.Add(button)
				button.Parent().(*gtk.FlowBoxChild).SetCanFocus(false)
			}
		}
	}
	hWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	appSearchResultWrapper.PackStart(hWrapper, false, false, 0)
	hWrapper.PackStart(flowBox, true, false, 0)

	resultWindow.ShowAll()

	return flowBox
}

func flowBoxButton(entry desktopEntry) *gtk.Button {
	button := gtk.NewButton()
	button.SetAlwaysShowImage(true)

	var pixbuf *gdkpixbuf.Pixbuf
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
	img = gtk.NewImageFromPixbuf(pixbuf)

	button.SetImage(img)
	button.SetImagePosition(gtk.PosTop)
	name := entry.NameLoc
	if len(name) > 20 {
		r := substring(name, 0, 17)
		name = fmt.Sprintf("%s…", r)
	}
	button.SetLabel(name)

	ID := entry.DesktopID
	exec := entry.Exec
	terminal := entry.Terminal
	desc := entry.CommentLoc
	if len(desc) > 120 {
		r := substring(desc, 0, 117)
		desc = fmt.Sprintf("%s…", r)
	}

	button.Connect("button-press-event", func() {
		// if not scrolled from now on, we will allow launching apps on button-release-event
		beenScrolled = false
	})

	button.Connect("button-release-event", func(btn *gtk.Button, event *gdk.Event) bool {
		btnEvent := event.AsButton()
		if btnEvent.Button() == 1 {
			if !beenScrolled {
				launch(exec, terminal, true)
				return true
			}
		} else if btnEvent.Button() == 3 {
			pinItem(ID)
			return true
		}
		return false
	})
	button.Connect("activate", func() {
		launch(exec, terminal, true)
	})
	button.Connect("enter-notify-event", func() {
		statusLabel.SetText(desc)
	})
	button.Connect("leave-notify-event", func() {
		statusLabel.SetText("")
	})
	button.Connect("focus-in-event", func() {
		statusLabel.SetText(desc)
	})
	return button
}

func powerButton(iconPathOrName, command string) *gtk.Button {
	button := gtk.NewButton()
	button.SetAlwaysShowImage(true)

	var pixbuf *gdkpixbuf.Pixbuf
	var img *gtk.Image
	var err error
	if !*pbUseIconTheme {
		pixbuf, err = gdkpixbuf.NewPixbufFromFileAtSize(iconPathOrName, *pbSize, *pbSize)
		if err != nil {
			pixbuf, _ = createPixbuf("unknown", *pbSize)
			log.Warnf("Couldn't find icon %s", iconPathOrName)
		}
		img = gtk.NewImageFromPixbuf(pixbuf)
	} else {
		img = gtk.NewImageFromIconName(iconPathOrName, int(gtk.IconSizeDialog))
	}

	button.SetImage(img)
	button.SetImagePosition(gtk.PosTop)

	button.Connect("button-release-event", func(btn *gtk.Button, event *gdk.Event) bool {
		btnEvent := event.AsButton()
		if btnEvent.Button() == 1 {
			launch(command, false, true)
			return true
		}
		return false
	})
	button.Connect("activate", func() {
		launch(command, false, true)
	})
	button.Connect("enter-notify-event", func() {
		statusLabel.SetText(command)
	})
	button.Connect("leave-notify-event", func() {
		statusLabel.SetText("")
	})
	button.Connect("focus-in-event", func() {
		statusLabel.SetText(command)
	})
	return button
}

func createCloseButtonBox(show bool, alignLeft bool) *gtk.Box {
  if (!show) {
    return nil
  }

  buttonBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
  if alignLeft {
    buttonBox.SetHAlign(gtk.AlignStart)
  } else {
    buttonBox.SetHAlign(gtk.AlignEnd)
  }

  closeButton := gtk.NewButtonFromIconName("window-close-symbolic", int(gtk.IconSizeMenu))
  closeButton.SetRelief(gtk.ReliefNone)
  closeButton.SetObjectProperty("name", "close-button")

  closeButton.Connect("clicked", func() {
      gtk.MainQuit()
  })

  if alignLeft {
    buttonBox.PackStart(closeButton, false, false, 10)
  } else {
    buttonBox.PackEnd(closeButton, false, false, 10)
  }  
  return buttonBox
}

func setUpFileSearchResultContainer() *gtk.FlowBox {
	if fileSearchResultFlowBox != nil {
		fileSearchResultFlowBox.Destroy()
	}
	flowBox := gtk.NewFlowBox()
	flowBox.SetObjectProperty("orientation", gtk.OrientationVertical)
	fileSearchResultWrapper.PackStart(flowBox, false, false, 10)

	return flowBox
}

func walk(path string, d fs.DirEntry, e error) error {
	if e != nil {
		return e
	}
	// don't search leading part of the path, as e.g. '/home/user/Pictures'
	toSearch := strings.Split(path, ignore)[1]

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
			fileSearchResults = append(fileSearchResults, fmt.Sprintf("#is_dir#%s", path))
		} else {
			fileSearchResults = append(fileSearchResults, path)
		}
	}

	return nil
}

func setUpSearchEntry() *gtk.SearchEntry {
	sEntry := gtk.NewSearchEntry()
	sEntry.SetPlaceholderText("Type to search")
	sEntry.Connect("search-changed", func() {
		for _, btn := range catButtons {
			btn.SetImagePosition(gtk.PosLeft)
			btn.SetSizeRequest(0, 0)
		}

		phrase = sEntry.Text()
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
				if len(fileSearchResultFlowBox.Children()) == 0 {
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
			var w *gtk.Button
			if appFlowBox != nil {
				b := appFlowBox.ChildAtIndex(0)
				if b != nil {
					button := b.Child().(*gtk.Button)
					button.SetCanFocus(true)
					button.GrabFocus()
					w = button
				}
			}
			if w == nil && fileSearchResultFlowBox != nil {
				f := fileSearchResultFlowBox.ChildAtIndex(0)
				if f != nil {
					button := f.Child().(*gtk.Box)
					button.SetCanFocus(true)
					button.GrabFocus()
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

	return sEntry
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
		btn.Parent().(*gtk.FlowBoxChild).SetCanFocus(false)

		for _, path := range fileSearchResults {
			log.Debugf("Path: %s", path)
			partOfPathToShow := strings.Split(path, userDirsMap[dir])[1]
			if partOfPathToShow != "" {
				if !(strings.HasPrefix(path, "#is_dir#") && isExcluded(path)) {
					button := setUpUserFileSearchResultButton(partOfPathToShow, path)
					fileSearchResultFlowBox.Add(button)
					button.Parent().(*gtk.FlowBoxChild).SetCanFocus(false)
				}

			}
		}
		fileSearchResultFlowBox.Hide()

		statusLabel.SetText(fmt.Sprintf("%v results | LMB: xdg-open | RMB: file manager",
			len(fileSearchResultFlowBox.Children())))
		num := uint(len(fileSearchResultFlowBox.Children())) / *fsColumns
		fileSearchResultFlowBox.SetMinChildrenPerLine(num + 1)
		fileSearchResultFlowBox.SetMaxChildrenPerLine(num + 1)

		fileSearchResultFlowBox.ShowAll()
	}
}

func setUpUserDirButton(iconName, displayName, entryName string, userDirsMap map[string]string) *gtk.Box {
	if displayName == "" {
		parts := strings.Split(userDirsMap[entryName], "/")
		displayName = parts[(len(parts) - 1)]
	}
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	button := gtk.NewButton()
	button.SetAlwaysShowImage(true)
	img := gtk.NewImageFromIconName(iconName, int(gtk.IconSizeMenu))
	button.SetImage(img)

	if len(displayName) > *nameLimit {
		displayName = fmt.Sprintf("%s…", displayName[:*nameLimit-3])
	}
	button.SetLabel(displayName)

	button.Connect("button-release-event", func(btn *gtk.Button, event *gdk.Event) bool {
		btnEvent := event.AsButton()
		if btnEvent.Button() == 1 {
			open(userDirsMap[entryName], true)
			return true
		} else if btnEvent.Button() == 3 {
			open(userDirsMap[entryName], false)
			return true
		}
		return false
	})

	button.Connect("activate", func() {
		open(userDirsMap[entryName], true)
	})

	box.PackStart(button, false, true, 0)
	return box
}

func setUpUserFileSearchResultButton(fileName, filePath string) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	button := gtk.NewButton()

	// in the walk function we've marked directories with the '#is_dir#' prefix
	if strings.HasPrefix(filePath, "#is_dir#") {
		filePath = filePath[8:]
		img := gtk.NewImageFromIconName("folder", int(gtk.IconSizeMenu))
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

	button.Connect("button-release-event", func(btn *gtk.Button, event *gdk.Event) bool {
		btnEvent := event.AsButton()
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

func setUpOperationResultWindow(operation string, result string) {
	window := gtk.NewWindow(gtk.WindowToplevel)
	window.SetModal(true)

	if wayland() {
		gtklayershell.InitForWindow(window)
		gtklayershell.SetLayer(window, gtklayershell.LayerShellLayerOverlay)
		gtklayershell.SetKeyboardMode(window, gtklayershell.LayerShellKeyboardModeExclusive)
	}

	// any key to close the window
	window.Connect("key-release-event", func(_ *gtk.Window, event *gdk.Event) bool {
		window.Destroy()
		return true
	})

	// any button to close the window
	window.Connect("button-release-event", func(_ *gtk.Window, event *gdk.Event) bool {
		window.Destroy()
		return true
	})

	outerVBox := gtk.NewBox(gtk.OrientationVertical, 6)
	window.Add(outerVBox)

  // close button mainly for touch users
  //closeButtonBox := createCloseButtonBox()
  //outerVBox.PackStart(closeButtonBox, false, false, 10)

	vBox := gtk.NewBox(gtk.OrientationHorizontal, 5)
	outerVBox.PackStart(vBox, true, true, 6)
	lbl := gtk.NewLabel(fmt.Sprintf("%s = %s", operation, result))
	lbl.SetObjectProperty("name", "math-label")
	vBox.PackStart(lbl, true, true, 12)

	mRefProvider := gtk.NewCSSProvider()
	css := "window { background-color: rgba (0, 0, 0, 255); color: #fff; border: solid 1px grey; border-radius: 5px}"
	err := mRefProvider.LoadFromData(css)
	if err != nil {
		log.Warn(err)
	}
	ctx := window.StyleContext()
	ctx.AddProvider(mRefProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

	window.ShowAll()

	if wayland() {
		cmd := fmt.Sprintf("wl-copy %v", result)
		launch(cmd, false, false)
	}
}
