package main

import (
	"strings"
	"testing"
)

var result desktopEntry

func BenchmarkDesktopEntryParser(b *testing.B) {
	var entry desktopEntry
	for n := 0; n < b.N; n++ {
		entry, _ = parseDesktopEntryFile("id", "./desktop-directories/game.directory")
	}
	result = entry
}

func TestWhitespaceHandling(t *testing.T) {
	const whitespace = `[Desktop Entry]
	Categories = Debugger; Development; Git; IDE; Programming; TextEditor; 
	Comment = Editor for building and debugging modern web and cloud applications
	Exec = bash -c "code-insiders ~/Workspaces/Linux/Flutter.code-workspace"
	GenericName = Text Editor
	Icon = vscode-flutter
	Keywords = editor; IDE; plaintext; text; write; 
	MimeType = application/x-shellscript; inode/directory; text/english; text/plain; text/x-c; text/x-c++; text/x-c++hdr; text/x-c++src; text/x-chdr; text/x-csrc; text/x-java; text/x-makefile; text/x-moc; text/x-pascal; text/x-tcl; text/x-tex; 
	Name = VSCode Insiders with Flutter
	Name[pt] = VSCode Insiders com Flutter
	StartupNotify = true
	StartupWMClass = code - insiders
	Terminal = false
	NoDisplay = false
	Type = Application
	Version = 1.0`

	*lang = "pt"
	entry, err := parseDesktopEntry("id", strings.NewReader(whitespace), "test.desktop")
	if err != nil {
		t.Fatal(err)
	}

	if entry.Name != "VSCode Insiders with Flutter" {
		t.Error("failed to parse desktop entry name")
	}

	if entry.NameLoc != "VSCode Insiders com Flutter" {
		t.Error("failed to parse localized name")
	}

	if entry.NoDisplay {
		t.Error("failed to parse desktop entry no display")
	}
}
