package main

import (
	"flag"
	"github.com/cyrilix/robocar-base/cli"
	events2 "github.com/cyrilix/robocar-protobuf/go/events"
	"github.com/cyrilix/robocar-simulator/pkg/events"
	"github.com/cyrilix/robocar-simulator/pkg/gateway"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"os"
)

const DefaultClientId = "robocar-simulator"

func main() {
	var mqttBroker, username, password, clientId, topicFrame, topicSteering, topicThrottle string
	var topicCtrlSteering, topicCtrlThrottle string
	var address string
	var debug bool

	mqttQos := cli.InitIntFlag("MQTT_QOS", 0)
	_, mqttRetain := os.LookupEnv("MQTT_RETAIN")

	cli.InitMqttFlags(DefaultClientId, &mqttBroker, &username, &password, &clientId, &mqttQos, &mqttRetain)

	flag.StringVar(&topicFrame, "events-topic-camera", os.Getenv("MQTT_TOPIC_CAMERA"), "Mqtt topic to events gateway frames, use MQTT_TOPIC_CAMERA if args not set")
	flag.StringVar(&topicSteering, "events-topic-steering", os.Getenv("MQTT_TOPIC_STEERING"), "Mqtt topic to events gateway steering, use MQTT_TOPIC_STEERING if args not set")
	flag.StringVar(&topicThrottle, "events-topic-throttle", os.Getenv("MQTT_TOPIC_THROTTLE"), "Mqtt topic to events gateway throttle, use MQTT_TOPIC_THROTTLE if args not set")
	flag.StringVar(&topicCtrlSteering, "topic-steering-ctrl", os.Getenv("MQTT_TOPIC_STEERING_CTRL"), "Mqtt topic to send steering instructions, use MQTT_TOPIC_STEERING_CTRL if args not set")
	flag.StringVar(&topicCtrlThrottle, "topic-throttle-ctrl", os.Getenv("MQTT_TOPIC_THROTTLE_CTRL"), "Mqtt topic to send throttle instructions, use MQTT_TOPIC_THROTTLE_CTRL if args not set")
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
		log.Fatalf("unable to connect to events broker: %v", err)
	}
	defer client.Disconnect(10)

	gtw := gateway.New(address)
	defer gtw.Stop()


	msgPub := events.NewMsgPublisher(
		gtw,
		events.NewMqttPublisher(client),
		topicFrame,
		topicSteering,
		topicThrottle,
	)
	defer msgPub.Stop()
	msgPub.Start()

	cli.HandleExit(gtw)

	if topicCtrlSteering != "" {
		log.Infof("configure mqtt route on steering command")
		client.Subscribe(topicCtrlSteering, byte(mqttQos), func(client mqtt.Client, message mqtt.Message) {
			onSteeringCommand(gtw, message)
		})
	}
	if topicCtrlThrottle != "" {
		log.Infof("configure mqtt route on throttle command")
		client.Subscribe(topicCtrlThrottle, byte(mqttQos), func(client mqtt.Client, message mqtt.Message) {
			onThrottleCommand(gtw, message)
		})
	}

	err = gtw.Start()
	if err != nil {
		log.Fatalf("unable to start service: %v", err)
	}

}

func onSteeringCommand(c *gateway.Gateway, message mqtt.Message) {
	var steeringMsg events2.SteeringMessage
	err := proto.Unmarshal(message.Payload(), &steeringMsg)
	if err != nil {
		log.Errorf("unable to unmarshal steering msg: %v", err)
		return
	}
	c.WriteSteering(&steeringMsg)
}

func onThrottleCommand(c *gateway.Gateway, message mqtt.Message) {
	var throttleMsg events2.ThrottleMessage
	err := proto.Unmarshal(message.Payload(), &throttleMsg)
	if err != nil {
		log.Errorf("unable to unmarshal throttle msg: %v", err)
		return
	}
	c.WriteThrottle(&throttleMsg)
}
