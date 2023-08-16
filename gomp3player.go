package main

/*
#cgo LDFLAGS: -lmpg123 -lao
#include <unistd.h>
#include <mpg123.h>
#include <ao/ao.h>
*/
import "C"

import (
	"fmt"
	"os"

	"log"
	"net/http"
	"net/url"

	"crypto/tls"
	"syscall"
	"time"
	"unsafe"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [url|filename|-]\n", os.Args[0])
		os.Exit(1)
	}

	var mh *C.mpg123_handle
	var res C.int

	C.mpg123_init()
	mh = C.mpg123_new(nil, nil)

	if len(os.Args[1]) == 1 && os.Args[1] == "-" {
		res = C.mpg123_open_fd(mh, 0)
	} else if url, _ := url.Parse(os.Args[1]); url.Scheme == "http" || url.Scheme == "https" {
		var client *http.Client

		if url.Scheme == "https" {
			transport := &http.Transport{}
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			client = &http.Client{
				Transport: transport,
				Timeout:   5 * time.Minute,
			}
		} else {
			client = &http.Client{
				Timeout: 5 * time.Minute,
			}
		}

		request, err := http.NewRequest("GET", url.String(), nil)

		if err != nil {
			log.Fatalf("Get request Error: %s", err.Error())
		}

		response, err := client.Do(request)

		if err != nil {
			log.Fatalf("No reponse from server: %s", err.Error())
		}

		if response.Status != "200 OK" {
			log.Fatalf("%v", response.Status)
		}

		var (
			lbuf [1024]byte
			pfd  [2]C.int
		)

		C.pipe((*C.int)(unsafe.Pointer(&pfd)))

		res = C.mpg123_open_fd(mh, pfd[0])
		defer syscall.Close(int(pfd[0]))

		go func() {
			defer syscall.Close(int(pfd[1]))
			defer response.Body.Close()

			for {
				n, err := response.Body.Read(lbuf[:])

				if n <= 0 && err != nil {
					break
				}

				syscall.Write(int(pfd[1]), lbuf[:n])
			}
		}()

	} else {
		res = C.mpg123_open(mh, C.CString(os.Args[1]))
	}

	defer C.mpg123_exit()

	if res < 0 {
		log.Fatalf("Error opening file")
	}

	defer C.mpg123_close(mh)

	var (
		rate     C.long
		encoding C.int
		channels C.int
	)

	C.mpg123_getformat(mh, &rate, &channels, &encoding)

	var format C.ao_sample_format

	format.bits = C.mpg123_encsize(encoding) * 8
	format.rate = (C.int)(rate)
	format.channels = channels
	format.byte_format = C.AO_FMT_NATIVE
	format.matrix = nil

	C.ao_initialize()
	defer C.ao_shutdown()

	driver := C.ao_default_driver_id()

	if driver == -1 {
		log.Panic("Couldn't get the default driver id")
	}

	var dev *C.ao_device = C.ao_open_live(driver, &format, nil)

	if dev == nil {
		log.Panic("Couldn't open a live playback audio device")
	}

	defer C.ao_close(dev)

	sch := make(chan bool)
	counterFlag := 1

	go func() {
		defer close(sch)
		<-sch

		h, m, s, us := 0, 0, 0, 0

		for counterFlag > 0 {
			time.Sleep(100000000 * time.Nanosecond)
			fmt.Fprintf(os.Stderr, "\r%02d:%02d:%02d:%0d", h, m, s, us)
			us++

			if us > 9 {
				s = s + 1
				us = 0
			}

			if s > 59 {
				m = m + 1
				s = 0
			}

			if m > 59 {
				h = h + 1
				m = 0
			}

			if h > 23 {
				h = 0
			}
		}

		fmt.Fprintf(os.Stderr, "\n")
	}()

	sreadlen := int(3 * C.int(rate) * C.mpg123_encsize(encoding) * 8 * channels / 8)
	buf := make([]byte, sreadlen*2)
	gbindex := 0
	bindex := 0
	chpcm := make(chan int, 1)

	var sizee C.ulong

	go func() {
		rbuflen := 0

		for {
			rbuflen = <-chpcm

			if rbuflen == 0 {
				counterFlag = 0
				break
			}

			C.ao_play(dev, (*C.char)(unsafe.Pointer(&buf[gbindex])), C.uint(rbuflen))
			gbindex ^= sreadlen
		}
	}()

	res = C.mpg123_read(mh, unsafe.Pointer(&buf[bindex]), C.ulong(sreadlen), &sizee)

	if sizee > 0 && (res == C.MPG123_OK || res == C.MPG123_DONE) {
		sch <- true
		chpcm <- int(sizee)
		bindex ^= sreadlen
	} else {
		counterFlag = 0
		close(chpcm)
		log.Panic("Couldn't start playback")
	}

	for {
		if gbindex != bindex {
			res = C.mpg123_read(mh, unsafe.Pointer(&buf[bindex]), C.ulong(sreadlen), &sizee)

			if sizee > 0 && (res == C.MPG123_OK || res == C.MPG123_DONE) {
				chpcm <- int(sizee)
				bindex ^= sreadlen
			} else {
				close(chpcm)
				break
			}
		}
	}

	<-sch
}
