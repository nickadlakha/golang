package main

import (
	"fmt"
	"gowebjukebox/audioplayer"
	jst "gowebjukebox/jukeboxstruct"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
)

var (
	mu       sync.Mutex
	songlist = make(chan string, 100)
	msgQ     jst.JukeboxStruct
)

var htmlData = `
	<html>
      <head>
        <title>Response</title>
		  <meta http-equiv = "refresh" content = "3; url = /"/>
	  </head>
	  <body>
	    <p>%s</p>
	  </body>
	</html>
	`

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
		file, fheader, err := r.FormFile("audiofile")

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
			message = "audio/video file added to playlist"
		}
	} else if flag == "1" {
		songEntry = r.FormValue("audiourl")

		if songEntry == "" {
			erroStr += "No Url Provided"
			goto BRK
		}

		message = "audio/video url added to playlist"
	} else if flag == "3" && !msgQ.FlagMaster {
		songEntry = r.FormValue("audiosync")

		if songEntry == "" {
			erroStr += "Master Host Not Provided"
			goto BRK
		}

		msgQ.Sync = jst.Sink
		msgQ.FlagMaster = true
		message = "Syncing With Master @ " + songEntry
	} else if flag == "4" {
		if msgQ.FlagMaster {
			msgQ.FlagMaster = false
			msgQ.Sync = jst.Stype(0)
			msgQ.SkipSinkHost = true
			audioplayer.StopClient()
			message = "Successfully Unsynced"
			goto IBRK
		}

		erroStr += "Unsync Not Successful"
	} else {
		erroStr += "Malformed Input"
	}

BRK:

	if erroStr != "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, fmt.Sprintf(htmlData, erroStr))
		return
	} else {
		mu.Lock()
		songlist <- songEntry
		mu.Unlock()
	}

	if !msgQ.PlayerR {
		mu.Lock()

		if !msgQ.PlayerR {
			go audioplayer.Player(songlist, &msgQ)
			msgQ.PlayerR = true
		}

		mu.Unlock()
	}

IBRK:
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, fmt.Sprintf(htmlData, message))
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	masterNode, _ := strconv.Atoi(os.Getenv("SMASTER"))

	if masterNode == 1 {
		msgQ.Sync = jst.Stream
		go audioplayer.StartServer()
		defer audioplayer.StopServer()
	}

	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if msgQ.FlagMaster {
			http.ServeFile(w, r, "html/master_new.html")
		} else {
			http.ServeFile(w, r, "html/index_new.html")
		}

	}).Methods("GET")

	router.HandleFunc("/jquery.min.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "html/jquery.min.js")
	}).Methods("GET")

	router.HandleFunc("/", hFunc).Methods("POST")
	http.Handle("/", router)
	log.Println("Listening on port 3000")
	log.Println("Error Server--> ", http.ListenAndServe(":3000", nil))
}
