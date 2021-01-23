# robocar-simulator

MQTT gateway for [robocar simulator](https://github.com/tawnkramer/gym-donkeycar)

## Usage

```
Usage of ./rc-simulator:
  -debug
        Debug logs
  -events-topic-frame string
        Mqtt topic to events gateway frames, use MQTT_TOPIC_FRAME if args not set
  -events-topic-steering string
        Mqtt topic to events gateway steering, use MQTT_TOPIC_STEERING if args not set
  -events-topic-throttle string
        Mqtt topic to events gateway throttle, use MQTT_TOPIC_THROTTLE if args not set
  -mqtt-broker string
        Broker Uri, use MQTT_BROKER env if arg not set (default "tcp://diabolo.local:1883")
  -mqtt-client-id string
        Mqtt client id, use MQTT_CLIENT_ID env if args not set (default "robocar-simulator")
  -mqtt-password string
        Broker Password, MQTT_PASSWORD env if args not set (default "satanas")
  -mqtt-qos int
        Qos to pusblish message, use MQTT_QOS env if arg not set
  -mqtt-retain
        Retain mqtt message, if not set, true if MQTT_RETAIN env variable is set
  -mqtt-username string
        Broker Username, use MQTT_USERNAME env if arg not set (default "satanas")
  -simulator-address string
        Simulator address (default "127.0.0.1:9091")
  -topic-steering-ctrl string
        Mqtt topic to send steering instructions, use MQTT_TOPIC_STEERING_CTRL if args not set
  -topic-throttle-ctrl string
        Mqtt topic to send throttle instructions, use MQTT_TOPIC_THROTTLE_CTRL if args not set

```

## Docker

To build images, run:

```bash 
docker buildx build . --platform linux/arm/7,linux/arm64,linux/amd64 -t cyrilix/robocar-simulator
```
