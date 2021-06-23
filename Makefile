get:
	go get github.com/gotk3/gotk3
	go get github.com/gotk3/gotk3/gdk
	go get github.com/gotk3/gotk3/glib
	go get github.com/dlasky/gotk3-layershell/layershell
	go get github.com/joshuarubin/go-sway
	go get github.com/allan-simon/go-singleinstance

build:
	go build -o bin/nwg-drawer *.go

install:
	mkdir -p /usr/share/nwg-drawer
	cp -r desktop-directories /usr/share/nwg-drawer
	cp drawer.css /usr/share/nwg-drawer
	cp bin/nwg-drawer /usr/bin

uninstall:
	rm -r /usr/share/nwg-drawer
	rm /usr/bin/nwg-drawer

run:
	go run *.go
