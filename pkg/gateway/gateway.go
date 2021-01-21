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

func (p *Gateway) Start() error {
	p.log.Info("connect to simulator")
	p.cancel = make(chan interface{})
	msgChan := make(chan *simulator.TelemetryMsg)

	go p.run(msgChan)

	for {
		select {
		case msg := <-msgChan:
			fr := p.publishFrame(msg)
			go p.publishInputSteering(msg, fr)
			go p.publishInputThrottle(msg, fr)
		case <-p.cancel:
			return nil
		}
	}
}

func (p *Gateway) Stop() {
	p.log.Info("close simulator gateway")
	close(p.cancel)

	if err := p.Close(); err != nil {
		p.log.Warnf("unexpected error while simulator connection is closed: %v", err)
	}
}

func (p *Gateway) Close() error {
	for c := range p.frameSubscribers {
		close(c)
	}
	for c := range p.steeringSubscribers {
		close(c)
	}
	for c := range p.throttleSubscribers {
		close(c)
	}
	if p.conn == nil {
		p.log.Warn("no connection to close")
		return nil
	}
	if err := p.conn.Close(); err != nil {
		return fmt.Errorf("unable to close connection to simulator: %v", err)
	}
	return nil
}

func (p *Gateway) run(msgChan chan<- *simulator.TelemetryMsg) {
	err := p.connect()
	if err != nil {
		p.log.Panicf("unable to connect to simulator: %v", err)
	}

	reader := bufio.NewReader(p.conn)

	err = retry.Do(
		func() error { return p.listen(msgChan, reader) },
	)
	if err != nil {
		p.log.Errorf("unable to connect to server: %v", err)
	}
}

func (p *Gateway) connect() error {
	p.muConn.Lock()
	defer p.muConn.Unlock()

	if p.conn != nil {
		// already connected
		return nil
	}
	err := retry.Do(func() error {
		p.log.Info("connect to simulator")
		conn, err := connect(p.address)
		if err != nil {
			return fmt.Errorf("unable to connect to simulator at %v", p.address)
		}
		p.conn = conn
		p.log.Info("connection success")
		return nil
	},
		retry.Delay(1*time.Second),
	)
	return err
}

func (p *Gateway) listen(msgChan chan<- *simulator.TelemetryMsg, reader *bufio.Reader) error {
	for {
		rawLine, err := reader.ReadBytes('\n')
		if err == io.EOF {
			p.log.Info("Connection closed")
			return err
		}
		if err != nil {
			return fmt.Errorf("unable to read response: %v", err)
		}

		var msg simulator.TelemetryMsg
		err = json.Unmarshal(rawLine, &msg)
		if err != nil {
			p.log.Errorf("unable to unmarshal simulator msg '%v': %v", string(rawLine), err)
		}
		if "telemetry" != msg.MsgType {
			continue
		}
		msgChan <- &msg
	}
}

func (p *Gateway) publishFrame(msgSim *simulator.TelemetryMsg) *events.FrameRef {
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
	for fs := range p.frameSubscribers {
		fs <- msg
	}
	return frameRef
}

func (p *Gateway) publishInputSteering(msgSim *simulator.TelemetryMsg, frameRef *events.FrameRef) {
	steering := &events.SteeringMessage{
		FrameRef:   frameRef,
		Steering:   float32(msgSim.SteeringAngle),
		Confidence: 1.0,
	}

	log.Debugf("events steering '%v'", steering.Steering)
	for ss := range p.steeringSubscribers {
		ss <- steering
	}
}

func (p *Gateway) publishInputThrottle(msgSim *simulator.TelemetryMsg, frameRef *events.FrameRef) {
	msg := &events.ThrottleMessage{
		FrameRef:   frameRef,
		Throttle:   float32(msgSim.Throttle),
		Confidence: 1.0,
	}

	log.Debugf("events throttle '%v'", msg.Throttle)
	for ts := range p.throttleSubscribers {
		ts <- msg
	}
}

func (p *Gateway) SubscribeFrame() <-chan *events.FrameMessage {
	frameChan := make(chan *events.FrameMessage)
	p.frameSubscribers[frameChan] = struct{}{}
	return frameChan
}
func (p *Gateway) SubscribeSteering() <-chan *events.SteeringMessage {
	steeringChan := make(chan *events.SteeringMessage)
	p.steeringSubscribers[steeringChan] = struct{}{}
	return steeringChan
}
func (p *Gateway) SubscribeThrottle() <-chan *events.ThrottleMessage {
	throttleChan := make(chan *events.ThrottleMessage)
	p.throttleSubscribers[throttleChan] = struct{}{}
	return throttleChan
}

var connect = func(address string) (io.ReadWriteCloser, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to %v", address)
	}
	return conn, nil
}

func (p *Gateway) WriteSteering(message *events.SteeringMessage) {
	p.muControl.Lock()
	defer p.muControl.Unlock()
	p.initLastControlMsg()

	p.lastControl.Steering = message.Steering
	p.writeControlCommandToSimulator()
}

func (p *Gateway) writeControlCommandToSimulator() {
	if err := p.connect(); err != nil {
		p.log.Errorf("unable to connect to simulator to send control command: %v", err)
		return
	}
	w := bufio.NewWriter(p.conn)
	content, err := json.Marshal(p.lastControl)
	if err != nil {
		p.log.Errorf("unable to marshall control msg \"%#v\": %v", p.lastControl, err)
		return
	}

	_, err = w.Write(append(content, '\n'))
	if err != nil {
		p.log.Errorf("unable to write control msg \"%#v\" to simulator: %v", p.lastControl, err)
		return
	}
	err = w.Flush()
	if err != nil {
		p.log.Errorf("unable to flush control msg \"%#v\" to simulator: %v", p.lastControl, err)
		return
	}
}

func (p *Gateway) WriteThrottle(message *events.ThrottleMessage) {
	p.muControl.Lock()
	defer p.muControl.Unlock()
	p.initLastControlMsg()

	if message.Throttle > 0 {
		p.lastControl.Throttle = message.Throttle
		p.lastControl.Brake = 0.
	} else {
		p.lastControl.Throttle = 0.
		p.lastControl.Brake = -1 * message.Throttle
	}

	p.writeControlCommandToSimulator()
}

func (p *Gateway) initLastControlMsg() {
	if p.lastControl != nil {
		return
	}
	p.lastControl = &simulator.ControlMsg{
		MsgType:  "control",
		Steering: 0.,
		Throttle: 0.,
		Brake:    0.,
	}
}
