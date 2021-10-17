package main

import (
	"flag"
	"fmt"
	"github.com/cyrilix/robocar-base/cli"
	events2 "github.com/cyrilix/robocar-protobuf/go/events"
	"github.com/cyrilix/robocar-simulator/pkg/events"
	"github.com/cyrilix/robocar-simulator/pkg/gateway"
	"github.com/cyrilix/robocar-simulator/pkg/simulator"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
	"log"
	"os"
	"strconv"
	"strings"
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

	var carName, carStyle, carColor string
	var carFontSize int
	carStyles := []string{
		string(simulator.CarConfigBodyStyleDonkey),
		string(simulator.CarConfigBodyStyleBare),
		string(simulator.CarConfigBodyStyleCar01),
	}
	flag.StringVar(&carName, "car-name", "simulator-gateway", "Car name to display")
	flag.StringVar(&carStyle, "car-style", string(simulator.CarConfigBodyStyleDonkey), fmt.Sprintf("Car style, only %s", strings.Join(carStyles, ",")))
	flag.StringVar(&carColor, "car-color", "0,0,0", "Color car as rgb value")
	flag.IntVar(&carFontSize, "car-font-size", 0, "Car font size")

	var racerName, racerBio, racerCountry,  racerGuid string
	flag.StringVar(&racerName, "racer-name", "", "")
	flag.StringVar(&racerBio, "racer-bio",      "", "")
	flag.StringVar(&racerCountry, "racer-country",   "", "")
	flag.StringVar(&racerGuid, "racer-guid",     "", "")

	var cameraFov, cameraImgW, cameraImgH, cameraImgD int
	var cameraFishEyeX, cameraFishEyeY float64
	var cameraOffsetX, cameraOffsetY, cameraOffsetZ, cameraRotX, cameraRotY, cameraRotZ float64
	var cameraImgEnc string
	flag.IntVar(&cameraFov, "camera-fov", 90, "")
	flag.Float64Var(&cameraFishEyeX, "camera-fish-eye-x", 0.4, "")
	flag.Float64Var(&cameraFishEyeY, "camera-fish-eye-y", 0.7, "")
	flag.IntVar(&cameraImgW, "camera-img-w", 160, "image width")
	flag.IntVar(&cameraImgH, "camera-img-h", 128, "image height")
	flag.IntVar(&cameraImgD, "camera-img-d", 3, "Image depth")
	flag.StringVar(&cameraImgEnc, "camera-img-enc", string(simulator.CameraImageEncJpeg), "")
	flag.Float64Var(&cameraOffsetX, "camera-offset-x", 0, "moves camera left/right")
	flag.Float64Var(&cameraOffsetY, "camera-offset-y", 1.120395, "moves camera up/down")
	flag.Float64Var(&cameraOffsetZ, "camera-offset-z", 0.5528488, "moves camera forward/back")
	flag.Float64Var(&cameraRotX, "camera-rot-x", 15.0, "rotate the camera around x-axis")
	flag.Float64Var(&cameraRotY, "camera-rot-y", 0.0, "rotate the camera around y-axis")
	flag.Float64Var(&cameraRotZ, "camera-rot-z", 0.0, "rotate the camera around z-axis")

	flag.Parse()
	if len(os.Args) <= 1 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	config := zap.NewDevelopmentConfig()
	if debug {
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	lgr, err := config.Build()
	if err != nil {
		log.Fatalf("unable to init logger: %v", err)
	}
	defer func() {
		if err := lgr.Sync(); err != nil {
			log.Printf("unable to Sync logger: %v\n", err)
		}
	}()
	zap.ReplaceGlobals(lgr)

	client, err := cli.Connect(mqttBroker, username, password, clientId)
	if err != nil {
		zap.S().Fatalf("unable to connect to events broker: %v", err)
	}
	defer client.Disconnect(10)

	bodyColors := strings.Split(carColor, ",")
	carConfig := simulator.CarConfigMsg{
		MsgType:   simulator.MsgTypeCarConfig,
		BodyStyle: simulator.CarStyle(carStyle),
		BodyR:     bodyColors[0],
		BodyG:     bodyColors[1],
		BodyB:     bodyColors[2],
		CarName:   carName,
		FontSize:  strconv.Itoa(carFontSize),
	}
	racer := simulator.RacerBioMsg{
		MsgType:   simulator.MsgTypeRacerInfo,
		RacerName: racerName,
		CarName:   carName,
		Bio:       racerBio,
		Country:   racerCountry,
		Guid:      racerGuid,
	}
	camera := simulator.CamConfigMsg{
		MsgType:  simulator.MsgTypeCameraConfig,
		Fov:      strconv.Itoa(cameraFov),
		FishEyeX: fmt.Sprintf("%.2f", cameraFishEyeX),
		FishEyeY: fmt.Sprintf("%.2f",cameraFishEyeY),
		ImgW:     strconv.Itoa(cameraImgW),
		ImgH:     strconv.Itoa(cameraImgH),
		ImgD:     strconv.Itoa(cameraImgD),
		ImgEnc:   simulator.CameraImageEnc(cameraImgEnc),
		OffsetX:  fmt.Sprintf("%.2f",cameraOffsetX),
		OffsetY: fmt.Sprintf("%.2f", cameraOffsetY),
		OffsetZ: fmt.Sprintf("%.2f", cameraOffsetZ),
		RotX:    fmt.Sprintf("%.2f", cameraRotX),
		RotY:    fmt.Sprintf("%.2f", cameraRotY),
		RotZ:    fmt.Sprintf("%.2f", cameraRotZ),
	}
	gtw, err := gateway.New(address, &carConfig, &racer, &camera)
	if err != nil {
		zap.S().Fatalf("unable to init gateway: %v", err)
	}
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
		zap.S().Info("configure mqtt route on steering command")
		client.Subscribe(topicCtrlSteering, byte(mqttQos), func(client mqtt.Client, message mqtt.Message) {
			onSteeringCommand(gtw, message)
		})
	}
	if topicCtrlThrottle != "" {
		zap.S().Info("configure mqtt route on throttle command")
		client.Subscribe(topicCtrlThrottle, byte(mqttQos), func(client mqtt.Client, message mqtt.Message) {
			onThrottleCommand(gtw, message)
		})
	}

	err = gtw.Start()
	if err != nil {
		zap.S().Fatalf("unable to start service: %v", err)
	}

}

func onSteeringCommand(c *gateway.Gateway, message mqtt.Message) {
	var steeringMsg events2.SteeringMessage
	err := proto.Unmarshal(message.Payload(), &steeringMsg)
	if err != nil {
		zap.S().Errorf("unable to unmarshal steering msg: %v", err)
		return
	}
	c.WriteSteering(&steeringMsg)
}

func onThrottleCommand(c *gateway.Gateway, message mqtt.Message) {
	var throttleMsg events2.ThrottleMessage
	err := proto.Unmarshal(message.Payload(), &throttleMsg)
	if err != nil {
		zap.S().Errorf("unable to unmarshal throttle msg: %v", err)
		return
	}
	c.WriteThrottle(&throttleMsg)
}
