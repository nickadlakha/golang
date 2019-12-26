package mp3player

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
	jst "gowebjukebox/jukeboxstruct"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"encoding/json"

	"io/ioutil"

	"github.com/streadway/amqp"
)

var songEntry string
var mh *C.mpg123_handle
var initializeOnce = true

func remove_file() {
	if songEntry[:5] == "/tmp/" {
		os.Remove(songEntry)
	}
}

func Mp3Player(songlist <-chan string, msgQC *jst.JukeboxStruct) {
	var pcmhandle *C.snd_pcm_t
	var res C.int
	var frames C.long
	var hwparams *C.snd_pcm_hw_params_t
	var endianess uint32 = 0x01020304

	if (*[4]byte)(unsafe.Pointer(&endianess))[0] == 0x04 {
		endianess = C.SND_PCM_FORMAT_S16_LE
	} else {
		endianess = C.SND_PCM_FORMAT_S16_BE
	}

	inner_ := func() {

		var mp3info jst.Mp3Info_
		var wg sync.WaitGroup
		var pcmDataFlag = false

		pcm_ep := strings.Split(msgQC.AmqpHost, "@")
		pcm_ep = strings.Split(pcm_ep[len(pcm_ep)-1], ":")

		resp, err := http.Get(fmt.Sprintf("http://%s:3000/pcmdata", pcm_ep[0]))

		if err == nil {
			pcmdata, err := ioutil.ReadAll(resp.Body)

			if err == nil {
				pcmDataFlag = true
				json.Unmarshal(pcmdata, &mp3info)

				res = C.snd_pcm_open(&pcmhandle, C.CString("default"), C.SND_PCM_STREAM_PLAYBACK, 0)

				if res < 0 {
					fmt.Println(C.snd_strerror(res))
				}

				res = C.snd_pcm_hw_params_malloc(&hwparams)

				if res < 0 {
					fmt.Println(C.snd_strerror(res))
				}

				res = C.snd_pcm_hw_params_any(pcmhandle, hwparams)

				if res < 0 {
					C.snd_pcm_hw_params_free(hwparams)
					fmt.Println(C.snd_strerror(res))
				}

				C.snd_pcm_hw_params_set_format(pcmhandle, hwparams, C.snd_pcm_format_t(endianess))
				C.snd_pcm_hw_params_set_rate(pcmhandle, hwparams, C.uint(mp3info.Rate), 0)
				C.snd_pcm_hw_params_set_channels(pcmhandle, hwparams, C.uint(mp3info.Channels))
				C.snd_pcm_hw_params(pcmhandle, hwparams)
				C.snd_pcm_hw_params_free(hwparams)
			}
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			pcmMsgChannel, err := msgQC.PcmData.(*amqp.Channel).Consume("pcm_queue", "", false, false, false, false, nil)

			if err != nil {
				fmt.Println("pcm consume channel ", err)
			} else {
				for msg := range pcmMsgChannel {
					json.Unmarshal(msg.Body, &mp3info)

					if !pcmDataFlag {
						pcmDataFlag = true
					}

					msg.Ack(false)
				}

			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			mp3MsgChannel, err := msgQC.Mp3Data.(*amqp.Channel).Consume("mp3_queue", "", false, false, false, false, nil)

			if err != nil {
				fmt.Println("mp3 consume channel ", err)
			} else {
				C.snd_pcm_prepare(pcmhandle)
				var llen C.long

				for msg := range mp3MsgChannel {
					if pcmDataFlag {
						pcmDataFlag = false
						C.snd_pcm_drain(pcmhandle)
						C.snd_pcm_close(pcmhandle)

						res = C.snd_pcm_open(&pcmhandle, C.CString("default"), C.SND_PCM_STREAM_PLAYBACK, 0)

						if res < 0 {
							fmt.Println(C.snd_strerror(res))
						}

						res = C.snd_pcm_hw_params_malloc(&hwparams)

						if res < 0 {
							fmt.Println(C.snd_strerror(res))
						}

						res = C.snd_pcm_hw_params_any(pcmhandle, hwparams)

						if res < 0 {
							C.snd_pcm_hw_params_free(hwparams)
							fmt.Println(C.snd_strerror(res))
						}

						C.snd_pcm_hw_params_set_format(pcmhandle, hwparams, C.snd_pcm_format_t(endianess))
						C.snd_pcm_hw_params_set_rate(pcmhandle, hwparams, C.uint(mp3info.Rate), 0)
						C.snd_pcm_hw_params_set_channels(pcmhandle, hwparams, C.uint(mp3info.Channels))
						C.snd_pcm_hw_params(pcmhandle, hwparams)
						C.snd_pcm_hw_params_free(hwparams)
						C.snd_pcm_prepare(pcmhandle)

					}

					frames = C.snd_pcm_bytes_to_frames(pcmhandle, C.long(len(msg.Body)))
					llen = C.snd_pcm_writei(pcmhandle, unsafe.Pointer(&msg.Body[0]), C.ulong(frames))

					if llen <= 0 {
						fmt.Println("Buffer Underrun ", llen)
						C.snd_pcm_prepare(pcmhandle)
						C.snd_pcm_writei(pcmhandle, unsafe.Pointer(&msg.Body[0]), C.ulong(frames))
					}

					msg.Ack(false)
				}

			}
		}()

		wg.Wait()
		C.snd_pcm_drain(pcmhandle)
		C.snd_pcm_close(pcmhandle)
	}

	if msgQC.Sync == jst.Sink {
		inner_()
	}

	var pcmMessage amqp.Publishing
	var mp3Message amqp.Publishing

	defer func() {
		if p := recover(); p != nil {
			fmt.Println("PANIC ", p)
			remove_file()
			C.mpg123_close(mh)
			Mp3Player(songlist, msgQC)
		}
	}()

	if initializeOnce {
		C.mpg123_init()
		initializeOnce = false
	}

	mh = C.mpg123_new(nil, nil)

	var ok bool

	for {
		songEntry, ok = <-songlist

		if msgQC.Sync == jst.Sink {
			inner_()
			continue
		} else if msgQC.SkipSinkHost {
			msgQC.SkipSinkHost = false
			continue
		}

		if !ok {
			msgQC.PlayerR = false
			initializeOnce = true
			C.mpg123_exit()
			return
		}

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

			go func() {
				defer syscall.Close(int(pfd[1]))
				defer response.Body.Close()
				defer syscall.Close(int(pfd[0]))

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

		var (
			rate     C.long
			encoding C.int
			channels C.int
			swidth   C.int = 16
		)

		C.mpg123_getformat(mh, &rate, &channels, &encoding)

		if msgQC.Sync == jst.Stream {
			msgQC.Mp3Info.Rate, msgQC.Mp3Info.Channels, msgQC.Mp3Info.Swidth = int(rate), int(channels), int(swidth)
			pcmdbytes, _ := json.Marshal(msgQC.Mp3Info)

			pcmMessage.Body = pcmdbytes

			msgQC.PcmData.(*amqp.Channel).Publish("pcmdata", "", false, false, pcmMessage)
		}

		buf := make([]byte, 3*C.int(rate)*swidth*channels/8)

		res = C.snd_pcm_open(&pcmhandle, C.CString("default"), C.SND_PCM_STREAM_PLAYBACK, 0)

		if res < 0 {
			panic(C.snd_strerror(res))
		}

		res = C.snd_pcm_hw_params_malloc(&hwparams)

		if res < 0 {
			panic(C.snd_strerror(res))
		}

		res = C.snd_pcm_hw_params_any(pcmhandle, hwparams)

		if res < 0 {
			C.snd_pcm_hw_params_free(hwparams)
			panic(C.snd_strerror(res))
		}

		C.snd_pcm_hw_params_set_format(pcmhandle, hwparams, C.snd_pcm_format_t(endianess))
		C.snd_pcm_hw_params_set_rate(pcmhandle, hwparams, C.uint(rate), 0)
		C.snd_pcm_hw_params_set_channels(pcmhandle, hwparams, C.uint(channels))
		C.snd_pcm_hw_params(pcmhandle, hwparams)
		C.snd_pcm_hw_params_free(hwparams)

		var (
			sizee C.ulong
		)

		C.snd_pcm_prepare(pcmhandle)

		if msgQC.Sync == jst.Stream {
			var err error

			for {
				res = C.mpg123_read(mh, (*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(len(buf)), &sizee)

				if sizee > 0 && (res == C.MPG123_OK || res == C.MPG123_DONE) {

					mp3Message.Body = buf[:sizee]
					err = msgQC.Mp3Data.(*amqp.Channel).Publish("mp3data", "", false, false, mp3Message)

					if err != nil {
						fmt.Fprintf(os.Stderr, err.Error())
					}

					frames = C.snd_pcm_bytes_to_frames(pcmhandle, C.long(sizee))
					C.snd_pcm_writei(pcmhandle, unsafe.Pointer(&buf[0]), C.ulong(frames))
				} else {
					break
				}
			}
		} else {

			for {
				res = C.mpg123_read(mh, (*C.uchar)(unsafe.Pointer(&buf[0])), C.ulong(len(buf)), &sizee)

				if sizee > 0 && (res == C.MPG123_OK || res == C.MPG123_DONE) {
					frames = C.snd_pcm_bytes_to_frames(pcmhandle, C.long(sizee))
					C.snd_pcm_writei(pcmhandle, unsafe.Pointer(&buf[0]), C.ulong(frames))
				} else {
					break
				}
			}
		}

		C.snd_pcm_drain(pcmhandle)
		remove_file()
		C.snd_pcm_close(pcmhandle)
		C.mpg123_close(mh)
	}
}
