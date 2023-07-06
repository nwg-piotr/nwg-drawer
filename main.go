package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/allan-simon/go-singleinstance"
	"github.com/dlasky/gotk3-layershell/layershell"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

const version = "0.3.9"

var (
	appDirs         []string
	configDirectory string
	pinnedFile      string
	pinned          []string
	id2entry        map[string]desktopEntry
	preferredApps   map[string]interface{}
	exclusions      []string
)

var categoryNames = [...]string{
	"utility",
	"development",
	"game",
	"graphics",
	"internet-and-network",
	"office",
	"audio-video",
	"system-tools",
	"other",
}

type category struct {
	Name        string
	DisplayName string
	Icon        string
}

var categories []category

type desktopEntry struct {
	DesktopID  string
	Name       string
	NameLoc    string
	Comment    string
	CommentLoc string
	Icon       string
	Exec       string
	Category   string
	Terminal   bool
	NoDisplay  bool
}

// slices below will hold DesktopID strings
var (
	listUtility            []string
	listDevelopment        []string
	listGame               []string
	listGraphics           []string
	listInternetAndNetwork []string
	listOffice             []string
	listAudioVideo         []string
	listSystemTools        []string
	listOther              []string
)

var desktopEntries []desktopEntry

// UI elements
var (
	win                     *gtk.Window
	resultWindow            *gtk.ScrolledWindow
	fileSearchResults       []string
	searchEntry             *gtk.SearchEntry
	phrase                  string
	fileSearchResultFlowBox *gtk.FlowBox
	userDirsMap             map[string]string
	appFlowBox              *gtk.FlowBox
	appSearchResultWrapper  *gtk.Box
	fileSearchResultWrapper *gtk.Box
	pinnedFlowBox           *gtk.FlowBox
	pinnedFlowBoxWrapper    *gtk.Box
	categoriesWrapper       *gtk.Box
	catButtons              []*gtk.Button
	statusLabel             *gtk.Label
	status                  string
	ignore                  string
	desktopTrigger          bool
	pinnedItemsChanged      chan interface{} = make(chan interface{}, 1)
)

func defaultStringIfBlank(s, fallback string) string {
	s = strings.TrimSpace(s)
	// os.Getenv("TERM") returns "linux" instead of empty string, if program has been started
	// from a key binding defined in the config file. See #23.
	if s == "" || s == "linux" {
		return fallback
	}
	return s
}

// Flags
var cssFileName = flag.String("s", "drawer.css", "Styling: css file name")
var targetOutput = flag.String("o", "", "name of the Output to display the drawer on (sway only)")
var displayVersion = flag.Bool("v", false, "display Version information")
var overlay = flag.Bool("ovl", false, "use OVerLay layer")
var gtkTheme = flag.String("g", "", "GTK theme name")
var gtkIconTheme = flag.String("i", "", "GTK icon theme name")
var iconSize = flag.Int("is", 64, "Icon Size")
var marginTop = flag.Int("mt", 0, "Margin Top")
var marginLeft = flag.Int("ml", 0, "Margin Left")
var marginRight = flag.Int("mr", 0, "Margin Right")
var marginBottom = flag.Int("mb", 0, "Margin Bottom")
var fsColumns = flag.Uint("fscol", 2, "File Search result COLumns")
var columnsNumber = flag.Uint("c", 6, "number of Columns")
var itemSpacing = flag.Uint("spacing", 20, "icon spacing")
var lang = flag.String("lang", "", "force lang, e.g. \"en\", \"pl\"")
var fileManager = flag.String("fm", "thunar", "File Manager")
var term = flag.String("term", defaultStringIfBlank(os.Getenv("TERM"), "foot"), "Terminal emulator")
var nameLimit = flag.Int("fslen", 80, "File Search name LENgth Limit")
var noCats = flag.Bool("nocats", false, "Disable filtering by category")
var noFS = flag.Bool("nofs", false, "Disable file search")
var resident = flag.Bool("r", false, "Leave the program resident in memory")
var debug = flag.Bool("d", false, "Turn on Debug messages")

func main() {
	timeStart := time.Now()
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	if *displayVersion {
		fmt.Printf("nwg-drawer version %s\n", version)
		os.Exit(0)
	}

	// Gentle SIGTERM handler thanks to reiki4040 https://gist.github.com/reiki4040/be3705f307d3cd136e85
	// v0.2: we also need to support SIGUSR from now on
	showWindowChannel := make(chan interface{}, 1)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGUSR1)

	go func() {
		for {
			s := <-signalChan
			switch s {
			case syscall.SIGTERM:
				log.Info("SIGTERM received, bye bye")
				gtk.MainQuit()
			case syscall.SIGUSR1:
				if *resident {
					// As win.Show() called from inside a goroutine randomly crashes GTK,
					// let's just set e helper variable here. We'll be checking it with glib.TimeoutAdd.
					if !win.IsVisible() {
						log.Debug("SIGUSR1 received, showing the window")
						showWindowChannel <- struct{}{}
					} else {
						log.Debug("SIGUSR1 received, hiding the window")
						restoreStateAndHide()
					}
				} else {
					log.Info("SIGUSR1 received, and I'm not resident, bye bye")
					gtk.MainQuit()
				}
			default:
				log.Infof("Unknown signal: %s", s.String())
			}
		}
	}()

	// If running instance found, we want it to show the window. The new instance will send SIGUSR1 and die
	// (equivalent of `pkill -USR1 nwg-drawer`).
	// Otherwise the command may behave in two ways:
	// 	1. kill the running non-residennt instance and exit;
	// 	2. die if a resident instance found.
	lockFilePath := path.Join(dataDir(), "nwg-drawer.lock")
	lockFile, err := singleinstance.CreateLockFile(lockFilePath)
	if err != nil {
		pid, err := readTextFile(lockFilePath)
		if err == nil {
			i, err := strconv.Atoi(pid)
			if err == nil {
				if *resident {
					log.Warnf("Resident instance already running (PID %v)", i)
				} else {
					log.Infof("Showing resident instance (PID %v)", i)
					err := syscall.Kill(i, syscall.SIGUSR1)
					if err != nil {
						return
					}
				}
			}
		}
		os.Exit(0)
	}
	defer lockFile.Close()

	log.Infof("term: %s", *term)

	// LANGUAGE
	if *lang == "" && os.Getenv("LANG") != "" {
		*lang = strings.Split(os.Getenv("LANG"), ".")[0]
	}
	log.Info(fmt.Sprintf("lang: %s", *lang))

	// ENVIRONMENT
	configDirectory = configDir()

	// Placing the drawer config files in the nwg-panel config directory was a mistake.
	// Let's move them to their own location.
	oldConfigDirectory, err := oldConfigDir()
	if err == nil {
		for _, p := range []string{"drawer.css", "preferred-apps.json"} {
			if pathExists(path.Join(oldConfigDirectory, p)) {
				log.Infof("File %s found in stale location, moving to %s", p, configDirectory)
				if !pathExists(path.Join(configDirectory, p)) {
					err = os.Rename(path.Join(oldConfigDirectory, p), path.Join(configDirectory, p))
					if err == nil {
						log.Info("Success")
					} else {
						log.Warn(err)
					}
				} else {
					log.Warnf("Failed moving %s to %s: path already exists!", path.Join(oldConfigDirectory, p), path.Join(configDirectory, p))
				}

			}
		}
	}

	// Copy default style sheet if not found
	if !pathExists(filepath.Join(configDirectory, "drawer.css")) {
		err := copyFile("/usr/share/nwg-drawer/drawer.css", filepath.Join(configDirectory, "drawer.css"))
		if err != nil {
			log.Errorf("Failed copying 'drawer.css' file: %s", err)
		}
	}

	cacheDirectory := cacheDir()
	if cacheDirectory == "" {
		log.Panic("Couldn't determine cache directory location")
	}

	// DATA
	pinnedFile = filepath.Join(cacheDirectory, "nwg-pin-cache")
	pinned, err = loadTextFile(pinnedFile)
	if err != nil {
		pinned = nil
		savePinned()
	}
	log.Info(fmt.Sprintf("Found %v pinned items", len(pinned)))

	if !strings.HasPrefix(*cssFileName, "/") {
		*cssFileName = filepath.Join(configDirectory, *cssFileName)
	}

	appDirs = getAppDirs()

	setUpCategories()

	desktopFiles := listDesktopFiles()
	log.Info(fmt.Sprintf("Found %v desktop files", len(desktopFiles)))

	status = parseDesktopFiles(desktopFiles)

	// For opening files we use xdg-open. As its configuration is PITA, we may override some associations
	// in the ~/.config/nwg-panel/preferred-apps.json file.
	paFile := path.Join(configDirectory, "preferred-apps.json")
	if pathExists(paFile) {
		preferredApps, err = loadPreferredApps(paFile)
		if err != nil {
			log.Infof("Custom associations file %s not found or invalid", paFile)
		} else {
			log.Infof("Found %v associations in %s", len(preferredApps), paFile)
		}
	} else {
		log.Infof("%s file not found", paFile)
	}

	// Load user-defined paths excluded from file search
	exFile := path.Join(configDirectory, "excluded-dirs")
	if pathExists(exFile) {
		exclusions, err = loadTextFile(exFile)
		if err != nil {
			log.Infof("Search exclusions file %s not found %s", exFile, err)
		} else {
			log.Infof("Found %v search exclusions in %s", len(exclusions), exFile)
		}
	} else {
		log.Infof("%s file not found", exFile)
	}

	// USER INTERFACE
	gtk.Init(nil)

	settings, _ := gtk.SettingsGetDefault()
	if *gtkTheme != "" {
		err = settings.SetProperty("gtk-theme-name", *gtkTheme)
		if err != nil {
			log.Error("Unable to set theme:", err)
		} else {
			log.Infof("User demanded theme: %s", *gtkTheme)
		}
	} else {
		err := settings.SetProperty("gtk-application-prefer-dark-theme", true)
		if err != nil {
			log.Error("Error setting 'gtk-application-prefer-dark-theme' property")
			return
		}
		log.Info("Preferring dark theme variants")
	}

	if *gtkIconTheme != "" {
		err = settings.SetProperty("gtk-icon-theme-name", *gtkIconTheme)
		if err != nil {
			log.Error("Unable to set icon theme:", err)
		} else {
			log.Infof("User demanded icon theme: %s", *gtkIconTheme)
		}
	}

	cssProvider, _ := gtk.CssProviderNew()

	err = cssProvider.LoadFromPath(*cssFileName)
	if err != nil {
		log.Errorf("ERROR: %s css file not found or erroneous. Using GTK styling.", *cssFileName)
		log.Errorf("%s", err)
	} else {
		log.Info(fmt.Sprintf("Using style from %s", *cssFileName))
		screen, _ := gdk.ScreenGetDefault()
		gtk.AddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}

	win, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatal("Unable to create window:", err)
	}

	if wayland() {
		layershell.InitForWindow(win)

		var output2mon map[string]*gdk.Monitor
		if *targetOutput != "" {
			// We want to assign layershell to a monitor, but we only know the output name!
			output2mon, err = mapOutputs()
			if err == nil {
				monitor := output2mon[*targetOutput]
				layershell.SetMonitor(win, monitor)

			} else {
				log.Errorf("%s", err)
			}
		}

		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_BOTTOM, true)
		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_TOP, true)
		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_LEFT, true)
		layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_RIGHT, true)

		if *overlay {
			layershell.SetLayer(win, layershell.LAYER_SHELL_LAYER_OVERLAY)
			layershell.SetExclusiveZone(win, -1)
		} else {
			layershell.SetLayer(win, layershell.LAYER_SHELL_LAYER_TOP)
		}

		layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_TOP, *marginTop)
		layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_LEFT, *marginLeft)
		layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_RIGHT, *marginRight)
		layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_BOTTOM, *marginBottom)

		layershell.SetKeyboardMode(win, layershell.LAYER_SHELL_KEYBOARD_MODE_EXCLUSIVE)
	}

	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	win.Connect("key-release-event", func(_ *gtk.Window, event *gdk.Event) bool {
		key := &gdk.EventKey{Event: event}
		if key.KeyVal() == gdk.KEY_Escape {
			s, _ := searchEntry.GetText()
			if s != "" {
				searchEntry.GrabFocus()
				searchEntry.SetText("")
			} else {
				if !*resident {
					gtk.MainQuit()
				} else {
					restoreStateAndHide()
				}
			}
			return true
		}
		return false
	})

	win.Connect("key-press-event", func(_ *gtk.Window, event *gdk.Event) bool {
		key := &gdk.EventKey{Event: event}
		switch key.KeyVal() {
		case gdk.KEY_downarrow, gdk.KEY_Up, gdk.KEY_Down, gdk.KEY_Left, gdk.KEY_Right, gdk.KEY_Tab,
			gdk.KEY_Return, gdk.KEY_Page_Up, gdk.KEY_Page_Down, gdk.KEY_Home, gdk.KEY_End:
			return false

		default:
			if !searchEntry.IsFocus() {
				searchEntry.GrabFocusWithoutSelecting()
			}
			return false
		}
	})

	/*
		In case someone REALLY needed to use X11 - for some stupid Zoom meeting or something, this allows
		the drawer to behave properly on Openbox, and possibly somewhere else. For sure not on i3.
		This feature is not really supported and will stay undocumented.
	*/
	if !wayland() {
		log.Info("Not Wayland, oh really?")
		win.SetDecorated(false)
		win.Maximize()
	}

	// Set up UI
	outerVBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	win.Add(outerVBox)

	searchBoxWrapper, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	outerVBox.PackStart(searchBoxWrapper, false, false, 10)

	searchEntry = setUpSearchEntry()
	searchEntry.SetMaxWidthChars(30)
	searchBoxWrapper.PackStart(searchEntry, true, false, 0)

	if !*noCats {
		categoriesWrapper, _ = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
		categoriesButtonBox := setUpCategoriesButtonBox()
		categoriesWrapper.PackStart(categoriesButtonBox, true, false, 0)
		outerVBox.PackStart(categoriesWrapper, false, false, 0)
	}

	pinnedWrapper, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	outerVBox.PackStart(pinnedWrapper, false, false, 0)

	pinnedFlowBoxWrapper, _ = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	outerVBox.PackStart(pinnedFlowBoxWrapper, false, false, 0)
	pinnedFlowBox = setUpPinnedFlowBox()

	resultWindow, _ = gtk.ScrolledWindowNew(nil, nil)
	resultWindow.SetEvents(int(gdk.ALL_EVENTS_MASK))
	resultWindow.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)

	resultWindow.Connect("button-release-event", func(_ *gtk.ScrolledWindow, event *gdk.Event) bool {
		btnEvent := gdk.EventButtonNewFromEvent(event)
		if btnEvent.Button() == 3 {
			if !*resident {
				gtk.MainQuit()
			} else {
				restoreStateAndHide()
			}
			return true
		}
		return false
	})
	outerVBox.PackStart(resultWindow, true, true, 10)

	resultsWrapper, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	resultWindow.Add(resultsWrapper)

	appSearchResultWrapper, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	resultsWrapper.PackStart(appSearchResultWrapper, false, false, 0)
	appFlowBox = setUpAppsFlowBox(nil, "")

	// Focus 1st pinned item if any, otherwise focus 1st found app icon
	var button gtk.IWidget
	if pinnedFlowBox.GetChildren().Length() > 0 {
		button, err = pinnedFlowBox.GetChildAtIndex(0).GetChild()
	} else {
		button, err = appFlowBox.GetChildAtIndex(0).GetChild()
	}
	if err == nil {
		button.ToWidget().GrabFocus()
	}

	userDirsMap = mapXdgUserDirs()
	log.Debugf("User dirs map: %s", userDirsMap)

	placeholder, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	resultsWrapper.PackStart(placeholder, true, true, 0)
	placeholder.SetSizeRequest(20, 20)

	if !*noFS {
		wrapper, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
		fileSearchResultWrapper, _ = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
		fileSearchResultWrapper.SetProperty("name", "files-box")
		wrapper.PackStart(fileSearchResultWrapper, true, false, 0)
		resultsWrapper.PackEnd(wrapper, false, false, 10)
	}

	statusLineWrapper, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	outerVBox.PackStart(statusLineWrapper, false, false, 10)
	statusLabel, _ = gtk.LabelNew(status)
	statusLineWrapper.PackStart(statusLabel, true, false, 0)

	win.ShowAll()

	if !*noFS {
		fileSearchResultWrapper.SetSizeRequest(appFlowBox.GetAllocatedWidth(), 1)
		fileSearchResultWrapper.Hide()
	}
	if !*noCats {
		categoriesWrapper.SetSizeRequest(1, categoriesWrapper.GetAllocatedHeight()*2)
	}
	if *resident {
		win.Hide()
	}

	t := time.Now()
	log.Info(fmt.Sprintf("UI created in %v ms. Thank you for your patience.", t.Sub(timeStart).Milliseconds()))

	// Check if showing the window has been requested (SIGUSR1)
	go func() {
		for {
			select {
			case <-showWindowChannel:
				log.Debug("Showing window")
				glib.TimeoutAdd(0, func() bool {
					if win != nil && !win.IsVisible() {

						// Refresh files before displaying the root window
						// some .desktop file changed
						if desktopTrigger {
							log.Debug(".desktop file changed")
							desktopFiles = listDesktopFiles()
							status = parseDesktopFiles(desktopFiles)
							appFlowBox = setUpAppsFlowBox(nil, "")
							desktopTrigger = false
						}

						// Show window and focus the search box
						win.ShowAll()
						if fileSearchResultWrapper != nil {
							fileSearchResultWrapper.Hide()
						}
						// focus 1st element
						var button gtk.IWidget
						if pinnedFlowBox.GetChildren().Length() > 0 {
							button, err = pinnedFlowBox.GetChildAtIndex(0).GetChild()
						} else {
							button, err = appFlowBox.GetChildAtIndex(0).GetChild()
						}
						if err == nil {
							button.ToWidget().GrabFocus()
						}
					}

					return false
				})

			case <-pinnedItemsChanged:
				glib.TimeoutAdd(0, func() bool {
					log.Debug("pinned file changed")
					pinned, _ = loadTextFile(pinnedFile)
					pinnedFlowBox = setUpPinnedFlowBox()

					return false
				})
			}
		}
	}()

	go watchFiles()

	gtk.Main()
}

func restoreStateAndHide() {
	timeStart1 := time.Now()
	if win != nil {
		win.Hide()
	}

	// clear search
	searchEntry.SetText("")

	// clear category filter (in gotk3 it means: rebuild, as we have no filtering here)
	appFlowBox = setUpAppsFlowBox(nil, "")
	for _, btn := range catButtons {
		btn.SetImagePosition(gtk.POS_LEFT)
		btn.SetSizeRequest(0, 0)
	}

	// scroll to the top
	if resultWindow != nil {
		resultWindow.GetVAdjustment().SetValue(0)
	}

	t := time.Now()
	log.Debugf(fmt.Sprintf("UI hidden and restored in the backgroud in %v ms", t.Sub(timeStart1).Milliseconds()))
}
