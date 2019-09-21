package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/gorilla/mux"
)

var (
	mu       sync.Mutex
	songlist = make(chan string, 100)
	playerR  = true
)

func Mp3Player(songlist <-chan string) {
	tmpdir := os.TempDir()

	for {
		songEntry, ok := <-songlist

		if !ok {
			break
		}

		exec.Command("./gomp3player", songEntry).Run()

		if songEntry[:len(tmpdir)] == tmpdir {
			os.Remove(songEntry)
		}
	}
}

func hFunc(w http.ResponseWriter, r *http.Request) {
	songEntry := ""
	erroStr := ""
	message := ""

	flag := r.FormValue("flag")

	if flag == "" {
		erroStr += "Malformed Input"
		goto BRK
	}

	if flag == "2" {
		file, fheader, err := r.FormFile("mp3file")

		if err != nil {
			erroStr += err.Error()
		} else {
			defer file.Close()
			f, err := ioutil.TempFile("", fheader.Filename)

			if err != nil {
				erroStr += err.Error()
				goto BRK
			}

			defer f.Close()
			io.Copy(f, file)
			songEntry = f.Name()
			message = "file added to playlist"
		}
	} else if flag == "1" {
		songEntry = r.FormValue("mp3url")

		if songEntry == "" {
			erroStr += "No Url Provided"
			goto BRK
		}

		message = "mp3 url added to playlist"
	} else {
		erroStr += "Malformed Input"
	}

BRK:

	if erroStr != "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, erroStr)
		return
	} else {
		mu.Lock()
		defer mu.Unlock()
		songlist <- songEntry
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, message)
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if playerR {
			go Mp3Player(songlist)
			playerR = false
		}

		http.ServeFile(w, r, "html/index.html")
	}).Methods("GET")

	router.HandleFunc("/jquery.min.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "scripts/jquery.min.js")
	}).Methods("GET")

	router.HandleFunc("/", hFunc).Methods("POST")
	http.Handle("/", router)

	fmt.Println("Error Server--> ", http.ListenAndServe(":3000", nil))
}
