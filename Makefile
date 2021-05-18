get:
	go get github.com/gotk3/gotk3
	go get github.com/gotk3/gotk3/gdk
	go get github.com/gotk3/gotk3/glib
	go get github.com/dlasky/gotk3-layershell/layershell
	go get github.com/joshuarubin/go-sway
	go get github.com/allan-simon/go-singleinstance

build:
	go build -o bin/nwg-menu *.go

install:
	mkdir -p /usr/share/nwg-menu
	cp -r desktop-directories /usr/share/nwg-menu
	cp menu-start.css /usr/share/nwg-menu
	cp bin/nwg-menu /usr/bin

uninstall:
	rm /usr/bin/nwg-menu

run:
	go run *.go
