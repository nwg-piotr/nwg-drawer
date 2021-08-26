package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/allan-simon/go-singleinstance"
	"github.com/dlasky/gotk3-layershell/layershell"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

const version = "0.1.8"

var (
	appDirs         []string
	configDirectory string
	pinnedFile      string
	pinned          []string
	src             glib.SourceHandle
	id2entry        map[string]desktopEntry
	preferredApps   map[string]interface{}
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
)

// Flags
var cssFileName = flag.String("s", "drawer.css", "Styling: css file name")
var targetOutput = flag.String("o", "", "name of the Output to display the drawer on (sway only)")
var displayVersion = flag.Bool("v", false, "display Version information")
var overlay = flag.Bool("ovl", false, "use OVerLay layer")
var iconSize = flag.Int("is", 64, "Icon Size")
var fsColumns = flag.Uint("fscol", 2, "File Search result COLumns")
var columnsNumber = flag.Uint("c", 6, "number of Columns")
var itemSpacing = flag.Uint("spacing", 20, "icon spacing")
var lang = flag.String("lang", "", "force lang, e.g. \"en\", \"pl\"")
var fileManager = flag.String("fm", "thunar", "File Manager")
var term = flag.String("term", "alacritty", "Terminal emulator")
var nameLimit = flag.Int("fslen", 80, "File Search name length Limit")
var noCats = flag.Bool("nocats", false, "Disable filtering by category")
var noFS = flag.Bool("nofs", false, "Disable file search")

func main() {
	timeStart := time.Now()
	flag.Parse()

	if *displayVersion {
		fmt.Printf("nwg-drawer version %s\n", version)
		os.Exit(0)
	}

	// Gentle SIGTERM handler thanks to reiki4040 https://gist.github.com/reiki4040/be3705f307d3cd136e85
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	go func() {
		for {
			s := <-signalChan
			if s == syscall.SIGTERM {
				println("SIGTERM received, bye bye!")
				gtk.MainQuit()
			}
		}
	}()

	// We want the same key/mouse binding to turn the dock off: kill the running instance and exit.
	lockFilePath := fmt.Sprintf("%s/nwg-drawer.lock", tempDir())
	lockFile, err := singleinstance.CreateLockFile(lockFilePath)
	if err != nil {
		pid, err := readTextFile(lockFilePath)
		if err == nil {
			i, err := strconv.Atoi(pid)
			if err == nil {
				println("Running instance found, sending SIGTERM and exiting...")
				syscall.Kill(i, syscall.SIGTERM)
			}
		}
		os.Exit(0)
	}
	defer lockFile.Close()

	// LANGUAGE
	if *lang == "" && os.Getenv("LANG") != "" {
		*lang = strings.Split(os.Getenv("LANG"), ".")[0]
	}
	println(fmt.Sprintf("lang: %s", *lang))

	// ENVIRONMENT
	configDirectory = configDir()

	if !pathExists(filepath.Join(configDirectory, "drawer.css")) {
		copyFile(filepath.Join(getDataHome(), "nwg-drawer/drawer.css"), filepath.Join(configDirectory, "drawer.css"))
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
	}
	println(fmt.Sprintf("Found %v pinned items", len(pinned)))

	cssFile := filepath.Join(configDirectory, *cssFileName)

	appDirs = getAppDirs()

	setUpCategories()

	desktopFiles := listDesktopFiles()
	println(fmt.Sprintf("Found %v desktop files", len(desktopFiles)))

	status = parseDesktopFiles(desktopFiles)

	// For opening files we use xdg-open. As its configuration is PITA, we may override some associations
	// in the ~/.config/nwg-panel/preferred-apps.json file.
	paFile := filepath.Join(configDirectory, "preferred-apps.json")
	preferredApps, err = loadPreferredApps(paFile)
	if err != nil {
		println(fmt.Sprintf("Custom associations file %s not found or invalid", paFile))
	} else {
		println(fmt.Sprintf("Found %v associations in %s", len(preferredApps), paFile))
		fmt.Println(preferredApps)
	}

	// USER INTERFACE
	gtk.Init(nil)

	cssProvider, _ := gtk.CssProviderNew()

	err = cssProvider.LoadFromPath(cssFile)
	if err != nil {
		println(fmt.Sprintf("ERROR: %s css file not found or erroneous. Using GTK styling.", cssFile))
		println(fmt.Sprintf(">>> %s", err))
	} else {
		println(fmt.Sprintf("Using style from %s", cssFile))
		screen, _ := gdk.ScreenGetDefault()
		gtk.AddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}

	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
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
				println(err)
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

		layershell.SetKeyboardMode(win, layershell.LAYER_SHELL_KEYBOARD_MODE_EXCLUSIVE)
	}

	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	win.Connect("key-press-event", func(window *gtk.Window, event *gdk.Event) bool {
		key := &gdk.EventKey{Event: event}
		switch key.KeyVal() {
		case gdk.KEY_Escape:
			s, _ := searchEntry.GetText()
			if s != "" {
				searchEntry.GrabFocus()
				searchEntry.SetText("")
			} else {
				gtk.MainQuit()
			}
			return false
		case gdk.KEY_downarrow, gdk.KEY_Up, gdk.KEY_Down, gdk.KEY_Left, gdk.KEY_Right, gdk.KEY_Tab,
			gdk.KEY_Return, gdk.KEY_Page_Up, gdk.KEY_Page_Down, gdk.KEY_Home, gdk.KEY_End:
			return false

		default:
			if !searchEntry.IsFocus() {
				searchEntry.GrabFocus()
			}
			return false
		}
	})

	// Close the window on leave, but not immediately, to avoid accidental closes
	win.Connect("leave-notify-event", func() {
		src = glib.TimeoutAdd(uint(500), func() bool {
			gtk.MainQuit()
			return false
		})
	})

	win.Connect("enter-notify-event", func() {
		cancelClose()
	})

	/*
		In case someone REALLY needed to use X11 - for some stupid Zoom meeting or something, this allows
		the drawer to behave properly on Openbox, and possibly somewhere else. For sure not on i3.
		This feature is not really supported and will stay undocumented.
	*/
	if !wayland() {
		println("Not Wayland, oh really?")
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
	resultWindow.Connect("enter-notify-event", func() {
		cancelClose()
	})
	resultWindow.Connect("button-release-event", func(sw *gtk.ScrolledWindow, e *gdk.Event) bool {
		btnEvent := gdk.EventButtonNewFromEvent(e)
		if btnEvent.Button() == 1 || btnEvent.Button() == 3 {
			gtk.MainQuit()
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

	t := time.Now()
	println(fmt.Sprintf("UI created in %v ms. Thank you for your patience.", t.Sub(timeStart).Milliseconds()))
	gtk.Main()
}
