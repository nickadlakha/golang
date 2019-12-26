Creating jukebox binary
-----------------------
i) copy gowebjukebox folder to ~/go/src

ii) go to ~/go/src/gowebjukebox/jukebox

iii) execute go build jukebox.go

iv) running jukebox, go to ~/go/src/gowebjukebox/jukebox

a) running a master node

    at the command prompt run the following command 
        
        SMASTER=1 AMQP_USER=rabbitmq_user AMQP_PASSWD=rabbitmq_password ./jukebox
b) running a non master node
        
        run the following command
          
          ./jukebox
          
 v) interaction with jukebox, point your browser to http://ip_address_of_the_running_jukebox_node:3000


Web Jukebox (Play mp3 file / Stream ) -- DOCKER
-----------------------------------------------

Build Image:
  docker build -t gowebjukebox .
  
Run Docker Image:
  docker run -d --name jukebox -p 3000:3000 --device /dev/snd -e PULSE_SERVER=unix:${XDG_RUNTIME_DIR}/pulse/native -v ${XDG_RUNTIME_DIR}/pulse/native:${XDG_RUNTIME_DIR}/pulse/native -v ~/.config/pulse/cookie:/root/.config/pulse/cookie --group-add $(getent group audio | cut -d: -f3) gowebjukebox

Web Interaction:
  point your browser to http://localhost:3000
