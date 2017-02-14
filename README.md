# filebrowser
Filebrowser + thumbnail viewer

![screenshot](https://github.com/lutfinasution/filebrowser/blob/master/image/filebrowser01.png?raw=true "screenshot")

Expanded example based on lxn walk win32 libraries https://github.com/lxn/walk filebrowser example, adding many new
features. 

Features:
  - Image browser, displaying thumbnail of images in a grid.
  - Image http server, serving thumbnail of images in a folder. Use http://localhost:8080/users/photos while the app is running.
  - Very fast and efficient multi threaded thumbnail processing with goroutines and channels.
  - Resizeable thumbnail size.
  - Filesystem/directory monitoring for changes (new/delete/renamed/modified files)

Golang's features explored here:
  - Goroutine, channels and synchronizations.
  - Maps, slices
  - cgo interfacing with external c libs
  - Database interface with sqlite3
  - Image processing (drawings/transforms)

Additional excellent golang libraries used here:
  - https://github.com/pixiv/go-libjpeg/jpeg
  - https://github.com/mattn/go-sqlite3
  - https://github.com/fsnotify/fsnotify
  - https://github.com/anthonynsimon/bild/transform



