package main

/*
#cgo LDFLAGS: -lmpg123 -lasound
#include <mpg123.h>
#include <linux/soundcard.h>
#include <alsa/asoundlib.h>
*/
import "C"

import (
	"fmt"
	"os"

	"net/http"
	"net/url"

	"crypto/tls"
	"syscall"
	"time"
	"unsafe"
)

var counterFlag int = 0
var sch = make(chan bool)
var counterRunning bool = true
var songEntry string

func remove_file() {
	if songEntry[:5] == "/tmp/" {
		os.Remove(songEntry)
	}
}

func counter() {
	defer close(sch)
	h, m, s, us := 0, 0, 0, 0

	for {
		<-sch

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

	}
}

func Mp3Player(songlist <-chan string) {
	if counterRunning {
		go counter()
		counterRunning = false
	}

	defer func() {
		if p := recover(); p != nil {
			fmt.Println("PANIC ", p)
			counterFlag = 0
			remove_file()
			Mp3Player(songlist)
		}
	}()

	var mh *C.mpg123_handle

	C.mpg123_init()
	mh = C.mpg123_new(nil, nil)

	defer C.mpg123_exit()

	var ok bool

	for {
		songEntry, ok = <-songlist

		if !ok {
			return
		}

		var res C.int

		if url, _ := url.Parse(songEntry); url.Scheme == "http" || url.Scheme == "https" {
			var client *http.Client

			if url.Scheme == "https" {
				transport := &http.Transport{}
				transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
				client = &http.Client{
					Transport: transport,
				}
			} else {
				client = &http.Client{}
			}

			request, err := http.NewRequest("GET", url.String(), nil)

			if err != nil {
				fmt.Println(fmt.Errorf("Get request Error: %s", err.Error()))
				remove_file()
				continue
			}

			response, err := client.Do(request)

			if err != nil {
				fmt.Println(fmt.Errorf("No reponse from server: %s", err.Error()))
				remove_file()
				continue
			}

			if response.Status != "200 OK" {
				fmt.Println(response.Status)
				remove_file()
				continue
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
			res = C.mpg123_open(mh, C.CString(songEntry))
		}

		if res < 0 {
			fmt.Println("Error opening file")
			remove_file()
			continue
		}

		defer C.mpg123_close(mh)

		var (
			rate     C.long
			encoding C.int
			channels C.int
			swidth   C.int = 16
		)

		C.mpg123_getformat(mh, &rate, &channels, &encoding)

		buf := make([]byte, 3*C.int(rate)*swidth*channels/8)

		var pcmhandle *C.snd_pcm_t

		res = C.snd_pcm_open(&pcmhandle, C.CString("default"), C.SND_PCM_STREAM_PLAYBACK, 0)

		if res < 0 {
			panic(C.snd_strerror(res))
		}

		var hwparams *C.snd_pcm_hw_params_t

		res = C.snd_pcm_hw_params_malloc(&hwparams)

		if res < 0 {
			panic(C.snd_strerror(res))
		}

		res = C.snd_pcm_hw_params_any(pcmhandle, hwparams)

		if res < 0 {
			C.snd_pcm_hw_params_free(hwparams)
			panic(C.snd_strerror(res))
		}

		var endianess uint32 = 0x01020304

		if (*[4]byte)(unsafe.Pointer(&endianess))[0] == 0x04 {
			endianess = C.SND_PCM_FORMAT_S16_LE
		} else {
			endianess = C.SND_PCM_FORMAT_S16_BE
		}

		C.snd_pcm_hw_params_set_format(pcmhandle, hwparams, C.snd_pcm_format_t(endianess))
		C.snd_pcm_hw_params_set_rate(pcmhandle, hwparams, C.uint(rate), 0)
		C.snd_pcm_hw_params_set_channels(pcmhandle, hwparams, C.uint(channels))
		C.snd_pcm_hw_params(pcmhandle, hwparams)
		C.snd_pcm_hw_params_free(hwparams)

		var (
			frames C.long
			sizee  C.ulong
		)

		C.snd_pcm_prepare(pcmhandle)
		defer C.snd_pcm_close(pcmhandle)

		res = C.mpg123_read(mh, (*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(len(buf)), &sizee)

		if sizee > 0 && (res == C.MPG123_OK || res == C.MPG123_DONE) {
			counterFlag = 1
			sch <- true
			frames = C.snd_pcm_bytes_to_frames(pcmhandle, C.long(sizee))
			C.snd_pcm_writei(pcmhandle, unsafe.Pointer(&buf[0]), C.ulong(frames))
		} else {
			counterFlag = 0
			remove_file()
			panic("Couldn't start playback")
		}

		for {
			res = C.mpg123_read(mh, (*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(len(buf)), &sizee)

			if sizee > 0 && (res == C.MPG123_OK || res == C.MPG123_DONE) {
				frames = C.snd_pcm_bytes_to_frames(pcmhandle, C.long(sizee))
				C.snd_pcm_writei(pcmhandle, unsafe.Pointer(&buf[0]), C.ulong(frames))
			} else {
				break
			}
		}

		C.snd_pcm_drain(pcmhandle)
		counterFlag = 0
		remove_file()
	}
}
