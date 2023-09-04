package audioplayer

/*
#cgo LDFLAGS: -lsctp -lavformat -lavcodec -lavutil -lasound
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <signal.h>
#include <sys/select.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <netinet/sctp.h>
#include <libavformat/avformat.h>
#include <libavcodec/avcodec.h>
#include <alsa/asoundlib.h>
#include <linux/soundcard.h>

#define die(...) {\
                        fprintf(stderr, __VA_ARGS__);\
                        return -1;\
                 }
#define SIZE 1024
#define SPORT 3124

enum {NC, CC, DC};
snd_pcm_t *pcmhandle = NULL;
snd_pcm_hw_params_t *hwparams = NULL;
unsigned char buf[SIZE];
int server_socket = -1, client_socket = -1, clients_connected = NC;

int prepare_snd_device(int channels, int sample_format, int sample_rate) {
	int res = 0;

	if (!pcmhandle) {
		if ((res = snd_pcm_open(&pcmhandle, "default", SND_PCM_STREAM_PLAYBACK, 0)) < 0) {
			die("can't open sound device: %s\n", snd_strerror(res));
		}
	}

	if ((res = snd_pcm_hw_params_malloc(&hwparams)) < 0) {
		die("hwparams couldn't be queried: %s\n", snd_strerror(res));
	}

	if ((res = snd_pcm_hw_params_any(pcmhandle, hwparams)) < 0) {
		snd_pcm_hw_params_free(hwparams);
		die("sound card can't be initialized: %s\n", snd_strerror(res));
	}

	snd_pcm_hw_params_set_format(pcmhandle, hwparams, sample_format);
	snd_pcm_hw_params_set_rate_near(pcmhandle, hwparams, &sample_rate, NULL);
	snd_pcm_hw_params_set_channels_near(pcmhandle, hwparams, &channels);
	snd_pcm_hw_params(pcmhandle, hwparams);
	snd_pcm_hw_params_free(hwparams);
	snd_pcm_prepare(pcmhandle);
	return 0;
}

int server(int *sockfd) {
	int slen, flags;
	struct sockaddr_in serv_addr, client_addr;
	struct sctp_sndrcvinfo sinfo;
	fd_set          rfds;

	*sockfd = socket(AF_INET, SOCK_SEQPACKET, IPPROTO_SCTP);

	if (*sockfd < 0) {
		die("Error creating sctp socket");
	}

	bzero(&serv_addr, sizeof(serv_addr));
	serv_addr.sin_family = AF_INET;
	serv_addr.sin_addr.s_addr = htonl(INADDR_ANY);
	serv_addr.sin_port = htons(SPORT);

	if (bind(*sockfd, (struct sockaddr *) &serv_addr, sizeof(serv_addr)) < 0) {
		die("Address binding failed");
	}

	struct sctp_event_subscribe events;

	bzero(&events, sizeof(events));
	events.sctp_association_event = 1;

	FD_ZERO(&rfds);
	FD_SET(*sockfd, &rfds);
	if (setsockopt(*sockfd, IPPROTO_SCTP, SCTP_EVENTS, &events, sizeof(events))) {
		die("set sock opt\n");
	}

	listen(*sockfd, SOMAXCONN);
	printf("Listening on sctp server port %d\n", SPORT);
	slen = sizeof(client_addr);
	select(*sockfd + 1, &rfds, NULL, NULL, NULL);

	sctp_recvmsg(*sockfd, buf, SIZE, (struct sockaddr *) &client_addr, &slen, &sinfo,
		&flags);

	clients_connected = CC;
	return 0;
}

void wait_for_client(int sockfd) {
	struct sockaddr_in client_addr;
	struct sctp_sndrcvinfo sinfo;
	int slen, flags;

	sctp_recvmsg(sockfd, buf, SIZE, (struct sockaddr *) &client_addr, &slen, &sinfo,
		&flags);
}

int client(const char *addr, int *sockfd) {
	struct sockaddr_in serv_addr, client_addr;
	struct sctp_sndrcvinfo sinfo;
	int slen, flags;

	*sockfd = socket(AF_INET, SOCK_STREAM, IPPROTO_SCTP);

	if (*sockfd < 0) {
		die("Error creating sctp socket");
	}

	bzero(&serv_addr, sizeof(serv_addr));
	serv_addr.sin_family = AF_INET;
	serv_addr.sin_addr.s_addr = inet_addr(addr);
	serv_addr.sin_port = htons(SPORT);

	if (connect(*sockfd, (struct sockaddr *)&serv_addr, sizeof(serv_addr)) < 0) {
    		die("connect to server failed");
	}

	printf("Connected to server port %d\n", SPORT);

	int res = 0;
	uint32_t ad_data = 0;
	long frames = 0;
	int abuf_s = 320 * 1024;
	int count = 0;
	uint8_t rbuf[33*1024];
	uint8_t abuf[abuf_s];

	while ((res = sctp_recvmsg(*sockfd, rbuf, sizeof(rbuf), (struct sockaddr *) &client_addr, &slen, &sinfo, &flags)) > 0) {
		while ((flags & MSG_EOR) == 0) {
			res += sctp_recvmsg(*sockfd, rbuf + res, sizeof(rbuf)-res, NULL, &slen, NULL, &flags);
		}


		if (ad_data != *(uint32_t *)rbuf) {
			ad_data = *(uint32_t *)rbuf;

			if (prepare_snd_device(rbuf[0], rbuf[1], ntohs(*(uint16_t *)(rbuf + 2))) < 0) {
				return -1;
			}
		}

		if ((abuf_s - count) < (res - 4)) {
			frames = snd_pcm_bytes_to_frames(pcmhandle, count);
			REWRITEI:
			if (snd_pcm_writei(pcmhandle, abuf, frames) < 0) {
				snd_pcm_prepare(pcmhandle);
				goto REWRITEI;
			}
			count ^= count;
		}

		memcpy(abuf + count, rbuf + 4, res -4);
		count += res - 4;
	}

	if (count) {
		frames = snd_pcm_bytes_to_frames(pcmhandle, count);

		if (snd_pcm_writei(pcmhandle, abuf, frames) < 0) {
			snd_pcm_prepare(pcmhandle);
			snd_pcm_writei(pcmhandle, abuf, frames);
		}
	}

	return 0;
}

int AVSampleFormat_SNDFormat[] = {SND_PCM_FORMAT_U8, SND_PCM_FORMAT_S16, SND_PCM_FORMAT_S32, SND_PCM_FORMAT_FLOAT, SND_PCM_FORMAT_FLOAT64,
                                  SND_PCM_FORMAT_U8, SND_PCM_FORMAT_S16, SND_PCM_FORMAT_S32, SND_PCM_FORMAT_FLOAT, SND_PCM_FORMAT_FLOAT64,
                                  SND_PCM_FORMAT_FLOAT64, SND_PCM_FORMAT_FLOAT64};

int play(const char *url, int server_socket) {
	AVFormatContext *afctx = avformat_alloc_context();

	if(avformat_open_input(&afctx, url, NULL, NULL) < 0){
		die("Could not open %s", url);
	}

	if(avformat_find_stream_info(afctx, NULL) < 0){
		die("Could not find file info");
	}

	int stream_id = -1;
   	int i;

	for(i = 0; i < afctx->nb_streams; i++){
		if(afctx->streams[i]->codecpar->codec_type == AVMEDIA_TYPE_AUDIO){
			stream_id = i;
         	break;
    		}
	}

	if(stream_id == -1){
   		die("Could not find Audio Stream");
	}

	const AVCodec *codec = avcodec_find_decoder(afctx->streams[stream_id]->codecpar->codec_id);

 	if(codec==NULL){
   		die("cannot find codec!");
   	}

   	AVCodecContext *ctx = avcodec_alloc_context3(codec);

   	if (!ctx) {
       	die("failed to allocate AVCodecContext");
   	}

   	if (avcodec_parameters_to_context(ctx, afctx->streams[stream_id]->codecpar) < 0) {
       	die("failed to fill the AVCodecContext");
   	}

   	if(avcodec_open2(ctx, codec, NULL) < 0){
       	die("Codec cannot be found");
   	}

   	enum AVSampleFormat sfmt = ctx->sample_fmt;

   	if (sfmt == AV_SAMPLE_FMT_NONE) {
       	die("no sample fmt detected\n");
   	}

   	int channels = ctx->ch_layout.nb_channels;
   	int sample_rate = ctx->sample_rate;
   	int sampleSize = av_get_bytes_per_sample(sfmt);

	if (prepare_snd_device(channels, AVSampleFormat_SNDFormat[sfmt], sample_rate) < 0) {
		return -1;
	}

   	int res = 0;
   	AVPacket *packet = av_packet_alloc();
   	AVFrame *frame = av_frame_alloc();
   	int buf_siz = 3 * sample_rate * sampleSize * channels; // 3sec data
   	uint8_t buffer[buf_siz];
   	int count = 0;
   	int resend_packet = 0;
   	long frames = 0;
	int sbuffer_s = 32 * 1024;
	int f_sbuffer_s = 4 + sbuffer_s;
	int s_count = 0;
	uint8_t sbuffer[f_sbuffer_s];
	uint8_t *sbuffer_start = sbuffer + 4;
	struct sctp_sndrcvinfo sinfo;

	sbuffer[0] = channels;
    	sbuffer[1] = AVSampleFormat_SNDFormat[sfmt];
    	*(uint16_t *)(sbuffer + 2) = htons(sample_rate);
    	bzero(&sinfo, sizeof(sinfo));
    	sinfo.sinfo_flags |= SCTP_SENDALL;

	while(av_read_frame(afctx, packet) >= 0) {
		if(packet->stream_index == stream_id) {
			SRESENDPACKET:
			res = avcodec_send_packet(ctx, packet);

			if (res == AVERROR(EAGAIN)) {
				resend_packet = 1;
			}else if (res < 0) {
				fprintf(stderr, "Error in sendng packet\n");
				break;
			}

			while (res >= 0) {
				res = avcodec_receive_frame(ctx, frame);

				if (res == AVERROR(EAGAIN)) {
					continue;
				}else if(res == AVERROR_EOF || res < 0) {
					break;
				}

				if ((buf_siz - count) < frame->linesize[0] * 2) {
					frames = snd_pcm_bytes_to_frames(pcmhandle, count);

					SPREWRITEI:
					if (snd_pcm_writei(pcmhandle, (void **)buffer, frames) < 0) {
						snd_pcm_prepare(pcmhandle);
						goto SPREWRITEI;
					}

					if (clients_connected == CC) {
						s_count = count / sbuffer_s;
						for (i = 0; i < s_count; i++) {
							if (clients_connected == CC) {
								memcpy(sbuffer_start, buffer + i*sbuffer_s, sbuffer_s);
								res = sctp_send(server_socket, sbuffer, f_sbuffer_s, &sinfo, 0);
								if (res <= 0) {
									clients_connected = DC;
									kill(getpid(), SIGUSR1);
								}
							}
						}

						s_count = count % sbuffer_s;

						if (s_count && clients_connected == CC) {
							memcpy(sbuffer_start, buffer + i*sbuffer_s, s_count);
							res = sctp_send(server_socket, sbuffer, s_count + 4, &sinfo, 0);
							if (res <= 0) {
								clients_connected = DC;
								kill(getpid(), SIGUSR1);
							}
						}
					}

					count ^= count;
				}

				for (i = 0; i < frame->nb_samples; i++) {
					for (int ch = 0; ch < channels; ch++) {
						memcpy(buffer + count, &frame->data[ch][i*sampleSize], sampleSize);
						count += sampleSize;
					}
				}
			}

			if (resend_packet) {
				resend_packet ^= resend_packet;
				goto SRESENDPACKET;
			}
		}

		av_frame_unref(frame);
		av_packet_unref(packet);
	}

    snd_pcm_writei(pcmhandle, (void **)buffer, snd_pcm_bytes_to_frames(pcmhandle, count));

    if (count && clients_connected == CC) {
		s_count = count / sbuffer_s;
		for (i = 0; i < s_count; i++) {
			if (clients_connected == CC) {
				memcpy(sbuffer_start, buffer + i*sbuffer_s, sbuffer_s);
				res = sctp_send(server_socket, sbuffer, f_sbuffer_s, &sinfo, 0);
				if (res <= 0) {
					clients_connected = DC;
					kill(getpid(), SIGUSR1);
				}
			}
		}

		s_count = count % sbuffer_s;

		if (s_count && clients_connected == CC) {
			memcpy(sbuffer_start, buffer + i*sbuffer_s, s_count);
			res = sctp_send(server_socket, sbuffer, s_count + 4, &sinfo, 0);
			if (res <= 0) {
				clients_connected = DC;
				kill(getpid(), SIGUSR1);
			}
		}
	}

    avformat_close_input(&afctx);
    av_packet_free(&packet);
    av_frame_free(&frame);
    avcodec_free_context(&ctx);
    avcodec_close(ctx);
    return 0;
}
*/
import "C"

import (
	jst "gowebjukebox/jukeboxstruct"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var songEntry string

func remove_file() {
	if len(songEntry) > 5 && songEntry[:5] == "/tmp/" {
		os.Remove(songEntry)
	}
}

func StartServer() {
	C.server(&C.server_socket)
}

func StopServer() {
	if C.server_socket > 0 {
		C.close(C.server_socket)
		C.server_socket = 0
	}
}

func StopClient() {
	if C.client_socket > 0 {
		C.close(C.client_socket)
		C.client_socket = 0
	}
}

func Player(songlist <-chan string, msgQC *jst.JukeboxStruct) {
	inner_ := func() {
		res := C.client(C.CString(songEntry), &C.client_socket)

		if res < 0 {
			log.Printf("Error connecting to the server %s\n", songEntry)
		}
	}

	defer func() {
		if p := recover(); p != nil {
			log.Println("PANIC ", p)
			remove_file()
			Player(songlist, msgQC)
		}
	}()

	var ok bool
	var sigc chan os.Signal

	if C.server_socket > 0 {
		sigc = make(chan os.Signal, 1)
		defer close(sigc)
		go func() {
			for {
				signal.Notify(sigc, syscall.SIGUSR1)
				//defer signal.Reset(syscall.SIGUSR1)
				<-sigc

				if C.server_socket > 0 && C.clients_connected == C.DC {
					C.wait_for_client(C.server_socket)
					C.clients_connected = C.CC
				}
			}
		}()
	}

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
			return
		}

		if C.play(C.CString(songEntry), C.server_socket) < 0 {
			log.Println("Error playing ", songEntry)
		}

		remove_file()
	}

	if C.pcmhandle != nil {
		C.snd_pcm_drain(C.pcmhandle)
		C.snd_pcm_close(C.pcmhandle)
	}
}
