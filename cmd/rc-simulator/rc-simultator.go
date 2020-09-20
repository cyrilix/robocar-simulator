package main

import (
	"flag"
	"github.com/cyrilix/robocar-base/cli"
	"github.com/cyrilix/robocar-simulator/camera"
	"log"
	"os"
)

const DefaultClientId = "robocar-camera"

func main() {
	var mqttBroker, username, password, clientId, topicBase string
	var address string

	mqttQos := cli.InitIntFlag("MQTT_QOS", 0)
	_, mqttRetain := os.LookupEnv("MQTT_RETAIN")

	cli.InitMqttFlags(DefaultClientId, &mqttBroker, &username, &password, &clientId, &mqttQos, &mqttRetain)

	flag.StringVar(&topicBase, "mqtt-topic", os.Getenv("MQTT_TOPIC"), "Mqtt topic to publish camera frames, use MQTT_TOPIC if args not set")
	flag.StringVar(&address, "simulator-address", "127.0.0.1:9091", "Simulator address")

	flag.Parse()
	if len(os.Args) <= 1 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	client, err := cli.Connect(mqttBroker, username, password, clientId)
	if err != nil {
		log.Fatalf("unable to connect to mqtt broker: %v", err)
	}
	defer client.Disconnect(10)


	c := camera.New(camera.NewMqttPublisher(client, topicBase), address)
	defer c.Stop()

	cli.HandleExit(c)

	err = c.Start()
	if err != nil {
		log.Fatalf("unable to start service: %v", err)
	}
}
