package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func parseDesktopEntryFile(id string, path string) (e desktopEntry, err error) {
	o, err := os.Open(path)
	if err != nil {
		return e, err
	}
	defer o.Close()

	return parseDesktopEntry(id, o, path)
}

func parseDesktopEntry(id string, in io.Reader, path string) (entry desktopEntry, err error) {
	entry.DesktopFile = path
	cleanexec := strings.NewReplacer("\"", "", "'", "")
	entry.DesktopID = id
	localizedName := fmt.Sprintf("Name[%s]", strings.Split(*lang, "_")[0])
	localizedComment := fmt.Sprintf("Comment[%s]", strings.Split(*lang, "_")[0])
	scanner := bufio.NewScanner(in)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		l := scanner.Text()
		if strings.HasPrefix(l, "[") && l != "[Desktop Entry]" {
			break
		}

		name, value := parseKeypair(l)
		if value == "" {
			continue
		}

		switch name {
		case "Name":
			entry.Name = value
		case localizedName:
			entry.NameLoc = value
		case "Comment":
			entry.Comment = value
		case localizedComment:
			entry.CommentLoc = value
		case "Icon":
			entry.Icon = value
		case "Categories":
			entry.Category = value
		case "Terminal":
			entry.Terminal, _ = strconv.ParseBool(value)
		case "NoDisplay":
			if !entry.NoDisplay {
				entry.NoDisplay, _ = strconv.ParseBool(value)
			}
		case "Hidden":
			if !entry.NoDisplay {
				entry.NoDisplay, _ = strconv.ParseBool(value)
			}
		case "OnlyShowIn":
			if !entry.NoDisplay {
				entry.NoDisplay = true
				currentDesktop := os.Getenv("XDG_CURRENT_DESKTOP")
				if currentDesktop != "" {
					for _, ele := range strings.Split(value, ";") {
						if ele == currentDesktop && ele != "" {
							entry.NoDisplay = false
						}
					}
				}
			}
		case "NotShowIn":
			currentDesktop := os.Getenv("XDG_CURRENT_DESKTOP")
			if !entry.NoDisplay && currentDesktop != "" {
				for _, ele := range strings.Split(value, ";") {
					if ele == currentDesktop && ele != "" {
						entry.NoDisplay = true
					}
				}
			}
		case "Exec":
			entry.Exec = cleanexec.Replace(value)
		}
	}

	// if name[ln] not found, let's try to find name[ln_LN]
	if entry.NameLoc == "" {
		entry.NameLoc = entry.Name
	}
	if entry.CommentLoc == "" {
		entry.CommentLoc = entry.Comment
	}
	return entry, err
}

func parseKeypair(s string) (string, string) {
	if idx := strings.IndexRune(s, '='); idx > 0 {
		return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+1:])
	}
	return s, ""
}
