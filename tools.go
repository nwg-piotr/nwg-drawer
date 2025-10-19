package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/joshuarubin/go-sway"
	log "github.com/sirupsen/logrus"

	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	"github.com/google/shlex"
)

func wayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland"
}

func createPixbuf(icon string, size int) (*gdkpixbuf.Pixbuf, error) {
	iconTheme := gtk.IconThemeGetDefault()

	if strings.Contains(icon, "/") {
		pixbuf, err := gdkpixbuf.NewPixbufFromFileAtSize(icon, size, size)
		if err != nil {
			log.Errorf("%s", err)
			return nil, err
		}
		return pixbuf, nil

	} else if strings.HasSuffix(icon, ".svg") || strings.HasSuffix(icon, ".png") || strings.HasSuffix(icon, ".xpm") {
		// for entries like "Icon=netflix-desktop.svg"
		icon = strings.Split(icon, ".")[0]
	}

	pixbuf, err := iconTheme.LoadIcon(icon, size, gtk.IconLookupForceSize)

	if err != nil {
		if strings.HasPrefix(icon, "/") {
			pixbuf, e := gdkpixbuf.NewPixbufFromFileAtSize(icon, size, size)
			if e != nil {
				return nil, e
			}
			return pixbuf, nil
		}

		pixbuf, err := iconTheme.LoadIcon(icon, size, gtk.IconLookupForceSize)
		if err != nil {
			return nil, err
		}
		return pixbuf, nil
	}
	return pixbuf, nil
}

func extractXDGUserDir(s string) string {
	re := regexp.MustCompile(`_([^_]+)_`)
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func mapXdgUserDirs() map[string]string {
	result := make(map[string]string)
	home := os.Getenv("HOME")

	result["home"] = home
	result["documents"] = filepath.Join(home, "Documents")
	result["downloads"] = filepath.Join(home, "Downloads")
	result["music"] = filepath.Join(home, "Music")
	result["pictures"] = filepath.Join(home, "Pictures")
	result["videos"] = filepath.Join(home, "Videos")

	userDirsFile := filepath.Join(filepath.Join(configHome(), "user-dirs.dirs"))
	if pathExists(userDirsFile) {
		log.Debugf("userDirsFile found: %s", userDirsFile)
		log.Info(fmt.Sprintf("Using XDG user dirs from %s", userDirsFile))
		lines, _ := loadTextFile(userDirsFile)
		for _, l := range lines {
			if strings.HasPrefix(l, "XDG_DOCUMENTS_DIR") {
				result["documents"] = getUserDir(home, l)
				continue
			}
			if strings.HasPrefix(l, "XDG_DOWNLOAD_DIR") {
				result["downloads"] = getUserDir(home, l)
				continue
			}
			if strings.HasPrefix(l, "XDG_MUSIC_DIR") {
				result["music"] = getUserDir(home, l)
				continue
			}
			if strings.HasPrefix(l, "XDG_PICTURES_DIR") {
				result["pictures"] = getUserDir(home, l)
				continue
			}
			if strings.HasPrefix(l, "XDG_VIDEOS_DIR") {
				result["videos"] = getUserDir(home, l)
				continue
			}
			if extractXDGUserDir(l) != "" {
				key := extractXDGUserDir(l)
				log.Info(fmt.Sprintf("Found additional XDG user dir: %s", key))
				result[key] = getUserDir(home, l)
			}

		}
	} else {
		log.Warnf("userDirsFile %s not found, using defaults", userDirsFile)
	}

	return result
}

func getUserDir(home, line string) string {
	// line is supposed to look like XDG_DOCUMENTS_DIR="$HOME/Dokumenty"
	result := strings.Split(line, "=")[1]
	result = strings.Replace(result, "$HOME", home, 1)

	// trim ""
	return result[1 : len(result)-1]
}

func cacheDir() string {
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return xdgCache
	}
	if home := os.Getenv("HOME"); home != "" && pathExists(filepath.Join(home, ".cache")) {
		p := filepath.Join(home, ".cache")
		return p
	}
	return ""
}

func readTextFile(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func oldConfigDir() (string, error) {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		dir := path.Join(xdgConfig, "nwg-panel")
		return dir, nil
	} else if home := os.Getenv("HOME"); home != "" {
		dir := path.Join(home, ".config/nwg-panel")
		return dir, nil
	}

	return "", errors.New("old config dir not found")
}

func configDir() string {
	var dir string
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		dir = path.Join(xdgConfig, "nwg-drawer")
	} else if home := os.Getenv("HOME"); home != "" {
		dir = path.Join(home, ".config/nwg-drawer")
	}

	log.Infof("Config dir: %s", dir)
	createDir(dir)

	return dir
}

func configHome() string {
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		return os.Getenv("XDG_CONFIG_HOME")
	}
	return path.Join(os.Getenv("HOME"), ".config")
}

func dataHome() string {
	var dir string
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		dir = path.Join(xdgData, "nwg-drawer")
	} else if home := os.Getenv("HOME"); home != "" {
		dir = path.Join(home, ".local/share/nwg-drawer")
	}

	log.Debugf("Data home: %s", dir)
	createDir(dir)

	return dir
}

func createDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, os.ModePerm)
		if err == nil {
			log.Infof("Creating dir: %s", dir)
		}
	}
}

func copyFile(src, dst string) error {
	log.Infof("Copying: '%s' => '%s'", src, dst)

	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer func(srcfd *os.File) {
		err := srcfd.Close()
		if err != nil {
			log.Errorf("Error closing file: %v", srcfd)
		}
	}(srcfd)

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer func(dstfd *os.File) {
		err := dstfd.Close()
		if err != nil {
			log.Errorf("Error closing file: %v", dstfd)
		}
	}(dstfd)

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}

func dataDir() string {
	xdgDataDirs := os.Getenv("XDG_DATA_DIRS")
	if xdgDataDirs == "" {
		xdgDataDirs = "/usr/local/share/:/usr/share/"
	}
	for _, d := range strings.Split(xdgDataDirs, ":") {
		p := filepath.Join(d, "nwg-drawer")
		q := filepath.Join(p, "desktop-directories")
		if pathExists(q) {
			log.Infof("Data dir: %v", p)
			return p
		}
	}
	log.Warnf("Data dir not found")
	return ""
}

func getAppDirs() []string {
	var dirs []string

	home := os.Getenv("HOME")
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	xdgDataDirs := os.Getenv("XDG_DATA_DIRS")
	if xdgDataDirs == "" {
		xdgDataDirs = "/usr/local/share/:/usr/share/"
	}
	if xdgDataHome != "" {
		dirs = append(dirs, filepath.Join(xdgDataHome, "applications"))
	} else if home != "" {
		dirs = append(dirs, filepath.Join(home, ".local/share/applications"))
	}
	for _, d := range strings.Split(xdgDataDirs, ":") {
		dirs = append(dirs, filepath.Join(d, "applications"))
	}
	flatpakDirs := []string{filepath.Join(home, ".local/share/flatpak/exports/share/applications"),
		"/var/lib/flatpak/exports/share/applications"}

	for _, d := range flatpakDirs {
		if pathExists(d) && !isIn(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	var confirmedDirs []string
	for _, d := range dirs {
		if pathExists(d) {
			confirmedDirs = append(confirmedDirs, d)
		}
	}
	return confirmedDirs
}

func loadPreferredApps(path string) (map[string]interface{}, error) {
	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {
			log.Errorf("Error closing file: %v", jsonFile)
		}
	}(jsonFile)

	byteValue, _ := io.ReadAll(jsonFile)

	var result map[string]interface{}
	err = json.Unmarshal(byteValue, &result)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("json invalid or empty")
	}

	return result, nil
}

func listFiles(dir string) ([]fs.DirEntry, error) {
	files, err := os.ReadDir(dir)
	if err == nil {
		return files, nil
	}
	return nil, err
}

func listDesktopFiles() []string {
	var paths []string
	for _, dir := range appDirs {
		dirs, err := listFiles(dir)
		if err == nil {
			for _, file := range dirs {
				parts := strings.Split(file.Name(), ".")
				if parts[len(parts)-1] == "desktop" {
					paths = append(paths, filepath.Join(dir, file.Name()))
				}
			}
		}
	}
	return paths
}

func setUpCategories() {
	var other category

	dDir := dataDir()
	for _, cName := range categoryNames {
		fileName := fmt.Sprintf("%s.directory", cName)
		fp := filepath.Join(dDir, "desktop-directories", fileName)
		lines, err := loadTextFile(fp)
		if err == nil {
			var cat category
			cat.Name = cName

			name := ""
			nameLoc := ""
			icon := ""

			for _, l := range lines {
				if strings.HasPrefix(l, "Name=") {
					name = strings.Split(l, "=")[1]
					continue
				}
				if strings.HasPrefix(l, fmt.Sprintf("Name[%s]=", strings.Split(*lang, "_")[0])) {
					nameLoc = strings.Split(l, "=")[1]
					continue
				}
				if strings.HasPrefix(l, "Icon=") {
					icon = strings.Split(l, "=")[1]
					continue
				}
			}

			if nameLoc == "" {
				for _, l := range lines {
					if strings.HasPrefix(l, fmt.Sprintf("Name[%s]=", *lang)) {
						nameLoc = strings.Split(l, "=")[1]
						break
					}
				}
			}
			if nameLoc != "" {
				cat.DisplayName = nameLoc
			} else {
				cat.DisplayName = name
			}
			cat.Icon = icon

			// We want "other" to be the last one. Let's append it when already sorted
			if fileName != "other.directory" {
				categories = append(categories, cat)
			} else {
				other = cat
			}
		} else {
			log.Errorf("Couldn't open %s", fp)
		}
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].DisplayName < categories[j].DisplayName
	})
	categories = append(categories, other)
}

func parseDesktopFiles(desktopFiles []string) string {
	desktopEntries = nil
	id2entry = make(map[string]desktopEntry)
	skipped := 0
	hidden := 0
	for _, file := range desktopFiles {
		id := filepath.Base(file)
		if _, ok := id2entry[id]; ok {
			skipped++
			continue
		}

		entry, err := parseDesktopEntryFile(id, file)
		if err != nil {
			continue
		}

		if entry.NoDisplay {
			hidden++
			// We still need hidden entries, so `continue` is disallowed here
			// Fixes bug introduced in #19
		}

		id2entry[entry.DesktopID] = entry
		desktopEntries = append(desktopEntries, entry)
		assignToLists(entry.DesktopID, entry.Category)
	}
	sort.Slice(desktopEntries, func(i, j int) bool {
		return strings.ToLower(desktopEntries[i].NameLoc) < strings.ToLower(desktopEntries[j].NameLoc)
	})
	summary := fmt.Sprintf("%v entries (+%v hidden)", len(desktopEntries)-hidden, hidden)
	log.Infof("Skipped %v duplicates; %v .desktop entries hidden by \"NoDisplay=true\"", skipped, hidden)
	return summary
}

// freedesktop Main Categories list consists of 13 entries. Let's contract it to 8+1 ("Other").
func assignToLists(desktopID, categories string) {
	cats := strings.Split(categories, ";")
	assigned := false
	for _, cat := range cats {
		if cat == "Utility" && !isIn(listUtility, desktopID) {
			listUtility = append(listUtility, desktopID)
			assigned = true
			continue
		}
		if cat == "Development" && !isIn(listDevelopment, desktopID) {
			listDevelopment = append(listDevelopment, desktopID)
			assigned = true
			continue
		}
		if cat == "Game" && !isIn(listGame, desktopID) {
			listGame = append(listGame, desktopID)
			assigned = true
			continue
		}
		if cat == "Graphics" && !isIn(listGraphics, desktopID) {
			listGraphics = append(listGraphics, desktopID)
			assigned = true
			continue
		}
		if cat == "Network" && !isIn(listInternetAndNetwork, desktopID) {
			listInternetAndNetwork = append(listInternetAndNetwork, desktopID)
			assigned = true
			continue
		}
		if isIn([]string{"Office", "Science", "Education"}, cat) && !isIn(listOffice, desktopID) {
			listOffice = append(listOffice, desktopID)
			assigned = true
			continue
		}
		if isIn([]string{"AudioVideo", "Audio", "Video"}, cat) && !isIn(listAudioVideo, desktopID) {
			listAudioVideo = append(listAudioVideo, desktopID)
			assigned = true
			continue
		}
		if isIn([]string{"Settings", "System", "DesktopSettings", "PackageManager"}, cat) && !isIn(listSystemTools, desktopID) {
			listSystemTools = append(listSystemTools, desktopID)
			assigned = true
			continue
		}
	}
	if categories != "" && !assigned && !isIn(listOther, desktopID) {
		listOther = append(listOther, desktopID)
	}
}

func isIn(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func pathExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func loadTextFile(path string) ([]string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(bytes), "\n")
	var output []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			output = append(output, line)
		}

	}
	return output, nil
}

func pinItem(itemID string) {
	for _, item := range pinned {
		if item == itemID {
			log.Warnf("%s already pinned", itemID)
			return
		}
	}
	pinned = append(pinned, itemID)
	savePinned()
	log.Infof("%s pinned", itemID)
}

func unpinItem(itemID string) {
	if isIn(pinned, itemID) {
		pinned = remove(pinned, itemID)
		savePinned()
		log.Infof("%s unpinned", itemID)
	}
}

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func savePinned() {
	f, err := os.OpenFile(pinnedFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatal(err)
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Errorf("Error closing file: %v", f)
		}
	}(f)

	for _, line := range pinned {
		//skip invalid lines
		if line != "" && id2entry[line].DesktopID != "" {
			_, err := f.WriteString(line + "\n")

			if err != nil {
				log.Error("Error saving pinned", err)
			}
		}
	}
}

func launch(command string, terminal bool, terminate bool) {
	// trim % and everything afterwards
	if strings.Contains(command, "%") {
		cutAt := strings.Index(command, "%")
		if cutAt != -1 {
			command = command[:cutAt-1]
		}
	}

	if *wm != "uwsm" {
		themeToPrepend := ""
		//add "GTK_THEME=<default_gtk_theme>" environment variable
		if *forceTheme {
			settings := gtk.SettingsGetDefault()
			th := settings.ObjectProperty("gtk-theme-name")
			themeToPrepend = th.(string)
		}

		if themeToPrepend != "" {
			command = fmt.Sprintf("GTK_THEME=%q %s", themeToPrepend, command)
		}
	} else {
		if *forceTheme {
			log.Warn("We can't force GTK_THEME= while running a command through uwsm")
		}
	}

	var elements = []string{"/usr/bin/env", "-S", command}

	cmd := exec.Command(elements[0], elements[1:]...)

	if terminal {
		var prefixCommand = *term
		var args []string
		if prefixCommand != "foot" {
			args = []string{"-e", command}
		} else {
			args = elements
		}
		cmd = exec.Command(prefixCommand, args...)
	} else if *wm == "sway" {
		if _, ok := os.LookupEnv("SWAYSOCK"); ok {
			cmd = exec.Command("swaymsg", "exec", strings.Join(elements, " "))
		} else {
			log.Warn("Unable to find SWAYSOCK, running command directly")
		}
	} else if *wm == "hyprland" || *wm == "Hyprland" {
		if _, ok := os.LookupEnv("HYPRLAND_INSTANCE_SIGNATURE"); ok {
			cmd = exec.Command("hyprctl", "dispatch", "exec", strings.Join(elements, " "))
		} else {
			log.Warn("Unable to find HYPRLAND_INSTANCE_SIGNATURE, running command directly")
		}
	} else if *wm == "river" {
		// a check if we're actually on river would be of use here, but we have none
		cmd = exec.Command("riverctl", "spawn", strings.Join(elements, " "))
	} else if *wm == "niri" {
		if os.Getenv("XDG_CURRENT_DESKTOP") == "niri" {
			cmd = exec.Command("niri", append([]string{"msg", "action", "spawn", "--"}, elements...)...)
		} else {
			log.Warn("$XDG_CURRENT_DESKTOP != 'niri', running command directly")
		}
	} else if *wm == "uwsm" {
		if _, err := exec.LookPath("uwsm"); err == nil {
			cParts, _ := shlex.Split(command)
			cmd = exec.Command("uwsm", append([]string{"app", "--"}, cParts...)...)
		} else {
			log.Warn("Unable to find uwsm, running command directly")
		}
	}

	msg := fmt.Sprintf("Executing command: %q; args: %q\n", cmd.Args[0], cmd.Args[1:])
	log.Info(msg)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if cmd.Start() != nil {
		log.Warn("Unable to launch terminal emulator!")
	} else {
		// Collect the exit code of the child process to prevent zombies
		// if the drawer runs in resident mode
		go func() {
			_ = cmd.Wait()
		}()
	}

	if terminate {
		if *resident {
			restoreStateAndHide()
		} else {
			gtk.MainQuit()
		}
	}
}

func open(filePath string, xdgOpen bool) {
	var cmd *exec.Cmd
	if xdgOpen {
		cmd = exec.Command("xdg-open", filePath)
		// Look for possible custom file association
		for key, element := range preferredApps {
			r, err := regexp.Compile(key)
			if err == nil && r.MatchString(filePath) {
				cmd = exec.Command(fmt.Sprintf("%v", element), filePath)
				break
			}
		}
	} else {
		cmd = exec.Command(*fileManager, filePath)
	}
	log.Infof("Executing: %s", cmd)

	if cmd.Start() != nil {
		log.Warn("Unable to execute command!")
	} else {
		// Collect the exit code of the child process to prevent zombies
		// if the drawer runs in resident mode
		go func() {
			_ = cmd.Wait()
		}()
	}

	if *resident {
		restoreStateAndHide()
	} else {
		gtk.MainQuit()
	}
}

// Returns map output name -> gdk.Monitor
func mapOutputs() (map[string]*gdk.Monitor, error) {
	result := make(map[string]*gdk.Monitor)

	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		err := listHyprlandMonitors()
		if err == nil {

			display := gdk.DisplayGetDefault()

			num := display.NMonitors()
			for i := 0; i < num; i++ {
				mon := display.Monitor(i)
				output := hyprlandMonitors[i]
				result[output.Name] = mon
			}
		} else {
			return nil, err
		}

	} else if os.Getenv("SWAYSOCK") != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		client, err := sway.New(ctx)
		if err != nil {
			return nil, err
		}

		outputs, err := client.GetOutputs(ctx)
		if err != nil {
			return nil, err
		}

		display := gdk.DisplayGetDefault()
		if err != nil {
			return nil, err
		}

		num := display.NMonitors()
		for i := 0; i < num; i++ {
			mon := display.Monitor(i)
			output := outputs[i]
			result[output.Name] = mon
		}
	} else {
		return nil, errors.New("output assignment only supported on sway and Hyprland")
	}

	return result, nil
}

// KAdot / https://stackoverflow.com/a/38537764/4040598 - thanks!
func substring(s string, start int, end int) string {
	startStrIdx := 0
	i := 0
	for j := range s {
		if i == start {
			startStrIdx = j
		}
		if i == end {
			return s[startStrIdx:j]
		}
		i++
	}
	return s[startStrIdx:]
}

func hyprctl(cmd string) ([]byte, error) {
	his := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	hyprDir := ""
	if xdgRuntimeDir != "" {
		hyprDir = fmt.Sprintf("%s/hypr", xdgRuntimeDir)
	} else {
		hyprDir = "/tmp/hypr"
	}

	socketFile := fmt.Sprintf("%s/%s/.socket.sock", hyprDir, his)
	conn, err := net.Dial("unix", socketFile)
	if err != nil {
		return nil, err
	}

	message := []byte(cmd)
	_, err = conn.Write(message)
	if err != nil {
		return nil, err
	}

	reply := make([]byte, 102400)
	n, err := conn.Read(reply)
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	return reply[:n], nil
}

func listHyprlandMonitors() error {
	reply, err := hyprctl("j/monitors")
	if err != nil {
		return err
	} else {
		err = json.Unmarshal([]byte(reply), &hyprlandMonitors)
	}
	return err
}
