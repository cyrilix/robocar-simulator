package camera

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/cyrilix/robocar-protobuf/go/events"
	"github.com/cyrilix/robocar-simulator/simulator"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"time"
)

func New(publisher Publisher, addressSimulator string) *Gateway {
	log.Info("run camera camera")

	return &Gateway{
		address:          addressSimulator,
		publisher:        publisher,
	}
}

/* Simulator interface to publish camera frames into mqtt topic */
type Gateway struct {
	cancel           chan interface{}

	address string
	conn    io.ReadCloser

	publisher Publisher
}

func (p *Gateway) Start() error {
	log.Info("connect to simulator")
	p.cancel = make(chan interface{})
	msgChan := make(chan *simulator.SimulatorMsg)

	go p.run(msgChan)

	for {
		select {
		case msg := <-msgChan:
			go p.publishFrame(msg)
		case <-p.cancel:
			return nil
		}
	}
}

func (p *Gateway) Stop() {
	log.Info("close simulator gateway")
	close(p.cancel)

	if err := p.Close(); err != nil {
		log.Printf("unexpected error while simulator connection is closed: %v", err)
	}
}

func (p *Gateway) Close() error {
	if p.conn == nil {
		log.Warnln("no connection to close")
		return nil
	}
	if err := p.conn.Close(); err != nil {
		return fmt.Errorf("unable to close connection to simulator: %v", err)
	}
	return nil
}

func (p *Gateway) run(msgChan chan<- *simulator.SimulatorMsg) {
	err := retry.Do(func() error {
		conn, err := connect(p.address)
		if err != nil {
			return fmt.Errorf("unable to connect to simulator at %v", p.address)
		}
		p.conn = conn
		return nil
	},
		retry.Delay(1*time.Second),
	)
	if err != nil {
		log.Panicf("unable to connect to simulator: %v", err)
	}

	reader := bufio.NewReader(p.conn)

	err = retry.Do(
		func() error { return p.listen(msgChan, reader) },
	)
	if err != nil {
		log.Errorf("unable to connect to server: %v", err)
	}
}

func (p *Gateway) listen(msgChan chan<- *simulator.SimulatorMsg, reader *bufio.Reader) error {
	for {
		rawLine, err := reader.ReadBytes('\n')
		if err == io.EOF {
			log.Info("Connection closed")
			return err
		}
		if err != nil {
			return fmt.Errorf("unable to read response: %v", err)
		}

		var msg simulator.SimulatorMsg
		err = json.Unmarshal(rawLine, &msg)
		if err != nil {
			log.Errorf("unable to unmarshal simulator msg: %v", err)
		}
		if "telemetry" != msg.MsgType {
			continue
		}
		msgChan <- &msg
	}
}

func (p *Gateway) publishFrame(msgSim *simulator.SimulatorMsg) {
	now := time.Now()
	msg := &events.FrameMessage{
		Id: &events.FrameRef{
			Name: "camera",
			Id:   fmt.Sprintf("%d%03d", now.Unix(), now.Nanosecond()/1000/1000),
			CreatedAt: &timestamp.Timestamp{
				Seconds: now.Unix(),
				Nanos:   int32(now.Nanosecond()),
			},
		},
		Frame: msgSim.Image,
	}

	payload, err := proto.Marshal(msg)
	if err != nil {
		log.Errorf("unable to marshal protobuf message: %v", err)
	}
	p.publisher.Publish(&payload)
}

var connect = func(address string) (io.ReadWriteCloser, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to %v", address)
	}
	return conn, nil
}

type Publisher interface {
	Publish(payload *[]byte)
}

func NewMqttPublisher(client mqtt.Client, topic string) Publisher {
	return &MqttPublisher{
		client: client,
		topic: topic,
	}
}

type MqttPublisher struct {
	client mqtt.Client
	topic string
}

func (m *MqttPublisher) Publish(payload *[]byte) {
	token := m.client.Publish(m.topic, 0, false, *payload)
	token.WaitTimeout(10 * time.Millisecond)
	if err := token.Error(); err != nil {
		log.Errorf("unable to publish frame: %v", err)
	}
}
