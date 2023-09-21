Creating/Running jukebox
-----------------------
Pre-requisites: libmpg123 libsctp libavformat libavcodec libavutil libasound2

     ex: on debian based systems one can use following command to install above libraries and related header files
   
     `sudo apt-get install -y libmpg123-dev libasound2-dev libavformat-dev libavcodec-dev libavutil-dev`

1) `cd` to gowebjukebox/jukebox

2) `run` go build [jukebox.go | jukebox_new.go]
  
3) running a master node

   at the command prompt run the following command 
        
   [old player] SMASTER=1 AMQP_USER=rabbitmq_user AMQP_PASSWD=rabbitmq_password ./jukebox

   [new player] SMASTER=1 ./jukebox_new

5) running a non master node
        
   [old player]  ./jukebox

   [new player]  ./jukebox_new
          
7) interaction with jukebox, point your browser to `http://ip_address_of_the_running_jukebox_node:3000`

   **ex:** `http://localhost:3000`


Web Jukebox (Play mp3 file / Stream ) -- DOCKER
-----------------------------------------------

Build Image:
  docker build -t gowebjukebox .
  
Run Docker Image:
  docker run -d --name jukebox -p 3000:3000 --device /dev/snd -e PULSE_SERVER=unix:${XDG_RUNTIME_DIR}/pulse/native -v ${XDG_RUNTIME_DIR}/pulse/native:${XDG_RUNTIME_DIR}/pulse/native -v ~/.config/pulse/cookie:/root/.config/pulse/cookie --group-add $(getent group audio | cut -d: -f3) gowebjukebox

Web Interaction:
  point your browser to http://localhost:3000
