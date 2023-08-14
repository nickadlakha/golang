package main

import (
	"fmt"
	jst "gowebjukebox/jukeboxstruct"
	"gowebjukebox/mp3player"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"

	"encoding/json"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
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

func getAMQPConnection(host string) (*amqp.Channel, *amqp.Channel, *amqp.Connection, error) {
	var connection *amqp.Connection
	var err error

	if host != "" {
		connection, err = amqp.Dial("amqp://" + host)
		msgQ.AmqpHost = host
	} else {
		amqp_host := os.Getenv("AMQP_HOST")

		if amqp_host == "" {
			amqp_host = jst.AmqpHost
		}

		msgQ.AmqpHost = amqp_host

		var amqp_port int

		amqp_port, err = strconv.Atoi(os.Getenv("AMQP_PORT"))

		if err != nil || amqp_port == 0 {
			amqp_port = jst.AmqpPort
		}

		amqp_user := os.Getenv("AMQP_USER")
		amqp_passwd := os.Getenv("AMQP_PASSWD")
		var amqp_url string

		if amqp_host != "" {
			amqp_url = fmt.Sprintf("amqp://%s:%s@%s:%d", amqp_user, amqp_passwd, amqp_host, amqp_port)
		} else {
			amqp_url = fmt.Sprintf("amqp://%s:%d", amqp_host, amqp_port)
		}

		connection, err = amqp.Dial(amqp_url)
	}

	if err != nil {
		return nil, nil, nil, err
	}

	mp3channel, err := connection.Channel()

	if err != nil {
		connection.Close()
		return nil, nil, nil, err
	}

	pcmchannel, err := connection.Channel()

	if err != nil {
		mp3channel.Close()
		connection.Close()
		return nil, nil, nil, err
	}

	return mp3channel, pcmchannel, connection, nil

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
	} else if flag == "3" && !msgQ.FlagMaster {
		songEntry = r.FormValue("mp3sync")

		if songEntry == "" {
			erroStr += "Master Host Not Provided"
			goto BRK
		}

		msgQ.Sync = jst.Sink
		var err error

		msgQ.Mp3Data, msgQ.PcmData, msgQ.Connection, err = getAMQPConnection(songEntry)

		if err != nil {
			erroStr += err.Error()
			goto BRK
		}

		/* end */

		_, err = msgQ.PcmData.(*amqp.Channel).QueueDeclare("pcm_queue", false, true, false, false, nil)

		if err != nil {
			erroStr += err.Error()
			goto BRK
		}

		err = msgQ.PcmData.(*amqp.Channel).QueueBind("pcm_queue", "#", "pcmdata", false, nil)

		if err != nil {
			erroStr += err.Error()
			goto BRK
		}

		_, err = msgQ.Mp3Data.(*amqp.Channel).QueueDeclare("mp3_queue", false, true, false, false, nil)

		if err != nil {
			erroStr += err.Error()
			goto BRK
		}

		err = msgQ.Mp3Data.(*amqp.Channel).QueueBind("mp3_queue", "#", "mp3data", false, nil)

		if err != nil {
			erroStr += err.Error()
			goto BRK
		}

		msgQ.FlagMaster = true
		message = "Syncing With Master @ " + songEntry
	} else if flag == "4" {
		if msgQ.FlagMaster {
			msgQ.FlagMaster = false
			msgQ.Sync = jst.Stype(0)
			msgQ.SkipSinkHost = true

			message = "Successfully Unsynced"

			msgQ.Mp3Data.(*amqp.Channel).Close()

			msgQ.PcmData.(*amqp.Channel).Close()

			msgQ.Connection.(*amqp.Connection).Close()

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
			go mp3player.Mp3Player(songlist, &msgQ)
			msgQ.PlayerR = true
		}

		mu.Unlock()
	}

IBRK:
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, fmt.Sprintf(htmlData, message))
}

func main() {
	masterNode, _ := strconv.Atoi(os.Getenv("SMASTER"))

	if masterNode == 1 {
		msgQ.Sync = jst.Stream
		var err error
		msgQ.Mp3Data, msgQ.PcmData, msgQ.Connection, err = getAMQPConnection("")

		if err != nil {
			panic(err)
		}

		err = msgQ.Mp3Data.(*amqp.Channel).ExchangeDeclare("mp3data", "fanout", false, false, false, false, nil)

		if err != nil {
			panic(err)
		}

		err = msgQ.PcmData.(*amqp.Channel).ExchangeDeclare("pcmdata", "fanout", false, false, false, false, nil)

		if err != nil {
			panic(err)
		}

		defer msgQ.Connection.(*amqp.Connection).Close()
		defer msgQ.PcmData.(*amqp.Channel).Close()
		defer msgQ.Mp3Data.(*amqp.Channel).Close()
	}

	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if msgQ.FlagMaster {
			http.ServeFile(w, r, "html/master.html")
		} else {
			http.ServeFile(w, r, "html/index.html")
		}

	}).Methods("GET")

	router.HandleFunc("/jquery.min.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "html/jquery.min.js")
	}).Methods("GET")

	router.HandleFunc("/pcmdata", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		encoder.Encode(msgQ.Mp3Info)
	}).Methods("GET")

	router.HandleFunc("/", hFunc).Methods("POST")
	http.Handle("/", router)

	fmt.Println("Error Server--> ", http.ListenAndServe(":3000", nil))
}
