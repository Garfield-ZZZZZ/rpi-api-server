tag="garfieldz.azurecr.io/rpi-api-server:`date +%F`_`git rev-parse HEAD | head -c 7`"
docker build -t $tag .