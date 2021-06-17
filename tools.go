package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/joshuarubin/go-sway"
)

/*
Window leave-notify-event event quits the program with glib Timeout 500 ms.
We might have left the window by accident, so let's clear the timeout if window re-entered.
Furthermore - hovering a widget triggers window leave-notify-event event, and the timeout
needs to be cleared as well.
*/
func cancelClose() {
	if src > 0 {
		glib.SourceRemove(src)
		src = 0
	}
}

func createPixbuf(icon string, size int) (*gdk.Pixbuf, error) {
	iconTheme, err := gtk.IconThemeGetDefault()
	if err != nil {
		log.Fatal("Couldn't get default theme: ", err)
	}

	if strings.Contains(icon, "/") {
		pixbuf, err := gdk.PixbufNewFromFileAtSize(icon, size, size)
		if err != nil {
			println(err)
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
		println(fmt.Sprintf("Using XDG user dirs from %s", userDirsFile))
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
		println(fmt.Sprintf("%s file not found, using defaults", userDirsFile))
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

func configDir() string {
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		dir := fmt.Sprintf("%s/nwg-panel", os.Getenv("XDG_CONFIG_HOME"))
		createDir(dir)
		return (fmt.Sprintf("%s/nwg-panel", os.Getenv("XDG_CONFIG_HOME")))
	}
	dir := fmt.Sprintf("%s/.config/nwg-panel", os.Getenv("HOME"))
	createDir(dir)
	return dir
}

func createDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, os.ModePerm)
		if err == nil {
			fmt.Println("Creating dir:", dir)
		}
	}
}

func copyFile(src, dst string) error {
	fmt.Println("Copying file:", dst)

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

func getAppDirs() []string {
	var dirs []string
	xdgDataDirs := ""

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
		if !isIn(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	return dirs
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
	path := "/usr/share/nwg-menu/desktop-directories"
	var other category

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
	id2entry = make(map[string]desktopEntry)
	var added []string
	skipped := 0
	hidden := 0
	for _, file := range desktopFiles {
		lines, err := loadTextFile(file)
		if err == nil {
			parts := strings.Split(file, "/")
			desktopID := parts[len(parts)-1]
			name := ""
			nameLoc := ""
			comment := ""
			commentLoc := ""
			icon := ""
			exec := ""
			terminal := false
			noDisplay := false

			categories := ""

			for _, l := range lines {
				if strings.HasPrefix(l, "[") && l != "[Desktop Entry]" {
					break
				}
				if strings.HasPrefix(l, "Name=") {
					name = strings.Split(l, "=")[1]
					continue
				}
				if strings.HasPrefix(l, fmt.Sprintf("Name[%s]=", strings.Split(*lang, "_")[0])) {
					nameLoc = strings.Split(l, "=")[1]
					continue
				}
				if strings.HasPrefix(l, "Comment=") {
					comment = strings.Split(l, "=")[1]
					continue
				}
				if strings.HasPrefix(l, fmt.Sprintf("Comment[%s]=", strings.Split(*lang, "_")[0])) {
					commentLoc = strings.Split(l, "=")[1]
					continue
				}
				if strings.HasPrefix(l, "Icon=") {
					icon = strings.Split(l, "=")[1]
					continue
				}
				if strings.HasPrefix(l, "Exec=") {
					exec = strings.Split(l, "Exec=")[1]
					disallowed := [2]string{"\"", "'"}
					for _, char := range disallowed {
						exec = strings.Replace(exec, char, "", -1)
					}
					continue
				}
				if strings.HasPrefix(l, "Categories=") {
					categories = strings.Split(l, "Categories=")[1]
					continue
				}
				if l == "Terminal=true" {
					terminal = true
					continue
				}
				if l == "NoDisplay=true" {
					noDisplay = true
					hidden++
					continue
				}
			}

			// if name[ln] not found, let's try to find name[ln_LN]
			if nameLoc == "" {
				nameLoc = name
			}
			if commentLoc == "" {
				commentLoc = comment
			}

			if !isIn(added, desktopID) {
				added = append(added, desktopID)

				var entry desktopEntry
				entry.DesktopID = desktopID
				entry.Name = name
				entry.NameLoc = nameLoc
				entry.Comment = comment
				entry.CommentLoc = commentLoc
				entry.Icon = icon
				entry.Exec = exec
				entry.Terminal = terminal
				entry.NoDisplay = noDisplay
				desktopEntries = append(desktopEntries, entry)

				id2entry[entry.DesktopID] = entry

				assignToLists(entry.DesktopID, categories)

			} else {
				skipped++
			}
		}
	}
	sort.Slice(desktopEntries, func(i, j int) bool {
		return desktopEntries[i].NameLoc < desktopEntries[j].NameLoc
	})
	summary := fmt.Sprintf("%v entries (+%v hidden)", len(desktopEntries)-hidden, hidden)
	println(fmt.Sprintf("Skipped %v duplicates; %v .desktop entries hidden by \"NoDisplay=true\"", skipped, hidden))
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
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(bytes), "\n")
	var output []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			output = append(output, line)
		}

	}
	return output, nil
}

func pinItem(itemID string) {
	for _, item := range pinned {
		if item == itemID {
			println(item, "already pinned")
			return
		}
	}
	pinned = append(pinned, itemID)
	savePinned()
	println(itemID, "pinned")
}

func unpinItem(itemID string) {
	if isIn(pinned, itemID) {
		pinned = remove(pinned, itemID)
		savePinned()
		println(itemID, "unpinned")
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
				println("Error saving pinned", err)
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

	cmdIdx := 0
	lastEnvVarIdx := 0

	if envVarsNum > 0 {
		for idx, item := range elements {
			if strings.Contains(item, "=") {
				lastEnvVarIdx = idx
				envVars = append(envVars, item)
			}
		}
		cmdIdx = lastEnvVarIdx + 1
	}

	cmd := exec.Command(elements[cmdIdx], elements[1+cmdIdx:]...)

	if terminal {
		args := []string{"-e", elements[cmdIdx]}
		cmd = exec.Command(*term, args...)
	}

	// set env variables
	if len(envVars) > 0 {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, envVars...)
	}

	msg := fmt.Sprintf("env vars: %s; command: '%s'; args: %s\n", envVars, elements[cmdIdx], elements[1+cmdIdx:])
	println(msg)

	go cmd.Run()

	glib.TimeoutAdd(uint(150), func() bool {
		gtk.MainQuit()
		return false
	})
}

func open(filePath string) {
	cmd := exec.Command(*fileManager, filePath)
	go cmd.Run()

	glib.TimeoutAdd(uint(150), func() bool {
		gtk.MainQuit()
		return false
	})
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
