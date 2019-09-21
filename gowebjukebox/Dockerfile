FROM ubuntu:16.04
FROM golang:latest
MAINTAINER nicklesh.adlakha@gmail.com
RUN apt-get update && apt-get install -y \
	libmpg123-0 libmpg123-dev \
	libasound2-dev \
	alsa-utils \
	pulseaudio

RUN mkdir /GOJUKEBOX
WORKDIR /GOJUKEBOX
COPY . /GOJUKEBOX
RUN go get github.com/gorilla/mux
RUN go build gomp3player.go
RUN go build -o gojukebox jukebox.go   
EXPOSE 3000
CMD ["/GOJUKEBOX/gojukebox"]
