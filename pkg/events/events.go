package events

import (
	"fmt"
	"github.com/cyrilix/robocar-simulator/pkg/gateway"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	"sync"
	"time"
	"go.uber.org/zap"
)

func NewMsgPublisher(srcEvents gateway.SimulatorSource, p Publisher, topicFrame, topicSteering, topicThrottle string) *MsgPublisher {
	return &MsgPublisher{
		p:             p,
		topicFrame:    topicFrame,
		topicSteering: topicSteering,
		topicThrottle: topicThrottle,
		srcEvents:     srcEvents,
		muCancel:      sync.Mutex{},
		cancel:        nil,
	}
}

type MsgPublisher struct {
	p             Publisher
	topicFrame    string
	topicSteering string
	topicThrottle string

	srcEvents gateway.SimulatorSource

	muCancel sync.Mutex
	cancel   chan interface{}
}

func (m *MsgPublisher) Start() {
	m.muCancel.Lock()
	defer m.muCancel.Unlock()

	m.cancel = make(chan interface{})

	if m.topicThrottle != "" {
		go m.listenThrottle()
	}
	if m.topicSteering != "" {
		go m.listenSteering()
	}
	if m.topicFrame != "" {
		go m.listenFrame()
	}
}

func (m *MsgPublisher) Stop() {
	m.muCancel.Lock()
	defer m.muCancel.Unlock()
	close(m.cancel)
	m.cancel = nil
}

func (m *MsgPublisher) listenThrottle() {
	logr := zap.S().With("msg_type", "throttleChan")
	msgChan := m.srcEvents.SubscribeThrottle()
	for {
		select {
		case <-m.cancel:
			logr.Debug("exit listen throttleChan loop")
			return
		case msg := <-msgChan:

			payload, err := proto.Marshal(msg)
			if err != nil {
				logr.Errorf("unable to marshal protobuf message: %v", err)
			} else {

				err = m.p.Publish(m.topicThrottle, payload)
				if err != nil {
					logr.Errorf("unable to publish events message: %v", err)
				}
			}
		}
	}
}

func (m *MsgPublisher) listenSteering() {
	logr := zap.S().With("msg_type", "steeringChan")
	msgChan := m.srcEvents.SubscribeSteering()
	for {
		select {

		case msg := <-msgChan:
			if m.topicSteering == "" {
				return
			}

			payload, err := proto.Marshal(msg)
			if err != nil {
				logr.Errorf("unable to marshal protobuf message: %v", err)
			} else {
				err = m.p.Publish(m.topicSteering, payload)
				if err != nil {
					logr.Errorf("unable to publish events message: %v", err)
				}
			}
		}
	}
}

func (m *MsgPublisher) listenFrame() {
	logr := zap.S().With("msg_type", "frame")
	msgChan := m.srcEvents.SubscribeFrame()
	for {
		msg := <-msgChan
		if msg == nil {
			// channel closed
			break
		}
		logr.Debugf("new frame %v", msg.Id)
		if m.topicFrame == "" {
			return
		}

		payload, err := proto.Marshal(msg)
		if err != nil {
			logr.Errorf("unable to marshal protobuf message: %v", err)
			continue
		}
		err = m.p.Publish(m.topicFrame, payload)
		if err != nil {
			logr.Errorf("unable to publish events message: %v", err)
		}
	}
}

type Publisher interface {
	Publish(topic string, payload []byte) error
}

func NewMqttPublisher(client mqtt.Client) *MqttPublisher {
	return &MqttPublisher{client: client}
}

type MqttPublisher struct {
	client mqtt.Client
}

func (m *MqttPublisher) Publish(topic string, payload []byte) error {
	token := m.client.Publish(topic, 0, false, payload)
	token.WaitTimeout(10 * time.Millisecond)
	if err := token.Error(); err != nil {
		return fmt.Errorf("unable to events to topic: %v", err)
	}
	return nil
}
