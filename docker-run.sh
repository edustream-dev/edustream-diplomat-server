apt update -y
apt install -y docker.io
docker build -t edustream-diplomat-server .
docker run --publish 443:443 --detach -it \
    --restart always \
    --name run-edustream-diplomat-server edustream-diplomat-server
