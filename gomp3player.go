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

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [url|filename|-]\n", os.Args[0])
		os.Exit(2)
	}

	var mh *C.mpg123_handle

	C.mpg123_init()
	mh = C.mpg123_new(nil, nil)

	var res C.int

	if len(os.Args[1]) == 1 && os.Args[1] == "-" {
		res = C.mpg123_open_fd(mh, 0)
	}

	url, _ := url.Parse(os.Args[1])

	if url.Scheme == "http" || url.Scheme == "https" {
		var client *http.Client

		if url.Scheme == "https" {
			transport := &http.Transport{}
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			client = &http.Client{Transport: transport}
		} else {
			client = &http.Client{}
		}

		request, err := http.NewRequest("GET", url.String(), nil)

		if err != nil {
			panic(fmt.Errorf("Get request Error: %s", err.Error()))
		}

		response, err := client.Do(request)

		if err != nil {
			panic(fmt.Errorf("No reponse from server: %s", err.Error()))
		}

		if response.Status != "200 OK" {
			panic(response.Status)
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
		panic("Error opening file")
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

	sch := make(chan bool)
	counterFlag := 1

	go func() {
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
		sch <- true
	}()

	defer close(sch)

	res = C.mpg123_read(mh, (*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(len(buf)), &sizee)

	if sizee > 0 && (res == C.MPG123_OK || res == C.MPG123_DONE) {
		sch <- true
		frames = C.snd_pcm_bytes_to_frames(pcmhandle, C.long(sizee))
		C.snd_pcm_writei(pcmhandle, unsafe.Pointer(&buf[0]), C.ulong(frames))
	} else {
		counterFlag = 0
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

	<-sch
}
