// Application drawer for wlroots-based Wayland compositors
// Copyright (C) 2021-2025 Piotr Miller & Contributors
// https://github.com/nwg-piotr/nwg-drawer
//
// This program is licensed under the MIT License.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/diamondburned/gotk4-layer-shell/pkg/gtklayershell"
	"github.com/expr-lang/expr"

	"github.com/allan-simon/go-singleinstance"
	log "github.com/sirupsen/logrus"

	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
)

const version = "0.7.4"

var (
	appDirs          []string
	configDirectory  string
	dataDirectory    string
	pinnedFile       string
	pinned           []string
	id2entry         map[string]desktopEntry
	preferredApps    map[string]interface{}
	exclusions       []string
	hyprlandMonitors []monitor
	beenScrolled     bool
	firstPowerBtn    *gtk.Button
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

type monitor struct {
	Id              int     `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Make            string  `json:"make"`
	Model           string  `json:"model"`
	Serial          string  `json:"serial"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	RefreshRate     float64 `json:"refreshRate"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
	ActiveWorkspace struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	} `json:"activeWorkspace"`
	Reserved   []int   `json:"reserved"`
	Scale      float64 `json:"scale"`
	Transform  int     `json:"transform"`
	Focused    bool    `json:"focused"`
	DpmsStatus bool    `json:"dpmsStatus"`
	Vrr        bool    `json:"vrr"`
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
	win                         *gtk.Window
	resultWindow                *gtk.ScrolledWindow
	mathResultWindow            *gtk.Window
	searchEntry                 *gtk.SearchEntry
	phrase                      string
	fileSearchResultFlowBox     *gtk.FlowBox
	fileSearchResultFlowBoxLock *sync.Mutex = &sync.Mutex{}
	fileSearchContextCancel     *context.CancelFunc
	userDirsMap                 map[string]string
	appFlowBox                  *gtk.FlowBox
	appSearchResultWrapper      *gtk.Box
	fileSearchResultWrapper     *gtk.Box
	powerButtonsWrapper         *gtk.Box
	pinnedFlowBox               *gtk.FlowBox
	pinnedFlowBoxWrapper        *gtk.Box
	categoriesWrapper           *gtk.Box
	catButtons                  []*gtk.Button
	statusLabel                 *gtk.Label
	status                      string
	ignore                      string
	desktopTrigger              bool
	pinnedItemsChanged          chan interface{} = make(chan interface{}, 1)
	inRestore                   bool
)

func defaultTermIfBlank(s, fallback string) string {
	s = strings.TrimSpace(s)
	// os.Getenv("TERM") returns "linux" instead of empty string, if program has been started
	// from a key binding defined in the config file. See #23.
	if s == "" || s == "linux" {
		return fallback
	}
	return s
}

func validateWm() {
	if !(*wm == "sway" || *wm == "hyprland" || *wm == "Hyprland" || *wm == "river" || *wm == "niri" || *wm == "uwsm") && *wm != "" {
		*wm = ""
		log.Warn("-wm argument can be only 'sway', 'hyprland', 'river', 'niri' or 'uwsm'")
	}
}

// Flags
var cssFileName = flag.String("s", "drawer.css", "Styling: css file name")
var targetOutput = flag.String("o", "", "name of the Output to display the drawer on (sway & Hyprland only)")
var displayVersion = flag.Bool("v", false, "display Version information")
var keyboard = flag.Bool("k", false, "set GTK layer shell Keyboard interactivity to 'on-demand' mode")
var overlay = flag.Bool("ovl", false, "use OVerLay layer")
var flagDrawerOpen = flag.Bool("open", false, "open drawer of existing instance")
var flagDrawerClose = flag.Bool("close", false, "close drawer of existing instance")
var gtkTheme = flag.String("g", "", "GTK theme name")
var gtkIconTheme = flag.String("i", "", "GTK icon theme name")
var iconSize = flag.Int("is", 64, "Icon Size")
var marginTop = flag.Int("mt", 0, "Margin Top")
var marginLeft = flag.Int("ml", 0, "Margin Left")
var marginRight = flag.Int("mr", 0, "Margin Right")
var marginBottom = flag.Int("mb", 0, "Margin Bottom")
var fsColumns = flag.Uint("fscol", 2, "File Search result COLumns")
var forceTheme = flag.Bool("ft", false, "Force Theme for libadwaita apps, by adding 'GTK_THEME=<default-gtk-theme>' env var; ignored if wm argument == 'uwsm'")
var columnsNumber = flag.Uint("c", 6, "number of Columns")
var itemSpacing = flag.Uint("spacing", 20, "icon spacing")
var lang = flag.String("lang", "", "force lang, e.g. \"en\", \"pl\"")
var fileManager = flag.String("fm", "thunar", "File Manager")
var term = flag.String("term", defaultTermIfBlank(os.Getenv("TERM"), "foot"), "Terminal emulator")
var wm = flag.String("wm", "", "use swaymsg exec (with 'sway' argument) or hyprctl dispatch exec (with 'hyprland') or riverctl spawn (with 'river') or niri msg action spawn -- (with 'niri') or uwsm app -- (with 'uwsm' for Universal Wayland Session Manager) to launch programs")
var nameLimit = flag.Int("fslen", 80, "File Search name LENgth Limit")
var noCats = flag.Bool("nocats", false, "Disable filtering by category")
var noFS = flag.Bool("nofs", false, "Disable file search")
var resident = flag.Bool("r", false, "Leave the program resident in memory")
var pbExit = flag.String("pbexit", "", "command for the Exit power bar icon")
var pbLock = flag.String("pblock", "", "command for the Lock power bar icon")
var pbPoweroff = flag.String("pbpoweroff", "", "command for the Poweroff power bar icon")
var pbReboot = flag.String("pbreboot", "", "command for the Reboot power bar icon")
var pbSleep = flag.String("pbsleep", "", "command for the sleep power bar icon")
var pbSize = flag.Int("pbsize", 64, "power bar icon size (only works w/ built-in icons)")
var pbUseIconTheme = flag.Bool("pbuseicontheme", false, "use icon theme instead of built-in icons in power bar")
var closeBtn = flag.String("closebtn", "none", "close button position: 'left' or 'right', 'none' by default")
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

	validateWm()

	// Gentle SIGTERM handler thanks to reiki4040 https://gist.github.com/reiki4040/be3705f307d3cd136e85
	// v0.2: we also need to support SIGUSR from now on
	showWindowChannel := make(chan interface{}, 1)
	signalChan := make(chan os.Signal, 1)
	const (
		SIG25 = syscall.Signal(0x25) // Which is SIGRTMIN+3 on Linux, it's not used by the system
	)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2, SIG25)
	go func() {
		for {
			s := <-signalChan
			switch s {
			case syscall.SIGTERM:
				log.Info("SIGTERM received, bye bye")
				gtk.MainQuit()
			case syscall.SIGUSR1: // toggle drawer
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
			case syscall.SIGUSR2: // open drawer
				if *resident {
					log.Debug("SIGUSR2 received, showing the window")
					showWindowChannel <- struct{}{}
				} else {
					log.Info("SIGUSR2 received, and I'm not resident but I'm still here, doing nothing")
				}
			case SIG25: // close drawer
				if *resident {
					log.Debug("SIG25 received, hiding the window")
					if win.IsVisible() {
						restoreStateAndHide()
					}
				} else {
					log.Info("A signal received, and I'm not resident, bye bye")
					gtk.MainQuit()
				}
			default:
				log.Infof("Unknown signal: %s", s.String())
			}
		}
	}()

	// If running instance found, we want it to show the window. The new instance will send SIGUSR1 and die
	// (equivalent of `pkill -USR1 nwg-drawer`).
	// Otherwise, the command may behave in two ways:
	// 	1. kill the running non-resident instance and exit;
	// 	2. die if a resident instance found.
	lockFilePath := path.Join(dataHome(), "nwg-drawer.lock")
	lockFile, err := singleinstance.CreateLockFile(lockFilePath)
	if err != nil {
		pid, err := readTextFile(lockFilePath)
		if err == nil {
			i, err := strconv.Atoi(pid)
			if err == nil {
				if *resident {
					log.Warnf("Resident instance already running (PID %v)", i)
				} else {
					var err error
					if *flagDrawerClose {
						log.Infof("Closing resident instance (PID %v)", i)
						err = syscall.Kill(i, SIG25)
					} else if *flagDrawerOpen {
						log.Infof("Showing resident instance (PID %v)", i)
						err = syscall.Kill(i, syscall.SIGUSR2)
					} else {
						log.Infof("Toggling resident instance (PID %v)", i)
						err = syscall.Kill(i, syscall.SIGUSR1)
					}
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
	dataDirectory = dataDir()

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
		err := copyFile(filepath.Join(dataDirectory, "drawer.css"), filepath.Join(configDirectory, "drawer.css"))
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
	gtk.Init()

	settings := gtk.SettingsGetDefault()
	if *gtkTheme != "" {
		settings.SetObjectProperty("gtk-theme-name", *gtkTheme)
		log.Infof("User demanded theme: %s", *gtkTheme)
	} else {
		settings.SetObjectProperty("gtk-application-prefer-dark-theme", true)
		log.Info("Preferring dark theme variants")
	}

	if *gtkIconTheme != "" {
		settings.SetObjectProperty("gtk-icon-theme-name", *gtkIconTheme)
		log.Infof("User demanded icon theme: %s", *gtkIconTheme)
	}

	cssProvider := gtk.NewCSSProvider()

	err = cssProvider.LoadFromPath(*cssFileName)
	if err != nil {
		log.Errorf("ERROR: %s css file not found or erroneous. Using GTK styling.", *cssFileName)
	} else {
		log.Info(fmt.Sprintf("Using style from %s", *cssFileName))
		screen := gdk.ScreenGetDefault()
		gtk.StyleContextAddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}

	win = gtk.NewWindow(gtk.WindowToplevel)
	if win != nil {
		log.Debugf("win addr: %p native: %x", win, win.Native())
	} else {
		log.Panic("Failed creating window")
	}

	if wayland() {
		gtklayershell.InitForWindow(win)
		gtklayershell.SetNamespace(win, "nwg-drawer")

		var output2mon map[string]*gdk.Monitor
		if *targetOutput != "" {
			// We want to assign layershell to a monitor, but we only know the output name!
			output2mon, err = mapOutputs()
			log.Debugf("output2mon: %v", output2mon)
			if err == nil {
				mon := output2mon[*targetOutput]
				gtklayershell.SetMonitor(win, mon)

			} else {
				log.Errorf("%s", err)
			}
		}

		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeBottom, true)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeTop, true)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeLeft, true)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeRight, true)

		if *overlay {
			gtklayershell.SetLayer(win, gtklayershell.LayerShellLayerOverlay)
			gtklayershell.SetExclusiveZone(win, -1)
		} else {
			gtklayershell.SetLayer(win, gtklayershell.LayerShellLayerTop)
		}

		gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeTop, *marginTop)
		gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeLeft, *marginLeft)
		gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeRight, *marginRight)
		gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeBottom, *marginBottom)

		if *keyboard {
			log.Info("Setting GTK layer shell keyboard mode to: on-demand")
			gtklayershell.SetKeyboardMode(win, gtklayershell.LayerShellKeyboardModeOnDemand)
		} else {
			log.Info("Setting GTK layer shell keyboard mode to default: exclusive")
			gtklayershell.SetKeyboardMode(win, gtklayershell.LayerShellKeyboardModeExclusive)
		}

	}

	//win.Connect("destroy", func() {
	//	gtk.MainQuit()
	//})
	win.Connect("destroy", func() {
		shuttingDown = true
		win = nil
	})

	win.Connect("key-release-event", func(_ *gtk.Window, event *gdk.Event) bool {
		//key := &gdk.EventKey{Event: event}
		key := event.AsKey()
		if key.Keyval() == gdk.KEY_Escape {
			s := searchEntry.Text()
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

		} else if key.Keyval() == gdk.KEY_Tab {
			if firstPowerBtn != nil {
				firstPowerBtn.GrabFocus()
			}

		} else if key.Keyval() == gdk.KEY_Return || key.Keyval() == gdk.KEY_KP_Enter {
			s := searchEntry.Text()
			if s != "" {
				// Check if execute command input
				if s[0] == ':' {
					// Make sure there's something to run
					if len(s) > 1 {
						launch(substring(s, 1, -1), false, true)
					}
				} else {
					// Check if the search box content is an arithmetic expression. If so, display the result
					// and copy to the clipboard with wl-copy.
					result, e := expr.Eval(s, nil)
					if e == nil {
						log.Debugf("Setting up mathemathical operation result window. Operation: %s, result: %v", s, result)
						mathResultWindow = setUpOperationResultWindow(s, fmt.Sprintf("%v", result))
					}
				}
			}
			return true
		}
		return true
	})

	win.Connect("key-press-event", func(_ *gtk.Window, event *gdk.Event) bool {
		//key := &gdk.EventKey{Event: event}
		key := event.AsKey()
		switch key.Keyval() {
		case gdk.KEY_downarrow, gdk.KEY_Up, gdk.KEY_Down, gdk.KEY_Left, gdk.KEY_Right, gdk.KEY_Tab,
			gdk.KEY_Return, gdk.KEY_Page_Up, gdk.KEY_Page_Down, gdk.KEY_Home, gdk.KEY_End:
			return false

		default:
			if !searchEntry.IsFocus() {
				searchEntry.GrabFocusWithoutSelecting()
			}
		}
		return false
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
	outerVBox := gtk.NewBox(gtk.OrientationVertical, 0)
	win.Add(outerVBox)

	closeButtonBox := createCloseButtonBox((*closeBtn != "none"), (*closeBtn != "right"))
	if closeButtonBox != nil {
		outerVBox.PackStart(closeButtonBox, false, false, 10)
	}

	searchBoxWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	outerVBox.PackStart(searchBoxWrapper, false, false, 10)

	searchEntry = setUpSearchEntry()
	searchEntry.SetMaxWidthChars(30)
	searchBoxWrapper.PackStart(searchEntry, true, false, 0)

	if !*noCats {
		categoriesWrapper = gtk.NewBox(gtk.OrientationHorizontal, 0)
		categoriesButtonBox := setUpCategoriesButtonBox()
		categoriesWrapper.PackStart(categoriesButtonBox, true, false, 0)
		outerVBox.PackStart(categoriesWrapper, false, false, 0)
	}

	pinnedWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	outerVBox.PackStart(pinnedWrapper, false, false, 0)

	pinnedFlowBoxWrapper = gtk.NewBox(gtk.OrientationHorizontal, 0)
	outerVBox.PackStart(pinnedFlowBoxWrapper, false, false, 0)
	pinnedFlowBox = setUpPinnedFlowBox()

	resultWindow = gtk.NewScrolledWindow(nil, nil)
	resultWindow.SetEvents(int(gdk.AllEventsMask))
	resultWindow.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)

	// On touch screen we don't want the button-release-event to launch the app if the user just wanted to scroll the
	// window. Let's forbid doing so if the content has been scrolled. We will reset the value on button-press-event.
	// Resolves https://github.com/nwg-piotr/nwg-drawer/issues/110
	vAdj := resultWindow.VAdjustment()
	vAdj.Connect("value-changed", func() {
		beenScrolled = true
	})
	hAdj := resultWindow.HAdjustment()
	hAdj.Connect("value-changed", func() {
		beenScrolled = true
	})

	resultWindow.Connect("button-release-event", func(_ *gtk.ScrolledWindow, event *gdk.Event) bool {
		//btnEvent := gdk.EventButtonNewFromEvent(event)
		btnEvent := event.AsButton()
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

	resultsWrapper := gtk.NewBox(gtk.OrientationVertical, 0)
	resultWindow.Add(resultsWrapper)

	appSearchResultWrapper = gtk.NewBox(gtk.OrientationVertical, 0)
	resultsWrapper.PackStart(appSearchResultWrapper, false, false, 0)
	appFlowBox = setUpAppsFlowBox(nil, "")

	// Focus 1st pinned item if any, otherwise focus 1st found app icon
	var button gtk.Widget
	if len(pinnedFlowBox.Children()) > 0 {
		button = pinnedFlowBox.ChildAtIndex(0).Widget
	} else {
		button = appFlowBox.ChildAtIndex(0).Widget
	}
	if err == nil {
		button.GrabFocus()
	}

	userDirsMap = mapXdgUserDirs()
	log.Debugf("User dirs map: %s", userDirsMap)

	placeholder := gtk.NewBox(gtk.OrientationVertical, 0)
	resultsWrapper.PackStart(placeholder, true, true, 0)
	placeholder.SetSizeRequest(20, 20)

	if !*noFS {
		wrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
		fileSearchResultWrapper = gtk.NewBox(gtk.OrientationHorizontal, 0)
		if fileSearchResultWrapper != nil {
			log.Debugf("fileSearchResultWrapper addr: %p native: %x", fileSearchResultWrapper, fileSearchResultWrapper.Native())
		}
		fileSearchResultWrapper.SetObjectProperty("name", "files-box")
		wrapper.PackStart(fileSearchResultWrapper, true, false, 0)
		resultsWrapper.PackEnd(wrapper, false, false, 10)
	}

	// Power Button Bar
	if dataDirectory != "" {
		if *pbExit != "" || *pbLock != "" || *pbPoweroff != "" || *pbReboot != "" || *pbSleep != "" {
			powerBarWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
			outerVBox.PackStart(powerBarWrapper, false, false, 0)
			powerButtonsWrapper = gtk.NewBox(gtk.OrientationHorizontal, 0)
			powerBarWrapper.PackStart(powerButtonsWrapper, true, false, 12)

			if *pbPoweroff != "" {
				btn := gtk.NewButton()
				if !*pbUseIconTheme {
					btn = powerButton(filepath.Join(dataDirectory, "img/poweroff.svg"), *pbPoweroff)
				} else {
					btn = powerButton("system-shutdown-symbolic", *pbPoweroff)
				}
				powerButtonsWrapper.PackEnd(btn, true, false, 0)
				firstPowerBtn = btn
			}
			if *pbSleep != "" {
				btn := gtk.NewButton()
				if !*pbUseIconTheme {
					btn = powerButton(filepath.Join(dataDirectory, "img/sleep.svg"), *pbSleep)
				} else {
					btn = powerButton("face-yawn-symbolic", *pbSleep)
				}
				powerButtonsWrapper.PackEnd(btn, true, false, 0)
				firstPowerBtn = btn
			}
			if *pbReboot != "" {
				btn := gtk.NewButton()
				if !*pbUseIconTheme {
					btn = powerButton(filepath.Join(dataDirectory, "img/reboot.svg"), *pbReboot)
				} else {
					btn = powerButton("system-reboot-symbolic", *pbReboot)
				}
				powerButtonsWrapper.PackEnd(btn, true, false, 0)
				firstPowerBtn = btn
			}
			if *pbExit != "" {
				btn := gtk.NewButton()
				if !*pbUseIconTheme {
					btn = powerButton(filepath.Join(dataDirectory, "img/exit.svg"), *pbExit)
				} else {
					btn = powerButton("system-log-out-symbolic", *pbExit)
				}
				powerButtonsWrapper.PackEnd(btn, true, false, 0)
				firstPowerBtn = btn
			}
			if *pbLock != "" {
				btn := gtk.NewButton()
				if !*pbUseIconTheme {
					btn = powerButton(filepath.Join(dataDirectory, "img/lock.svg"), *pbLock)
				} else {
					btn = powerButton("system-lock-screen-symbolic", *pbLock)
				}
				powerButtonsWrapper.PackEnd(btn, true, false, 0)
				firstPowerBtn = btn
			}
		}
	} else {
		log.Warn("Couldn't find data dir, power bar icons unavailable")
	}

	statusLineWrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	statusLineWrapper.SetObjectProperty("name", "status-line-wrapper")
	outerVBox.PackStart(statusLineWrapper, false, false, 10)
	statusLabel = gtk.NewLabel(status)
	statusLabel.SetObjectProperty("name", "status-label")
	statusLineWrapper.PackStart(statusLabel, true, false, 0)

	win.ShowAll()

	if !*noFS {
		fileSearchResultWrapper.SetSizeRequest(appFlowBox.AllocatedWidth(), 1)
		fileSearchResultWrapper.Hide()
	}
	if !*noCats {
		categoriesWrapper.SetSizeRequest(1, categoriesWrapper.AllocatedHeight()*2)
	}
	if powerButtonsWrapper != nil {
		powerButtonsWrapper.SetSizeRequest(300, 1)
	}
	if *resident && win.IsVisible() {
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
						var button gtk.Widget
						if len(pinnedFlowBox.Children()) > 0 {
							button = pinnedFlowBox.ChildAtIndex(0).Widget
						} else {
							button = appFlowBox.ChildAtIndex(0).Widget
						}
						if err == nil {
							button.GrabFocus()
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

var shuttingDown bool

func restoreStateAndHide() {
	if inRestore {
		log.Warn("restoreStateAndHide already in progress")
		return
	}
	inRestore = true
	defer func() { inRestore = false }()

	if shuttingDown {
		log.Warn("restoreStateAndHide skipped — shutting down")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("restoreStateAndHide panic: %v", r)
		}
	}()

	timeStart := time.Now()

	if mathResultWindow != nil {
		mathResultWindow.Destroy()
		mathResultWindow = nil
	}

	// Hide the window via glib.IdleAdd to avoid calling Hide() outside the main thread
	if win != nil {
		winPtr := win
		glib.IdleAdd(func() {
			if shuttingDown || winPtr == nil {
				log.Debug("IdleAdd: skip win.Hide() — win destroyed or shutting down")
				return
			}
			log.Debugf("IdleAdd: hiding win: native=%x", winPtr.Native())
			winPtr.Hide()
		})
	}

	// Reset search entry
	if searchEntry != nil && searchEntry.Native() != 0 {
		searchEntry.SetText("")
	}

	// Rebuild FlowBox
	appFlowBox = setUpAppsFlowBox(nil, "")

	// Reset category buttons
	for _, btn := range catButtons {
		if btn != nil && btn.Native() != 0 {
			btn.SetImagePosition(gtk.PosLeft)
			btn.SetSizeRequest(0, 0)
		}
	}

	// Scroll up
	if resultWindow != nil && resultWindow.Native() != 0 {
		resultWindow.VAdjustment().SetValue(0)
	}

	log.Debugf("UI hidden and restored in %d ms", time.Since(timeStart).Milliseconds())
}
