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

const version = "0.0.1"

var (
	appDirs                   []string
	configDirectory           string
	pinnedFile                string
	pinned                    []string
	rightBox                  *gtk.Box
	src                       glib.SourceHandle
	imgSizeScaled             int
	currentWsNum, targetWsNum int64
	win                       *gtk.Window
	id2entry                  map[string]desktopEntry
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
	userDirsListBox         *gtk.ListBox
	resultWrapper           *gtk.Box
	resultWindow            *gtk.ScrolledWindow
	fileSearchResults       map[string]string
	fileSearchResultWindow  *gtk.ScrolledWindow
	searchEntry             *gtk.SearchEntry
	phrase                  string
	fileSearchResultListBox *gtk.ListBox
	buttonsWrapper          *gtk.Box
	buttonBox               *gtk.EventBox
	confirmationBox         *gtk.Box
	userDirsMap             map[string]string
	appFlowBox              *gtk.FlowBox
	appFlowBoxWrapper       *gtk.Box
	pinnedFlowBox           *gtk.FlowBox
	pinnedFlowBoxWrapper    *gtk.Box
	catButtons              []*gtk.Button
	statusLabel             *gtk.Label
	status                 string
)

// Flags
var cssFileName = flag.String("s", "drawer.css", "Styling: css file name")
var targetOutput = flag.String("o", "", "name of the Output to display the menu on")
var displayVersion = flag.Bool("v", false, "display Version information")
var iconSizeLarge = flag.Int("isl", 64, "Icon Size Large")
var iconSizeSmall = flag.Int("iss", 16, "Icon Size Small")
var columnsNumber = flag.Uint("c", 6, "number of Columns")
var itemPadding = flag.Uint("padding", 2, "vertical item padding")
var lang = flag.String("lang", "", "force lang, e.g. \"en\", \"pl\"")
var fileManager = flag.String("fm", "thunar", "File Manager")
var term = flag.String("term", "alacritty", "Terminal emulator")

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
		copyFile("/usr/share/nwg-drawer/drawer.css", filepath.Join(configDirectory, "drawer.css"))
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

	cssFile := filepath.Join(configDirectory, *cssFileName)

	appDirs = getAppDirs()

	setUpCategories()

	desktopFiles := listDesktopFiles()
	println(fmt.Sprintf("Found %v desktop files", len(desktopFiles)))

	status = parseDesktopFiles(desktopFiles)

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

	layershell.SetLayer(win, layershell.LAYER_SHELL_LAYER_TOP)

	layershell.SetKeyboardMode(win, layershell.LAYER_SHELL_KEYBOARD_MODE_EXCLUSIVE)

	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	win.Connect("key-release-event", func(window *gtk.Window, event *gdk.Event) {
		key := &gdk.EventKey{Event: event}
		if key.KeyVal() == gdk.KEY_Escape {
			s, _ := searchEntry.GetText()
			if s != "" {
				clearSearchResult()
				searchEntry.GrabFocus()
				searchEntry.SetText("")
			} else {
				if resultWindow == nil || !resultWindow.GetVisible() {
					gtk.MainQuit()
				} else {
					clearSearchResult()
				}
			}
		}
	})

	// Close the window on leave, but not immediately, to avoid accidental closes
	win.Connect("leave-notify-event", func() {
		src, err = glib.TimeoutAdd(uint(500), func() bool {
			gtk.MainQuit()
			return false
		})
	})

	win.Connect("enter-notify-event", func() {
		cancelClose()
	})

	outerVBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	win.Add(outerVBox)

	searchBoxWrapper, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	outerVBox.PackStart(searchBoxWrapper, false, false, 10)

	searchEntry = setUpSearchEntry()
	searchEntry.SetMaxWidthChars(30)
	searchBoxWrapper.PackStart(searchEntry, true, false, 0)
	outerVBox.PackStart(searchBoxWrapper, false, false, 10)

	pinnedWrapper, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	outerVBox.PackStart(pinnedWrapper, false, false, 0)

	pinnedFlowBoxWrapper, _ = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	outerVBox.PackStart(pinnedFlowBoxWrapper, false, false, 0)
	pinnedFlowBox = setUpPinnedFlowBox()

	categoriesWrapper, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	categoriesButtonBox := setUpCategoriesButtonBox()
	categoriesWrapper.PackStart(categoriesButtonBox, true, false, 0)
	outerVBox.PackStart(categoriesWrapper, false, false, 10)

	resultWindow, _ = gtk.ScrolledWindowNew(nil, nil)
	resultWindow.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	resultWindow.Connect("enter-notify-event", func() {
		cancelClose()
	})
	outerVBox.PackStart(resultWindow, true, true, 10)

	appFlowBoxWrapper, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	resultWindow.Add(appFlowBoxWrapper)
	appFlowBox = setUpAppsFlowBox(nil, "")

	statusLineWrapper, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	outerVBox.PackStart(statusLineWrapper, false, false, 10)
	statusLabel, _ = gtk.LabelNew(status)
	statusLineWrapper.PackStart(statusLabel, true, false, 0)

	/*alignmentBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	outerBox.PackStart(alignmentBox, true, true, 0)

	rightColumn, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)

	alignmentBox.PackStart(rightColumn, true, true, 0)

	wrapper, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	searchEntry = setUpSearchEntry()
	searchEntry.SetMaxWidthChars(30)
	wrapper.PackStart(searchEntry, true, false, 0)
	rightColumn.PackStart(wrapper, false, false, 10)

	wrapper, _ = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	pinnedFlowBoxWrapper, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	pinnedFlowBox = setUpPinnedFlowBox()
	wrapper.PackStart(pinnedFlowBoxWrapper, true, true, 0)
	rightColumn.PackStart(wrapper, false, false, 10)

	wrapper, _ = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	categoriesMenuBar := setUpCategoriesButtonBox()
	wrapper.PackStart(categoriesMenuBar, true, false, 0)
	rightColumn.PackStart(wrapper, false, false, 10)

	wrapper, _ = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	resultWrapper, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	wrapper.PackStart(resultWrapper, true, false, 0)

	rightColumn.PackStart(wrapper, true, true, 0)

	resultWindow, _ = gtk.ScrolledWindowNew(nil, nil)
	resultWindow.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	resultWindow.Connect("enter-notify-event", func() {
		cancelClose()
	})
	rightColumn.PackStart(resultWindow, true, true, 0)
	resultWindow.Add(wrapper)
	//resultWrapper.PackStart(resultWindow, true, true, 0)

	appFlowBoxWrapper, _ = gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	resultWindow.Add(appFlowBoxWrapper)
	appFlowBox = setUpAppsFlowBox(nil, "")*/

	win.ShowAll()

	//searchEntry.GrabFocus()
	t := time.Now()
	println(fmt.Sprintf("UI created in %v ms. Thank you for your patience.", t.Sub(timeStart).Milliseconds()))
	gtk.Main()
}
