// fb_net
package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

//func indexHandler(w http.ResponseWriter, req *http.Request) {
func HandleDirRequest(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	name := vars["name"]
	log.Println("handling HandleDirRequest", vars)
	//var sRes string
	if name == "photos" {
		fmt.Fprintf(w, "<!DOCTYPE html>")
		fmt.Fprintf(w, "<body>")
		for k, _ := range Mw.thumbView.ItemsMap {
			k = url.QueryEscape(k)
			fmt.Fprintf(w, "<a href='/users/photos/%s'><img src='/users/photos/%s' alt=Image />", k, k)
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
	if name == "photos" {
		if item != "" {
			if v, ok := Mw.thumbView.ItemsMap[item]; ok {
				w.Header().Set("Content-Type", "image/jpeg")
				w.Write(v.Imagedata)
				//log.Println("outputting", item)
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
