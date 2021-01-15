package main

import (
	"flag"
	"github.com/cyrilix/robocar-base/cli"
	"github.com/cyrilix/robocar-simulator/pkg/gateway"
	log "github.com/sirupsen/logrus"
	"os"
)

const DefaultClientId = "robocar-simulator"

func main() {
	var mqttBroker, username, password, clientId, topicFrame string
	var address string
	var debug bool

	mqttQos := cli.InitIntFlag("MQTT_QOS", 0)
	_, mqttRetain := os.LookupEnv("MQTT_RETAIN")

	cli.InitMqttFlags(DefaultClientId, &mqttBroker, &username, &password, &clientId, &mqttQos, &mqttRetain)

	flag.StringVar(&topicFrame, "mqtt-topic-frame", os.Getenv("MQTT_TOPIC"), "Mqtt topic to publish gateway frames, use MQTT_TOPIC_FRAME if args not set")
	flag.StringVar(&address, "simulator-address", "127.0.0.1:9091", "Simulator address")
	flag.BoolVar(&debug, "debug", false, "Debug logs")

	flag.Parse()
	if len(os.Args) <= 1 {
		flag.PrintDefaults()
		os.Exit(1)
	}
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	client, err := cli.Connect(mqttBroker, username, password, clientId)
	if err != nil {
		log.Fatalf("unable to connect to mqtt broker: %v", err)
	}
	defer client.Disconnect(10)

	c := gateway.New(gateway.NewMqttPublisher(client, topicFrame, "", ""), address)
	defer c.Stop()

	cli.HandleExit(c)

	err = c.Start()
	if err != nil {
		log.Fatalf("unable to start service: %v", err)
	}
}
