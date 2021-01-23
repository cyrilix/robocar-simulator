# robocar-simulator

MQTT gateway for [robocar simulator](https://github.com/tawnkramer/gym-donkeycar)

## Docker

To build images, run:

```bash 
docker buildx build . --platform linux/arm/7,linux/arm64,linux/amd64 -t cyrilix/robocar-simulator
```
