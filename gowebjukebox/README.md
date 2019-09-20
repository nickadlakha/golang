Web Jukebox (Play mp3 file / Stream )

Build Image:
  docker build -t gowebjukebox .
  
Run Docker Image:
  docker run -d --name jukebox -p 3000:3000 --device /dev/snd -e PULSE_SERVER=unix:${XDG_RUNTIME_DIR}/pulse/native -v ${XDG_RUNTIME_DIR}/pulse/native:${XDG_RUNTIME_DIR}/pulse/native -v ~/.config/pulse/cookie:/root/.config/pulse/cookie --group-add $(getent group audio | cut -d: -f3) gowebjukebox

Web Interaction:
  point your browser to http://localhost:3000
