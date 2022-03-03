package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/joshuarubin/go-sway"
)

func wayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland"
}

func createPixbuf(icon string, size int) (*gdk.Pixbuf, error) {
	iconTheme, err := gtk.IconThemeGetDefault()
	if err != nil {
		log.Fatal("Couldn't get default theme: ", err)
	}

	if strings.Contains(icon, "/") {
		pixbuf, err := gdk.PixbufNewFromFileAtSize(icon, size, size)
		if err != nil {
			log.Errorf("%s", err)
			return nil, err
		}
		return pixbuf, nil

	} else if strings.HasSuffix(icon, ".svg") || strings.HasSuffix(icon, ".png") || strings.HasSuffix(icon, ".xpm") {
		// for entries like "Icon=netflix-desktop.svg"
		icon = strings.Split(icon, ".")[0]
	}

	pixbuf, err := iconTheme.LoadIcon(icon, size, gtk.ICON_LOOKUP_FORCE_SIZE)

	if err != nil {
		if strings.HasPrefix(icon, "/") {
			pixbuf, err := gdk.PixbufNewFromFileAtSize(icon, size, size)
			if err != nil {
				return nil, err
			}
			return pixbuf, nil
		}

		pixbuf, err := iconTheme.LoadIcon(icon, size, gtk.ICON_LOOKUP_FORCE_SIZE)
		if err != nil {
			return nil, err
		}
		return pixbuf, nil
	}
	return pixbuf, nil
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

	userDirsFile := filepath.Join(home, ".config/user-dirs.dirs")
	if pathExists(userDirsFile) {
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
			}
		}
	} else {
		log.Warnf("%s file not found, using defaults", userDirsFile)
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
	if os.Getenv("XDG_CACHE_HOME") != "" {
		return os.Getenv("XDG_CONFIG_HOME")
	}
	if os.Getenv("HOME") != "" && pathExists(filepath.Join(os.Getenv("HOME"), ".cache")) {
		p := filepath.Join(os.Getenv("HOME"), ".cache")
		return p
	}
	return ""
}

func tempDir() string {
	if os.Getenv("TMPDIR") != "" {
		return os.Getenv("TMPDIR")
	} else if os.Getenv("TEMP") != "" {
		return os.Getenv("TEMP")
	} else if os.Getenv("TMP") != "" {
		return os.Getenv("TMP")
	}
	return "/tmp"
}

func readTextFile(path string) (string, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func oldConfigDir() (string, error) {
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		dir := path.Join(os.Getenv("XDG_CONFIG_HOME"), "nwg-panel")
		return dir, nil
	} else if os.Getenv("HOME") != "" {
		dir := path.Join(os.Getenv("HOME"), ".config/nwg-panel")
		return dir, nil
	}

	return "", errors.New("old config dir not found")
}

func configDir() string {
	var dir string
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		dir = path.Join(os.Getenv("XDG_CONFIG_HOME"), "nwg-drawer")
	} else if os.Getenv("HOME") != "" {
		dir = path.Join(os.Getenv("HOME"), ".config/nwg-drawer")
	}

	log.Infof("Config dir: %s", dir)
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
	log.Infof("Copying file: %s", dst)

	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}

func getDataHome() string {
	if os.Getenv("XDG_DATA_HOME") != "" {
		return os.Getenv("XDG_DATA_HOME")
	}
	return "/usr/share/"
}

func getAppDirs() []string {
	var dirs []string
	xdgDataDirs := ""

	userApps := *userDefinedAppsDirectory
	if userApps != "" {
		dirs = append(dirs, userApps)
	} else {
		home := os.Getenv("HOME")
		xdgDataHome := os.Getenv("XDG_DATA_HOME")
		if os.Getenv("XDG_DATA_DIRS") != "" {
			xdgDataDirs = os.Getenv("XDG_DATA_DIRS")
		} else {
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
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var result map[string]interface{}
	json.Unmarshal([]byte(byteValue), &result)

	if len(result) == 0 {
		return nil, errors.New("json invalid or empty")
	}

	return result, nil
}

func listFiles(dir string) ([]fs.FileInfo, error) {
	files, err := ioutil.ReadDir(dir)
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
	path := filepath.Join(getDataHome(), "nwg-drawer/desktop-directories")
	var other category

	jsonFile, err := os.Open("/home/apoema/.config/nwg-drawer/categories.json")
	if err == nil {
		byteValue, _ := ioutil.ReadAll(jsonFile)

		var usrCats []category
		json.Unmarshal([]byte(byteValue), &usrCats)
		categories = append(categories, usrCats[:]...)
	}
	defer jsonFile.Close()

	for _, cName := range categoryNames {
		fileName := fmt.Sprintf("%s.directory", cName)
		lines, err := loadTextFile(filepath.Join(path, fileName))
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
		return desktopEntries[i].NameLoc < desktopEntries[j].NameLoc
	})
	summary := fmt.Sprintf("%v entries (+%v hidden)", len(desktopEntries)-hidden, hidden)
	log.Infof("Skipped %v duplicates; %v .desktop entries hidden by \"NoDisplay=true\"", skipped, hidden)
	return summary
}

// freedesktop Main Categories list consists of 13 entries. Let's contract it to 8+1 ("Other").
func assignToLists(desktopID, appCategories string) {
	cats := strings.Split(appCategories, ";")
	for i := 0; i < len(categories); i++ {
		for _, appCat := range cats {
			if appCat == categories[i].Name {
				categories[i].Apps = append(categories[i].Apps, desktopID)
				continue
			}
		}
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
	bytes, err := ioutil.ReadFile(path)
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

	defer f.Close()

	for _, line := range pinned {
		if line != "" {
			_, err := f.WriteString(line + "\n")

			if err != nil {
				log.Errorf("Error saving pinned", err)
			}
		}
	}
}

func launch(command string, terminal bool) {
	// trim % and everything afterwards
	if strings.Contains(command, "%") {
		cutAt := strings.Index(command, "%")
		if cutAt != -1 {
			command = command[:cutAt-1]
		}
	}

	elements := strings.Split(command, " ")

	// find prepended env variables, if any
	envVarsNum := strings.Count(command, "=")
	var envVars []string

	cmdIdx := -1

	if envVarsNum > 0 {
		for idx, item := range elements {
			if strings.Contains(item, "=") {
				envVars = append(envVars, item)
			} else if !strings.HasPrefix(item, "-") && cmdIdx == -1 {
				cmdIdx = idx
			}
		}
	}
	if cmdIdx == -1 {
		cmdIdx = 0
	}

	cmd := exec.Command(elements[cmdIdx], elements[1+cmdIdx:]...)

	if terminal {
		var args []string
		if *term != "foot" {
			args = []string{"-e", elements[cmdIdx]}
		} else {
			args = []string{elements[cmdIdx]}
		}

		cmd = exec.Command(*term, args...)
	}

	// set env variables
	if len(envVars) > 0 {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, envVars...)
	}

	msg := fmt.Sprintf("env vars: %s; command: '%s'; args: %s\n", envVars, elements[cmdIdx], elements[1+cmdIdx:])
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

	if *resident {
		restoreStateAndHide()
	} else {
		gtk.MainQuit()
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

	display, err := gdk.DisplayGetDefault()
	if err != nil {
		return nil, err
	}

	num := display.GetNMonitors()
	for i := 0; i < num; i++ {
		monitor, _ := display.GetMonitor(i)
		geometry := monitor.GetGeometry()
		// assign output to monitor on the basis of the same x, y coordinates
		for _, output := range outputs {
			if int(output.Rect.X) == geometry.GetX() && int(output.Rect.Y) == geometry.GetY() {
				result[output.Name] = monitor
			}
		}
	}
	return result, nil
}
