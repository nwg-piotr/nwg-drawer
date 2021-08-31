package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func parseDesktopEntryFileDeprecated(id string, path string) (entry desktopEntry, err error) {
	lines, err := loadTextFile(path)
	if err != nil {
		return entry, err
	}

	return parseDesktopEntryDeprecated(id, lines)
}

func parseDesktopEntryDeprecated(id string, lines []string) (entry desktopEntry, err error) {
	desktopID := id
	name := ""
	nameLoc := ""
	comment := ""
	commentLoc := ""
	icon := ""
	exec := ""
	categories := ""
	terminal := false
	noDisplay := false

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

	entry.DesktopID = desktopID
	entry.Name = name
	entry.NameLoc = nameLoc
	entry.Comment = comment
	entry.CommentLoc = commentLoc
	entry.Icon = icon
	entry.Exec = exec
	entry.Terminal = terminal
	entry.NoDisplay = noDisplay
	entry.Category = categories
	return entry, nil
}

func parseDesktopEntryFile(id string, path string) (e desktopEntry, err error) {
	o, err := os.Open(path)
	if err != nil {
		return e, err
	}
	defer o.Close()

	return parseDesktopEntry(id, o)
}

func parseDesktopEntry(id string, in io.Reader) (entry desktopEntry, err error) {
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
			entry.NoDisplay, _ = strconv.ParseBool(value)
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
