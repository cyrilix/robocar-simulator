package events

import (
	"fmt"
	"github.com/cyrilix/robocar-protobuf/go/events"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"testing"
	"time"
)

func NewMockSimulator() *SrcEventsMock {
	return &SrcEventsMock{
		frameChan:    make(chan *events.FrameMessage),
		steeringChan: make(chan *events.SteeringMessage),
		throttleChan: make(chan *events.ThrottleMessage),
	}
}

type SrcEventsMock struct {
	frameChan    chan *events.FrameMessage
	steeringChan chan *events.SteeringMessage
	throttleChan chan *events.ThrottleMessage
}

func (s *SrcEventsMock) Close() {
	if s.frameChan != nil {
		close(s.frameChan)
	}
	if s.steeringChan != nil {
		close(s.steeringChan)
	}
	if s.throttleChan != nil {
		close(s.throttleChan)
	}
}

func (s *SrcEventsMock) WriteFrame(msg *events.FrameMessage) {
	s.frameChan <- msg
}
func (s *SrcEventsMock) WriteSteering(msg *events.SteeringMessage) {
	s.steeringChan <- msg
}
func (s *SrcEventsMock) WriteThrottle(msg *events.ThrottleMessage) {
	s.throttleChan <- msg
}

func (s *SrcEventsMock) SubscribeFrame() <-chan *events.FrameMessage {
	return s.frameChan
}

func (s *SrcEventsMock) SubscribeSteering() <-chan *events.SteeringMessage {
	return s.steeringChan
}

func (s *SrcEventsMock) SubscribeThrottle() <-chan *events.ThrottleMessage {
	return s.throttleChan
}

func NewPublisherMock(topicFrame string, topicSteering string, topicThrottle string) *PublisherMock {
	return &PublisherMock{
		topicFrame:    topicFrame,
		topicSteering: topicSteering,
		topicThrottle: topicThrottle,
		frameChan:     make(chan []byte),
		steeringChan:  make(chan []byte),
		throttleChan:  make(chan []byte),
	}
}

type PublisherMock struct {
	frameChan     chan []byte
	steeringChan  chan []byte
	throttleChan  chan []byte
	topicFrame    string
	topicSteering string
	topicThrottle string
}

func (p *PublisherMock) Close() {
	close(p.frameChan)
	close(p.steeringChan)
	close(p.throttleChan)
}

func (p *PublisherMock) NotifyFrame() <-chan []byte {
	return p.frameChan
}
func (p *PublisherMock) NotifySteering() <-chan []byte {
	return p.steeringChan
}
func (p *PublisherMock) NotifyThrottle() <-chan []byte {
	return p.throttleChan
}

func (p PublisherMock) Publish(topic string, payload []byte) error {

	switch topic {
	case p.topicFrame:
		p.frameChan <- payload
	case p.topicSteering:
		p.steeringChan <- payload
	case p.topicThrottle:
		p.throttleChan <- payload
	default:
		return fmt.Errorf("invalid topic: %v", topic)
	}
	return nil
}

func TestMsgPublisher_Frame(t *testing.T) {
	sem := NewMockSimulator()
	defer sem.Close()

	p := NewPublisherMock("frame", "steering", "throttle")
	defer p.Close()

	mp := NewMsgPublisher(sem, p, "frame", "steering", "throttle")
	mp.Start()
	defer mp.Stop()

	frameMsg := &events.FrameMessage{
		Id: &events.FrameRef{
			Name: "frame1",
			Id:   "1",
			CreatedAt: &timestamp.Timestamp{
				Seconds: time.Now().Unix(),
			},
		},
		Frame: []byte("image content"),
	}

	go sem.WriteFrame(frameMsg)

	framePayload := <-p.NotifyFrame()
	var frameSended events.FrameMessage
	err := proto.Unmarshal(framePayload, &frameSended)
	if err != nil {
		t.Errorf("unable to unmarshal frame msg: %v", err)
	}
	if frameSended.Id.Id != frameMsg.Id.Id {
		t.Errorf("invalid id frame '%v', wants %v", frameSended.Id.Id, frameMsg.Id.Id)
	}
	if frameSended.Id.Name != frameMsg.Id.Name {
		t.Errorf("invalid name frame '%v', wants %v", frameSended.Id.Name, frameMsg.Id.Name)
	}

}
func TestMsgPublisher_Steering(t *testing.T) {
	sem := NewMockSimulator()
	defer sem.Close()

	p := NewPublisherMock("frame", "steering", "throttle")
	defer p.Close()

	mp := NewMsgPublisher(sem, p, "frame", "steering", "throttle")
	mp.Start()
	defer mp.Stop()

	steeringMsg := &events.SteeringMessage{
		FrameRef: &events.FrameRef{
			Name: "frame1",
			Id:   "1",
			CreatedAt: &timestamp.Timestamp{
				Seconds: time.Now().Unix(),
			},
		},
		Steering:   0.8,
		Confidence: 1.0,
	}

	go sem.WriteSteering(steeringMsg)

	steeringPayload := <-p.NotifySteering()
	var steeringSended events.SteeringMessage
	err := proto.Unmarshal(steeringPayload, &steeringSended)
	if err != nil {
		t.Errorf("unable to unmarshal steering msg: %v", err)
	}
	if steeringSended.FrameRef.Id != steeringMsg.FrameRef.Id {
		t.Errorf("invalid id frame '%v', wants %v", steeringSended.FrameRef.Id, steeringMsg.FrameRef.Id)
	}
	if steeringSended.FrameRef.Name != steeringMsg.FrameRef.Name {
		t.Errorf("invalid name frame '%v', wants %v", steeringSended.FrameRef.Name, steeringMsg.FrameRef.Name)
	}
	if steeringSended.Steering != steeringMsg.Steering {
		t.Errorf("invalid steering value '%v', wants '%v'", steeringSended.Steering, steeringMsg.Steering)
	}
	if steeringSended.Confidence != steeringMsg.Confidence {
		t.Errorf("invalid steering confidence value '%v', wants '%v'", steeringSended.Confidence, steeringMsg.Confidence)
	}
}
func TestMsgPublisher_Throttle(t *testing.T) {
	sem := NewMockSimulator()
	defer sem.Close()

	p := NewPublisherMock("frame", "steering", "throttle")
	defer p.Close()

	mp := NewMsgPublisher(sem, p, "frame", "steering", "throttle")
	mp.Start()
	defer mp.Stop()

	throttleMsg := &events.ThrottleMessage{
		FrameRef: &events.FrameRef{
			Name: "frame1",
			Id:   "1",
			CreatedAt: &timestamp.Timestamp{
				Seconds: time.Now().Unix(),
			},
		},
		Throttle:   0.7,
		Confidence: 1.0,
	}

	go sem.WriteThrottle(throttleMsg)

	throttlePayload := <-p.NotifyThrottle()
	var throttleSended events.ThrottleMessage
	err := proto.Unmarshal(throttlePayload, &throttleSended)
	if err != nil {
		t.Errorf("unable to unmarshal throttle msg: %v", err)
	}
	if throttleSended.FrameRef.Id != throttleMsg.FrameRef.Id {
		t.Errorf("invalid id frame '%v', wants %v", throttleSended.FrameRef.Id, throttleMsg.FrameRef.Id)
	}
	if throttleSended.FrameRef.Name != throttleMsg.FrameRef.Name {
		t.Errorf("invalid name frame '%v', wants %v", throttleSended.FrameRef.Name, throttleMsg.FrameRef.Name)
	}
	if throttleSended.Throttle != throttleMsg.Throttle {
		t.Errorf("invalid throttle value '%v', wants '%v'", throttleSended.Throttle, throttleMsg.Throttle)
	}
	if throttleSended.Confidence != throttleMsg.Confidence {
		t.Errorf("invalid throttle confidence value '%v', wants '%v'", throttleSended.Confidence, throttleMsg.Confidence)
	}
}
