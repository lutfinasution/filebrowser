// fb_net
package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

//func indexHandler(w http.ResponseWriter, req *http.Request) {
func HandleDirRequest(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	name := vars["name"]
	log.Println("handling HandleDirRequest", vars)
	//var sRes string

	switch name {
	case "photos":
		fmt.Fprintf(w, "<!DOCTYPE html>")
		fmt.Fprintf(w, "<body>")
		for k, _ := range Mw.thumbView.ItemsMap {
			k = url.PathEscape(k)
			fmt.Fprintf(w, "<a href='/users/photos/%s'><img src='/users/photos/%s' alt=Image />", k, k)
		}
		fmt.Fprintf(w, "</body>")
		fmt.Fprintf(w, "</html>")
	case "albums":
		Mw.albumView.AlbumDBEnum("")

		fmt.Fprintf(w, "<!DOCTYPE html>")
		fmt.Fprintf(w, "<header>")
		fmt.Fprintf(w, "<h2>ALBUMS:</h2>")
		fmt.Fprintf(w, "</header>")
		fmt.Fprintf(w, "<body>")
		for k, _ := range Mw.albumView.ItemsMap {
			ks := url.PathEscape(k)
			fmt.Fprintf(w, "<a href='/users/albums/%s'><img src='/users/album-image/%s' alt=Image <h4>   %s</h4><br/>", ks, ks, k)
		}
		fmt.Fprintf(w, "</body>")
		fmt.Fprintf(w, "</html>")
	}
}
func HandleItemRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	item := vars["item"]
	log.Println("handling HandleItemRequest", vars)

	switch name {
	case "photos":
		if item != "" {
			if v, ok := Mw.thumbView.ItemsMap[item]; ok {
				w.Header().Set("Content-Type", "image/jpeg")
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(v.Imagedata)))
				w.Write(v.Imagedata)
			}
		}
	case "album-image":
		//useralbum imagedata
		if item != "" {
			if v, ok := Mw.albumView.ItemsMap[item]; ok {
				w.Header().Set("Content-Type", "image/jpeg")
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(v.Imagedata)))
				w.Write(v.Imagedata)
			}
		}
	case "albums":
		//useralbumitems listing
		if item != "" {
			if fi := Mw.albumView.AlbumDBEnumByNameItems(item); fi != nil {

				fmt.Fprintf(w, "<!DOCTYPE html>")
				fmt.Fprintf(w, "<header>")
				fmt.Fprintf(w, "<h2>ALBUM: %s</h2>", item)
				fmt.Fprintf(w, "<h4><a href='/users/albums'>Back to Albums</h4>")
				fmt.Fprintf(w, "</header>")
				fmt.Fprintf(w, "<body>")

				for _, v := range fi {
					ks := url.PathEscape(v.Name)
					fmt.Fprintf(w, "<a href='/users/albums-image/%s'><img src='/users/albums-thumb/%s' alt=Image />", ks, ks)
				}
				fmt.Fprintf(w, "</body>")
				fmt.Fprintf(w, "</html>")
			}
		}
	case "albums-thumb":
		//useralbumitems imagedata
		if item != "" {
			if v := Mw.albumView.AlbumDBGetItem(item); v != nil {
				w.Header().Set("Content-Type", "image/jpeg")
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(v.Imagedata)))
				w.Write(v.Imagedata)
			}
		}
	case "albums-image":
		//useralbumitems full image from filesystem
		if item != "" {
			if v := Mw.albumView.AlbumDBGetItem(item); v != nil {
				fn := filepath.Join(v.URL, v.Name)

				if f, err := os.Open(fn); err == nil {
					defer f.Close()

					info, _ := f.Stat()
					buff := make([]byte, info.Size())
					nLen, _ := f.Read(buff)

					w.Header().Set("Content-Type", "image/jpeg")
					w.Header().Set("Content-Length", fmt.Sprintf("%d", nLen))
					w.Write(buff)
				}
			}
		}
	}

}
func StartNet() bool {
	r := mux.NewRouter()

	r.HandleFunc("/users/{name}", HandleDirRequest).Methods("GET")
	r.HandleFunc("/users/{name}/{item}", HandleItemRequest).Methods("GET")

	err := http.ListenAndServe(":8080", r)

	return err == nil
}
