package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/cyrilix/robocar-protobuf/go/events"
	"github.com/cyrilix/robocar-simulator/pkg/simulator"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.uber.org/zap"
	"io"
	"net"
	"sync"
	"time"
)

type SimulatorSource interface {
	FrameSource
	SteeringSource
	ThrottleSource
}

type FrameSource interface {
	SubscribeFrame() <-chan *events.FrameMessage
}
type SteeringSource interface {
	SubscribeSteering() <-chan *events.SteeringMessage
}
type ThrottleSource interface {
	SubscribeThrottle() <-chan *events.ThrottleMessage
}

func New(addressSimulator string, car *simulator.CarConfigMsg, racer *simulator.RacerBioMsg, camera *simulator.CamConfigMsg) *Gateway {
	l := zap.S().With("simulator", addressSimulator)
	l.Info("run gateway from simulator")

	return &Gateway{
		address:              addressSimulator,
		log:                  l,
		frameSubscribers:     make(map[chan<- *events.FrameMessage]interface{}),
		steeringSubscribers:  make(map[chan<- *events.SteeringMessage]interface{}),
		throttleSubscribers:  make(map[chan<- *events.ThrottleMessage]interface{}),
		telemetrySubscribers: make(map[chan *simulator.TelemetryMsg]interface{}),
		carSubscribers:       make(map[chan *simulator.Msg]interface{}),
		racerSubscribers:     make(map[chan *simulator.Msg]interface{}),
		cameraSubscribers:    make(map[chan *simulator.Msg]interface{}),
		carConfig:            car,
		racer:                racer,
		cameraConfig:         camera,
	}
}

// Gateway is Simulator interface to events gateway frames into events topicFrame
type Gateway struct {
	cancel chan interface{}

	address string

	muCommand sync.Mutex
	muConn    sync.Mutex
	conn      io.ReadWriteCloser

	muControl   sync.Mutex
	lastControl *simulator.ControlMsg

	log *zap.SugaredLogger

	frameSubscribers    map[chan<- *events.FrameMessage]interface{}
	steeringSubscribers map[chan<- *events.SteeringMessage]interface{}
	throttleSubscribers map[chan<- *events.ThrottleMessage]interface{}

	telemetrySubscribers map[chan *simulator.TelemetryMsg]interface{}
	carSubscribers       map[chan *simulator.Msg]interface{}
	racerSubscribers     map[chan *simulator.Msg]interface{}
	cameraSubscribers    map[chan *simulator.Msg]interface{}

	carConfig    *simulator.CarConfigMsg
	racer        *simulator.RacerBioMsg
	cameraConfig *simulator.CamConfigMsg
}

func (g *Gateway) Start() error {
	g.log.Info("connect to simulator")
	g.cancel = make(chan interface{})
	msgChan := g.subscribeTelemetryEvents()

	go g.run()

	err := g.writeRacerConfig()
	if err != nil {
		return fmt.Errorf("unable to configure racer to server: %v", err)
	}
	err = g.writeCarConfig()
	if err != nil {
		return fmt.Errorf("unable to configure car to server: %v", err)
	}
	err = g.writeCameraConfig()
	if err != nil {
		return fmt.Errorf("unable to configure camera to server: %v", err)
	}

	for {
		select {
		case msg := <-msgChan:
			fr := g.publishFrame(msg)
			go g.publishInputSteering(msg, fr)
			go g.publishInputThrottle(msg, fr)
		case <-g.cancel:
			return nil
		}
	}
}

func (g *Gateway) Stop() {
	g.log.Info("close simulator gateway")
	close(g.cancel)

	if err := g.Close(); err != nil {
		g.log.Warnf("unexpected error while simulator connection is closed: %v", err)
	}
}

func (g *Gateway) Close() error {
	for c := range g.frameSubscribers {
		close(c)
	}
	for c := range g.steeringSubscribers {
		close(c)
	}
	for c := range g.throttleSubscribers {
		close(c)
	}
	if g.conn == nil {
		g.log.Warn("no connection to close")
		return nil
	}
	if err := g.conn.Close(); err != nil {
		return fmt.Errorf("unable to close connection to simulator: %v", err)
	}
	return nil
}

func (g *Gateway) run() {
	err := g.connect()
	if err != nil {
		g.log.Panicf("unable to connect to simulator: %v", err)
	}

	err = retry.Do(
		func() error { return g.listen() },
	)
	if err != nil {
		g.log.Errorf("unable to connect to server: %v", err)
	}
}

func (g *Gateway) connect() error {
	g.muConn.Lock()
	defer g.muConn.Unlock()

	if g.conn != nil {
		// already connected
		return nil
	}
	err := retry.Do(func() error {
		g.log.Info("connect to simulator")
		conn, err := connect(g.address)
		if err != nil {
			return fmt.Errorf("unable to connect to simulator at %v", g.address)
		}
		g.conn = conn
		g.log.Info("connection success")
		return nil
	},
		retry.Delay(1*time.Second),
	)
	return err
}

func (g *Gateway) listen() error {
	reader := bufio.NewReader(g.conn)

	for {
		rawLine, err := reader.ReadBytes('\n')
		if err == io.EOF {
			g.log.Info("Connection closed")
			return err
		}
		if err != nil {
			return fmt.Errorf("unable to read response: %v", err)
		}

		var msg simulator.Msg
		err = json.Unmarshal(rawLine, &msg)
		if err != nil {
			g.log.Errorf("unable to unmarshal simulator msg '%v': %v", string(rawLine), err)
		}

		switch msg.MsgType {
		case simulator.MsgTypeTelemetry:
			g.broadcastTelemetryMsg(rawLine)
		case simulator.MsgTypeCarLoaded:
			g.broadcastCarMsg(rawLine)
		case simulator.MsgTypeRacerInfo:
			g.broadcastRacerMsg(rawLine)
		default:
			g.log.Warnf("unmanaged simulator message: %v", string(rawLine))
		}
	}
}

func (g *Gateway) broadcastTelemetryMsg(rawLine []byte) {
	for c := range g.telemetrySubscribers {

		var tMsg simulator.TelemetryMsg
		err := json.Unmarshal(rawLine, &tMsg)
		if err != nil {
			g.log.Errorf("unable to unmarshal telemetry simulator msg '%v': %v", string(rawLine), err)
		}
		c <- &tMsg
	}
}

func (g *Gateway) broadcastCarMsg(rawLine []byte) {
	for c := range g.carSubscribers {
		var tMsg simulator.Msg
		err := json.Unmarshal(rawLine, &tMsg)
		if err != nil {
			g.log.Errorf("unable to unmarshal car simulator msg '%v': %v", string(rawLine), err)
		}
		c <- &tMsg
	}
}

func (g *Gateway) broadcastRacerMsg(rawLine []byte) {
	for c := range g.racerSubscribers {
		var tMsg simulator.Msg
		err := json.Unmarshal(rawLine, &tMsg)
		if err != nil {
			g.log.Errorf("unable to unmarshal racer simulator msg '%v': %v", string(rawLine), err)
		}
		c <- &tMsg
	}
}
func (g *Gateway) broadcastCameraMsg(rawLine []byte) {
	for c := range g.cameraSubscribers {
		var tMsg simulator.Msg
		err := json.Unmarshal(rawLine, &tMsg)
		if err != nil {
			g.log.Errorf("unable to unmarshal camera simulator msg '%v': %v", string(rawLine), err)
		}
		c <- &tMsg
	}
}

func (g *Gateway) publishFrame(msgSim *simulator.TelemetryMsg) *events.FrameRef {
	now := time.Now()
	frameRef := &events.FrameRef{
		Name: "gateway",
		Id:   fmt.Sprintf("%d%03d", now.Unix(), now.Nanosecond()/1000/1000),
		CreatedAt: &timestamp.Timestamp{
			Seconds: now.Unix(),
			Nanos:   int32(now.Nanosecond()),
		},
	}
	msg := &events.FrameMessage{
		Id:    frameRef,
		Frame: msgSim.Image,
	}

	g.log.Debugf("events frame '%v/%v'", msg.Id.Name, msg.Id.Id)

	for fs := range g.frameSubscribers {
		fs <- msg
	}
	return frameRef
}

func (g *Gateway) publishInputSteering(msgSim *simulator.TelemetryMsg, frameRef *events.FrameRef) {
	steering := &events.SteeringMessage{
		FrameRef:   frameRef,
		Steering:   float32(msgSim.SteeringAngle),
		Confidence: 1.0,
	}

	g.log.Debugf("events steering '%v'", steering.Steering)
	for ss := range g.steeringSubscribers {
		ss <- steering
	}
}

func (g *Gateway) publishInputThrottle(msgSim *simulator.TelemetryMsg, frameRef *events.FrameRef) {
	msg := &events.ThrottleMessage{
		FrameRef:   frameRef,
		Throttle:   float32(msgSim.Throttle),
		Confidence: 1.0,
	}

	g.log.Debugf("events throttle '%v'", msg.Throttle)
	for ts := range g.throttleSubscribers {
		ts <- msg
	}
}

func (g *Gateway) SubscribeFrame() <-chan *events.FrameMessage {
	frameChan := make(chan *events.FrameMessage)
	g.frameSubscribers[frameChan] = struct{}{}
	return frameChan
}
func (g *Gateway) SubscribeSteering() <-chan *events.SteeringMessage {
	steeringChan := make(chan *events.SteeringMessage)
	g.steeringSubscribers[steeringChan] = struct{}{}
	return steeringChan
}
func (g *Gateway) SubscribeThrottle() <-chan *events.ThrottleMessage {
	throttleChan := make(chan *events.ThrottleMessage)
	g.throttleSubscribers[throttleChan] = struct{}{}
	return throttleChan
}

func (g *Gateway) subscribeTelemetryEvents() chan *simulator.TelemetryMsg {
	telemetryChan := make(chan *simulator.TelemetryMsg)
	g.telemetrySubscribers[telemetryChan] = struct{}{}
	return telemetryChan
}
func (g *Gateway) unsubscribeTelemetryEvents(telemetryChan chan *simulator.TelemetryMsg) {
	delete(g.telemetrySubscribers, telemetryChan)
	close(telemetryChan)
}
func (g *Gateway) subscribeCarEvents() chan *simulator.Msg {
	carChan := make(chan *simulator.Msg)
	g.carSubscribers[carChan] = struct{}{}
	return carChan
}
func (g *Gateway) unsubscribeCarEvents(carChan chan *simulator.Msg) {
	delete(g.carSubscribers, carChan)
	close(carChan)
}

func (g *Gateway) subscribeRacerEvents() chan *simulator.Msg {
	racerChan := make(chan *simulator.Msg)
	g.racerSubscribers[racerChan] = struct{}{}
	return racerChan
}

func (g *Gateway) unsubscribeRacerEvents(racerChan chan *simulator.Msg) {
	delete(g.racerSubscribers, racerChan)
	close(racerChan)
}

func (g *Gateway) subscribeCameraEvents() chan *simulator.Msg {
	cameraChan := make(chan *simulator.Msg)
	g.cameraSubscribers[cameraChan] = struct{}{}
	return cameraChan
}

func (g *Gateway) unsubscribeCameraEvents(cameraChan chan *simulator.Msg) {
	delete(g.cameraSubscribers, cameraChan)
	close(cameraChan)
}

var connect = func(address string) (io.ReadWriteCloser, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to %v", address)
	}
	return conn, nil
}

func (g *Gateway) WriteSteering(message *events.SteeringMessage) {
	g.muControl.Lock()
	defer g.muControl.Unlock()
	g.initLastControlMsg()

	g.lastControl.Steering = fmt.Sprintf("%.2f", message.Steering)
	g.writeControlCommandToSimulator()
}

func (g *Gateway) writeControlCommandToSimulator() {
	content, err := json.Marshal(g.lastControl)
	if err != nil {
		g.log.Errorf("unable to marshall control msg \"%#v\": %v", g.lastControl, err)
		return
	}

	err = g.writeCommand(content)
	if err != nil {
		g.log.Errorf("unable to send control command to simulator: %v", err)
	}
}

func (g *Gateway) writeCommand(content []byte) error {
	g.muCommand.Lock()
	defer g.muCommand.Unlock()

	if err := g.connect(); err != nil {
		g.log.Errorf("unable to connect to simulator to send control command: %v", err)
		return nil
	}
	g.log.Debugf("write command to simulator: %v", string(content))
	w := bufio.NewWriter(g.conn)

	_, err := w.Write(append(content, '\n'))
	if err != nil {
		return fmt.Errorf("unable to write control msg \"%#v\" to simulator: %v", g.lastControl, err)
	}
	err = w.Flush()
	if err != nil {
		return fmt.Errorf("unable to flush control msg \"%#v\" to simulator: %v", g.lastControl, err)
	}
	return nil
}

func (g *Gateway) WriteThrottle(message *events.ThrottleMessage) {
	g.muControl.Lock()
	defer g.muControl.Unlock()
	g.initLastControlMsg()

	if message.Throttle > 0 {
		g.lastControl.Throttle = fmt.Sprintf("%.2f", message.Throttle)
		g.lastControl.Brake = "0.0"
	} else {
		g.lastControl.Throttle = "0.0"
		g.lastControl.Brake = fmt.Sprintf("%.2f", -1*message.Throttle)
	}

	g.writeControlCommandToSimulator()
}

func (g *Gateway) initLastControlMsg() {
	if g.lastControl != nil {
		return
	}
	g.lastControl = &simulator.ControlMsg{
		MsgType:  simulator.MsgTypeControl,
		Steering: "0.0",
		Throttle: "0.0",
		Brake:    "0.0",
	}
}

func (g *Gateway) writeCarConfig() error {
	carChan := g.subscribeCarEvents()
	defer g.unsubscribeCarEvents(carChan)

	g.log.Info("Send car configuration")

	content, err := json.Marshal(g.carConfig)
	if err != nil {
		return fmt.Errorf("unable to marshall car config msg \"%#v\": %v", g.lastControl, err)
	}

	err = g.writeCommand(content)
	if err != nil {
		return fmt.Errorf("unable to send car config to simulator: %v", err)
	}

	msg := <-carChan
	g.log.Infof("Car loaded: %v", msg)
	time.Sleep(250 * time.Millisecond)
	return nil
}

func (g *Gateway) writeRacerConfig() error {
	racerChan := g.subscribeRacerEvents()
	defer g.unsubscribeRacerEvents(racerChan)

	g.log.Info("Send racer configuration")

	content, err := json.Marshal(g.racer)
	if err != nil {
		return fmt.Errorf("unable to marshall racer config msg \"%#v\": %v", g.lastControl, err)
	}

	err = g.writeCommand(content)
	if err != nil {
		return fmt.Errorf("unable to send racer config to simulator: %v", err)
	}

	select {
	case msg := <-racerChan:
		g.log.Infof("Racer loaded: %v", msg)
	case <-time.Tick(250 * time.Millisecond):
	}
	return nil
}

func (g *Gateway) writeCameraConfig() error {
	cameraChan := g.subscribeCameraEvents()
	defer g.unsubscribeCameraEvents(cameraChan)

	g.log.Info("Send camera configuration")

	content, err := json.Marshal(g.cameraConfig)
	if err != nil {
		return fmt.Errorf("unable to marshall camera config msg \"%#v\": %v", g.lastControl, err)
	}

	err = g.writeCommand(content)
	if err != nil {
		return fmt.Errorf("unable to send camera config to simulator: %v", err)
	}

	select {
	case msg := <-cameraChan:
		g.log.Infof("Camera configured: %v", msg)
	case <-time.Tick(250 * time.Millisecond):
	}
	return nil
}
