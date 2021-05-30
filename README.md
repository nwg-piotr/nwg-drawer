# nwg-drawer

This application is a part of the [nwg-shell](https://github.com/nwg-piotr/nwg-shell) project.

Nwg-drawer is a golang replacement to the `nwggrid` command
(a part of [nwg-launchers](https://github.com/nwg-piotr/nwg-launchers)). It's being developed with
[sway](https://github.com/swaywm/sway) in mind, but should also work with other wlroots-based Wayland compositors.
X Window System is not supported.

The `nwg-drawer` command displays the application grid. The search entry allows to look for installed applications,
and for files in XDG user directories. The grid view may also be filtered by categories.

You may pin applications by right-clicking them. Pinned items will appear above the application grid. Right-click
a pinned item to unpin it. The pinned items cache is shared with `nwg-menu` and `nwggrid`.

![screenshot-01.png](https://scrot.cloud/images/2021/05/30/screenshot-01.png)

[more screenshots](https://scrot.cloud/album/nwg-drawer.Bogd)

## Installation

### Dependencies

- go 1.16 (just to build)
- gtk3
- gtk-layer-shell

Optional (recommended):

- thunar
- alacritty

You may use another file manager and terminal emulator (see command line arguments), but for now the program has
 only been tested with the two mentioned above.

### Steps

1. Clone the repository, cd into it.
2. Install necessary golang libraries with `make get`.
3. `make build`
4. `sudo make install`

Building the gotk3 library takes quite a lot of time. If your machine is x86_64, you may skip steps 3-4, and
install the provided binary by executing step 4.

## Command line arguments

```text
$ nwg-drawer -h
Usage of nwg-drawer:
  -c uint
    	number of Columns (default 6)
  -fm string
    	File Manager (default "thunar")
  -fscol uint
    	File Search result COLumns (default 2)
  -fslen int
    	File Search name length Limit (default 80)
  -is int
    	Icon Size (default 64)
  -lang string
    	force lang, e.g. "en", "pl"
  -o string
    	name of the Output to display the menu on (sway only)
  -ovl
    	use OVerLay layer
  -s string
    	Styling: css file name (default "drawer.css")
  -spacing uint
    	icon spacing (default 20)
  -term string
    	Terminal emulator (default "alacritty")
  -v	display Version information
  ```

## Styling

Edit `~/.config/nwg-panel/drawer.css` to your taste.

## Credits

This program uses some great libraries:

- [gotk3](https://github.com/gotk3/gotk3) Copyright (c) 2013-2014 Conformal Systems LLC,
Copyright (c) 2015-2018 gotk3 contributors
- [gotk3-layershell](https://github.com/dlasky/gotk3-layershell) by [@dlasky](https://github.com/dlasky/gotk3-layershell/commits?author=dlasky) - many thanks for writing this software, and for patience with my requests!
- [go-sway](https://github.com/joshuarubin/go-sway) Copyright (c) 2019 Joshua Rubin
- [go-singleinstance](github.com/allan-simon/go-singleinstance) Copyright (c) 2015 Allan Simon
