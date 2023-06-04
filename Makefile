PREFIX ?= /usr
DESTDIR ?=

get:
	go get github.com/gotk3/gotk3
	go get github.com/gotk3/gotk3/gdk
	go get github.com/gotk3/gotk3/glib
	go get github.com/dlasky/gotk3-layershell/layershell
	go get github.com/joshuarubin/go-sway
	go get github.com/allan-simon/go-singleinstance
	go get "github.com/sirupsen/logrus"
	go get github.com/fsnotify/fsnotify

build:
	go build -o bin/nwg-drawer .

install:
	-pkill -f nwg-drawer
	sleep 1
	mkdir -p $(DESTDIR)$(PREFIX)/bin
	mkdir -p $(DESTDIR)$(PREFIX)/share/nwg-dock
	cp -r desktop-directories $(DESTDIR)$(PREFIX)/share/nwg-drawer
	cp drawer.css $(DESTDIR)$(PREFIX)/share/nwg-drawer
	cp bin/nwg-drawer $(DESTDIR)$(PREFIX)/bin

uninstall:
	rm -r $(DESTDIR)$(PREFIX)/share/nwg-drawer
	rm $(DESTDIR)$(PREFIX)/bin/nwg-drawer

run:
	go run .
