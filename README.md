<img src="https://github.com/nwg-piotr/nwg-drawer/assets/20579136/1a7578e8-5332-4e4c-bdce-9b9bf875c0e7" width="90" style="margin-right:10px" align=left alt="logo">
<H1>nwg-drawer</H1><br>

This application is a part of the [nwg-shell](https://nwg-piotr.github.io/nwg-shell) project.

**Nwg-drawer** is an application launcher. It's being developed with [sway](https://github.com/swaywm/sway) and 
[Hyprland](https://github.com/hyprwm/Hyprland) in mind, but should also work with other wlroots-based Wayland 
compositors.

The `nwg-drawer` command displays the application grid. The search entry allows to look for installed applications,
and for files in XDG user directories. The grid view may also be filtered by categories.

You may pin applications by right-clicking them. Pinned items will appear above the application grid. Right-click
a pinned item to unpin it. The pinned items cache is shared with [nwg-menu](https://github.com/nwg-piotr/nwg-menu).

Below the grid there is the power bar - a row of buttons to lock the screen, exit the compositor, reboot, suspend and power
the machine off. For each button to appear, you need to provide a corresponding command. See "Command line arguments"
below.

<img src="https://github.com/nwg-piotr/nwg-drawer/assets/20579136/8f4eacb4-5395-4350-889b-a9037aa34f08" width=640 alt="screenshot"><br>

To close the window w/o running a program, you may use the `Esc` key, or right-click the window next to the grid.

## Installation

[![Packaging status](https://repology.org/badge/vertical-allrepos/nwg-drawer.svg)](https://repology.org/project/nwg-drawer/versions)

### Dependencies

- go >=1.20 (just to build)
- gtk3
- gtk-layer-shell
- xdg-utils

Optional (recommended):

- thunar
- foot

You may use another file manager and terminal emulator (see command line arguments), but mentioned above have been
confirmed to work well with the program. Also see **Files** below.

### Steps

1. Clone the repository, cd into it.
2. Install necessary golang libraries with `make get`.
3. `make build`
4. `sudo make install`

## Command line arguments

```text
$ nwg-drawer -h
Usage of /tmp/go-build3511850078/b001/exe/nwg-drawer:
  -c uint
    	number of Columns (default 6)
  -d	Turn on Debug messages
  -fm string
    	File Manager (default "thunar")
  -fscol uint
    	File Search result COLumns (default 2)
  -fslen int
    	File Search name LENgth Limit (default 80)
  -ft
    	Force Theme for libadwaita apps, by adding 'GTK_THEME=<default-gtk-theme>' env var
  -g string
    	GTK theme name
  -i string
    	GTK icon theme name
  -is int
    	Icon Size (default 64)
  -k	set GTK layer shell Keyboard interactivity to 'on-demand' mode
  -lang string
    	force lang, e.g. "en", "pl"
  -mb int
    	Margin Bottom
  -ml int
    	Margin Left
  -mr int
    	Margin Right
  -mt int
    	Margin Top
  -nocats
    	Disable filtering by category
  -nofs
    	Disable file search
  -o string
    	name of the Output to display the drawer on (sway & Hyprland only)
  -ovl
    	use OVerLay layer
  -pbexit string
    	command for the Exit power bar icon
  -pblock string
    	command for the Lock power bar icon
  -pbpoweroff string
    	command for the Poweroff power bar icon
  -pbreboot string
    	command for the Reboot power bar icon
  -pbsize int
    	power bar icon size (default 64)
  -pbsleep string
    	command for the sleep power bar icon
  -r	Leave the program resident in memory
  -s string
    	Styling: css file name (default "drawer.css")
  -spacing uint
    	icon spacing (default 20)
  -term string
    	Terminal emulator (default "foot")
  -v	display Version information
  -wm string
    	use swaymsg exec (with 'sway' argument) or hyprctl dispatch exec (with 'hyprland') to launch programs
  ```

  *NOTE: the `$TERM` environment variable overrides the `-term` argument.*

## Running

You may use the drawer in two ways:

1. Simply run the `nwg-drawer` command, by adding a key binding to your sway config file, e.g.:

```text
bindsym Mod1+F1 exec nwg-drawer
```

2. Run a resident instance on startup, and use the `nwg-drawer` command to show the window, e.g.:

```text
exec_always nwg-drawer -r
bindsym Mod1+F1 exec nwg-drawer
```

The second line does nothing but `pkill -USR1 nwg-drawer`, so you may just use this command instead. Actually
this should be a little bit faster.

Running a resident instance should speed up use of the drawer significantly. Pay attention to the fact, that you
need to `pkill -f nwg-drawer` and reload the compositor to apply any new arguments!

## Logging

In case you encounter an issue, you may need debug messages. If you use the resident instance, you'll see nothing
in the terminal. Please edit your sway config file:

```text
exec nwg-drawer -r -d 2> ~/drawer.log
```

exit sway, launch it again and include the `drawer.log` content in the GitHub issue. Do not use `exec_always` here: it'll destroy the log file content on sway reload.

## Styling

Edit `~/.config/nwg-drawer/drawer.css` to your taste.

## File search

When the search phrase is at least 3 characters long, your XDG user directories are being searched.

Use the **left mouse button** to open a file with the `xdg-open` command. As configuring file associations for it is
PITA, you may override them, by creating the `~/.config/nwg-panel/preferred-apps.json` file with your own definitions.

### Sample `preferred-apps.json` file content

```json
{
  "\\.pdf$": "atril",
  "\\.svg$": "inkscape",
  "\\.(jpg|png|tiff|gif)$": "swayimg",
  "\\.(mp3|ogg|flac|wav|wma)$": "audacious",
  "\\.(avi|mp4|mkv|mov|wav)$": "mpv",
  "\\.(doc|docx|xls|xlsx)$": "libreoffice"
}
```

Use the **right mouse button** to open the file with your file manager (see `-fm` argument). The result depends
on the file manager you use.

- thunar will open the file location
- pcmanfm will open the file with its associated program
- caja won't open anything, except for directories

I've noy yet tried other file managers.

### File search exclusions

You may want to exclude some paths inside your XDG user directories from searching. If so, define exclusions in the
`~/.config/nwg-panel/excluded-dirs` file, e.g. like this:

```text
# exclude all paths containing 'node_modules'
node_modules
```

## Credits

This program uses some great libraries:

- [gotk3](https://github.com/gotk3/gotk3) Copyright (c) 2013-2014 Conformal Systems LLC,
Copyright (c) 2015-2018 gotk3 contributors
- [gotk3-layershell](https://github.com/dlasky/gotk3-layershell) by [@dlasky](https://github.com/dlasky/gotk3-layershell/commits?author=dlasky) - many thanks for writing this software, and for patience with my requests!
- [go-sway](https://github.com/joshuarubin/go-sway) Copyright (c) 2019 Joshua Rubin
- [go-singleinstance](github.com/allan-simon/go-singleinstance) Copyright (c) 2015 Allan Simon
- [logrus](https://github.com/sirupsen/logrus) Copyright (c) 2014 Simon Eskildsen
- [fsnotify](https://github.com/fsnotify/fsnotify) Copyright (c) 2012-2019 fsnotify Authors
