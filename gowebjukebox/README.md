Creating/Running jukebox
-----------------------
i)  `cd` to gowebjukebox/jukebox

ii) `run` go build [jukebox.go | jukebox_new.go]
  
iii) running a master node

     at the command prompt run the following command 
        
     [old player] SMASTER=1 AMQP_USER=rabbitmq_user AMQP_PASSWD=rabbitmq_password ./jukebox
     [new player] SMASTER=1 ./jukebox_new

iv)  running a non master node
        
      [old player]  ./jukebox
      [new player]  ./jukebox_new
          
 v) interaction with jukebox, point your browser to http://ip_address_of_the_running_jukebox_node:3000


Web Jukebox (Play mp3 file / Stream ) -- DOCKER
-----------------------------------------------

Build Image:
  docker build -t gowebjukebox .
  
Run Docker Image:
  docker run -d --name jukebox -p 3000:3000 --device /dev/snd -e PULSE_SERVER=unix:${XDG_RUNTIME_DIR}/pulse/native -v ${XDG_RUNTIME_DIR}/pulse/native:${XDG_RUNTIME_DIR}/pulse/native -v ~/.config/pulse/cookie:/root/.config/pulse/cookie --group-add $(getent group audio | cut -d: -f3) gowebjukebox

Web Interaction:
  point your browser to http://localhost:3000
