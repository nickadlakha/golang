package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func main() {
	cmd := exec.Command("lsmod")
	out, err := cmd.CombinedOutput()

	if err != nil {
		log.Fatal(err)
	}

	if !strings.Contains(strings.ToLower(string(out)), "snd_pcm_oss") {
		log.Fatal("no oss device loaded, kindly load snd_pcm_oss kernel module")
	}

	var oss_device string

	cmd = exec.Command("sh", "-c", "ls /dev/dsp*")
	out, err = cmd.CombinedOutput()

	if err != nil {
		cmd = exec.Command("sh", "-c", "ls /dev/audio*")
		out, err = cmd.CombinedOutput()

		if err != nil {
			log.Fatal("no audio dev node found")
		}
	}

	sar := strings.Split(string(out), "\n")
	oss_device = sar[0]

	fmt.Printf("Selected audio dev node %s\n", oss_device)

	if len(os.Args) < 2 {
		log.Fatal(fmt.Errorf(" --> Usage: %s raw_audio_file [audio_frequency]\n", os.Args[0]))
	}

	var AFMT_S16_NE uint32 = 0x01020304

	if (*[4]byte)(unsafe.Pointer(&AFMT_S16_NE))[0] == 0x04 {
		AFMT_S16_NE = 0x00000010
	} else {
		AFMT_S16_NE = 0x00000020
	}

	var SNDCTL_DSP_CHANNELS, SNDCTL_DSP_SPEED, SNDCTL_DSP_SETFMT uint32 = 3221508102, 3221508098, 3221508101

	var afd *os.File

	if os.Args[1] == "-" {
		afd = os.Stdin
	} else {
		afd, err = os.OpenFile(os.Args[1], os.O_RDONLY, 0)

		if err != nil {
			log.Fatal("no audio file specified")
		}
	}

	defer afd.Close()

	osfd, err := os.OpenFile(oss_device, os.O_WRONLY, 0)

	if err != nil {
		log.Fatal("no audio file specified")
	}

	defer osfd.Close()

	frequency := 44100

	if len(os.Args) == 3 {
		frequency, _ = strconv.Atoi(os.Args[2])
	}

	blen := 5 * uint32(frequency) * 2 * AFMT_S16_NE

	if os.Args[1] == "-" {
		blen = 2048
	}

	type setfmt struct {
		a uint32
	}

	tmp := setfmt{AFMT_S16_NE}

	syscall.Syscall(syscall.SYS_IOCTL, osfd.Fd(), uintptr(SNDCTL_DSP_SETFMT), uintptr(unsafe.Pointer(&tmp)))

	if tmp.a != AFMT_S16_NE {
		log.Fatal("couldnot set the format")
	}

	tmp = setfmt{2}

	syscall.Syscall(syscall.SYS_IOCTL, osfd.Fd(), uintptr(SNDCTL_DSP_CHANNELS), uintptr(unsafe.Pointer(&tmp)))

	tmp = setfmt{uint32(frequency)}

	syscall.Syscall(syscall.SYS_IOCTL, osfd.Fd(), uintptr(SNDCTL_DSP_SPEED), uintptr(unsafe.Pointer(&tmp)))

	fmt.Printf("Speed set to %v HZ\n", tmp.a)

	/* 5 sec data (stereo) */
	buf := make([]byte, blen)
	sch := make(chan bool)
	flag := 1

	go func() {
		<-sch

		h, m, s, us := 0, 0, 0, 0

		for flag > 0 {
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

		sch <- true
	}()

	defer close(sch)

	var n int

	if n, err = afd.Read(buf); err != nil {
		log.Fatal(err)
	}

	sch <- true

	osfd.Write(buf[:n])

	for {
		n, err = afd.Read(buf)

		if err != nil {
			flag = 0
			break
		}

		osfd.Write(buf[:n])
	}

	<-sch

	fmt.Println()
}
