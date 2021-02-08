package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/cyrilix/robocar-protobuf/go/events"
	"github.com/cyrilix/robocar-simulator/pkg/simulator"
	"github.com/golang/protobuf/ptypes/timestamp"
	log "github.com/sirupsen/logrus"
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

func New(addressSimulator string) *Gateway {
	l := log.WithField("simulator", addressSimulator)
	l.Info("run gateway from simulator")

	return &Gateway{
		address:             addressSimulator,
		log:                 l,
		frameSubscribers:    make(map[chan<- *events.FrameMessage]interface{}),
		steeringSubscribers: make(map[chan<- *events.SteeringMessage]interface{}),
		throttleSubscribers: make(map[chan<- *events.ThrottleMessage]interface{}),
	}
}

/* Simulator interface to events gateway frames into events topicFrame */
type Gateway struct {
	cancel chan interface{}

	address string
	muConn  sync.Mutex
	conn    io.ReadWriteCloser

	muControl   sync.Mutex
	lastControl *simulator.ControlMsg

	log *log.Entry

	frameSubscribers    map[chan<- *events.FrameMessage]interface{}
	steeringSubscribers map[chan<- *events.SteeringMessage]interface{}
	throttleSubscribers map[chan<- *events.ThrottleMessage]interface{}
}

func (g *Gateway) Start() error {
	g.log.Info("connect to simulator")
	g.cancel = make(chan interface{})
	msgChan := make(chan *simulator.TelemetryMsg)

	go g.run(msgChan)

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

func (g *Gateway) run(msgChan chan<- *simulator.TelemetryMsg) {
	err := g.connect()
	if err != nil {
		g.log.Panicf("unable to connect to simulator: %v", err)
	}

	reader := bufio.NewReader(g.conn)

	err = retry.Do(
		func() error { return g.listen(msgChan, reader) },
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

func (g *Gateway) listen(msgChan chan<- *simulator.TelemetryMsg, reader *bufio.Reader) error {
	for {
		rawLine, err := reader.ReadBytes('\n')
		if err == io.EOF {
			g.log.Info("Connection closed")
			return err
		}
		if err != nil {
			return fmt.Errorf("unable to read response: %v", err)
		}

		var msg simulator.TelemetryMsg
		err = json.Unmarshal(rawLine, &msg)
		if err != nil {
			g.log.Errorf("unable to unmarshal simulator msg '%v': %v", string(rawLine), err)
		}
		if "telemetry" != msg.MsgType {
			continue
		}
		msgChan <- &msg
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

	log.Debugf("events frame '%v/%v'", msg.Id.Name, msg.Id.Id)
	log.Infof("publish frame to %v receiver", len(g.frameSubscribers))

	for fs := range g.frameSubscribers {
		log.Info("publish frame")
		fs <- msg
		log.Info("frame published")
	}
	return frameRef
}

func (g *Gateway) publishInputSteering(msgSim *simulator.TelemetryMsg, frameRef *events.FrameRef) {
	steering := &events.SteeringMessage{
		FrameRef:   frameRef,
		Steering:   float32(msgSim.SteeringAngle),
		Confidence: 1.0,
	}

	log.Debugf("events steering '%v'", steering.Steering)
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

	log.Debugf("events throttle '%v'", msg.Throttle)
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
	if err := g.connect(); err != nil {
		g.log.Errorf("unable to connect to simulator to send control command: %v", err)
		return
	}
	log.Debugf("write command to simulator: %v", g.lastControl)
	w := bufio.NewWriter(g.conn)
	content, err := json.Marshal(g.lastControl)
	if err != nil {
		g.log.Errorf("unable to marshall control msg \"%#v\": %v", g.lastControl, err)
		return
	}

	_, err = w.Write(append(content, '\n'))
	if err != nil {
		g.log.Errorf("unable to write control msg \"%#v\" to simulator: %v", g.lastControl, err)
		return
	}
	err = w.Flush()
	if err != nil {
		g.log.Errorf("unable to flush control msg \"%#v\" to simulator: %v", g.lastControl, err)
		return
	}
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
		g.lastControl.Brake = fmt.Sprintf("%.2f", -1 * message.Throttle)
	}

	g.writeControlCommandToSimulator()
}

func (g *Gateway) initLastControlMsg() {
	if g.lastControl != nil {
		return
	}
	g.lastControl = &simulator.ControlMsg{
		MsgType:  "control",
		Steering: "0.0",
		Throttle: "0.0",
		Brake:    "0.0",
	}
}
