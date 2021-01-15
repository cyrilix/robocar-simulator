package gateway

import (
	"encoding/json"
	"github.com/cyrilix/robocar-protobuf/go/events"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
	"time"
)

type MockPublisher struct {
	notifyFrameChan        chan []byte
	initNotifyFrameChan    sync.Once
	notifySteeringChan     chan []byte
	initNotifySteeringChan sync.Once
	notifyThrottleChan     chan []byte
	initNotifyThrottleChan sync.Once
}

func (p *MockPublisher) Close() error {
	if p.notifyFrameChan != nil {
		close(p.notifyFrameChan)
	}
	if p.notifyThrottleChan != nil {
		close(p.notifyThrottleChan)
	}
	if p.notifySteeringChan != nil {
		close(p.notifySteeringChan)
	}
	return nil
}

func (p *MockPublisher) PublishFrame(payload []byte) {
	p.notifyFrameChan <- payload
}
func (p *MockPublisher) PublishSteering(payload []byte) {
	p.notifySteeringChan <- payload
}
func (p *MockPublisher) PublishThrottle(payload []byte) {
	p.notifyThrottleChan <- payload
}

func (p *MockPublisher) NotifyFrame() <-chan []byte {
	p.initNotifyFrameChan.Do(func() { p.notifyFrameChan = make(chan []byte) })
	return p.notifyFrameChan
}
func (p *MockPublisher) NotifySteering() <-chan []byte {
	p.initNotifySteeringChan.Do(func() { p.notifySteeringChan = make(chan []byte) })
	return p.notifySteeringChan
}
func (p *MockPublisher) NotifyThrottle() <-chan []byte {
	p.initNotifyThrottleChan.Do(func() { p.notifyThrottleChan = make(chan []byte) })
	return p.notifyThrottleChan
}

func TestGateway_ListenEvents(t *testing.T) {
	simulatorMock := Sim2GwMock{}
	err := simulatorMock.Start()
	if err != nil {
		t.Errorf("unable to start mock server: %v", err)
	}
	defer func() {
		if err := simulatorMock.Close(); err != nil {
			t.Errorf("unable to close mock server: %v", err)
		}
	}()

	publisher := MockPublisher{}

	part := New(&publisher, simulatorMock.Addr())
	go func() {
		err := part.Start()
		if err != nil {
			t.Fatalf("unable to start gateway simulator: %v", err)
		}
	}()
	defer func() {
		if err := part.Close(); err != nil {
			t.Errorf("unable to close gateway simulator: %v", err)
		}
	}()

	simulatorMock.WaitConnection()
	log.Trace("read test data")
	testContent, err := ioutil.ReadFile("testdata/msg.json")
	lines := strings.Split(string(testContent), "\n")

	for idx, line := range lines {
		err = simulatorMock.EmitMsg(line)
		if err != nil {
			t.Errorf("[line %v/%v] unable to write line: %v", idx+1, len(lines), err)
		}

		eventsType := map[string]bool{"frame": false, "steering": false, "throttle": false}
		nbEventsExpected := len(eventsType)
		wg := sync.WaitGroup{}
		// Expect number mqtt event

		wg.Add(nbEventsExpected)
		finished := make(chan struct{})
		go func() {
			wg.Wait()
			finished <- struct{}{}
		}()

		timeout := time.Tick(100 * time.Millisecond)

		endLoop := false
		for {
			select {
			case byteMsg := <-publisher.NotifyFrame():
				checkFrame(t, byteMsg)
				eventsType["frame"] = true
				wg.Done()
			case byteMsg := <-publisher.NotifySteering():
				checkSteering(t, byteMsg, line)
				eventsType["steering"] = true
				wg.Done()
			case byteMsg := <-publisher.NotifyThrottle():
				checkThrottle(t, byteMsg, line)
				eventsType["throttle"] = true
				wg.Done()
			case <-finished:
				log.Trace("loop ended")
				endLoop = true
			case <-timeout:
				t.Errorf("not all event are published")
				t.FailNow()
			}
			if endLoop {
				break
			}
		}
		for k, v := range eventsType {
			if !v {
				t.Errorf("no %v event published for line %v", k, line)
			}
		}
	}
}

func checkFrame(t *testing.T, byteMsg []byte) {
	var msg events.FrameMessage
	err := proto.Unmarshal(byteMsg, &msg)
	if err != nil {
		t.Errorf("unable to unmarshal frame msg: %v", err)
	}
	if msg.GetId() == nil {
		t.Error("frame msg has not Id")
	}
	if len(msg.Frame) < 10 {
		t.Errorf("[%v] invalid frame image: %v", msg.Id, msg.GetFrame())
	}
}
func checkSteering(t *testing.T, byteMsg []byte, rawLine string) {
	var input map[string]interface{}
	err := json.Unmarshal([]byte(rawLine), &input)
	if err != nil {
		t.Fatalf("unable to parse input data '%v': %v", rawLine, err)
	}
	steering := input["steering_angle"].(float64)
	expectedSteering := float32(steering)

	var msg events.SteeringMessage
	err = proto.Unmarshal(byteMsg, &msg)
	if err != nil {
		t.Errorf("unable to unmarshal steering msg: %v", err)
	}

	if msg.GetSteering() != expectedSteering {
		t.Errorf("invalid steering value: %f, wants %f", msg.GetSteering(), expectedSteering)
	}
	if msg.Confidence != 1.0 {
		t.Errorf("invalid steering confidence: %f, wants %f", msg.Confidence, 1.0)
	}
}

func checkThrottle(t *testing.T, byteMsg []byte, rawLine string) {
	var input map[string]interface{}
	err := json.Unmarshal([]byte(rawLine), &input)
	if err != nil {
		t.Fatalf("unable to parse input data '%v': %v", rawLine, err)
	}
	throttle := input["throttle"].(float64)
	expectedThrottle := float32(throttle)

	var msg events.SteeringMessage
	err = proto.Unmarshal(byteMsg, &msg)
	if err != nil {
		t.Errorf("unable to unmarshal throttle msg: %v", err)
	}

	if msg.GetSteering() != expectedThrottle {
		t.Errorf("invalid throttle value: %f, wants %f", msg.GetSteering(), expectedThrottle)
	}
	if msg.Confidence != 1.0 {
		t.Errorf("invalid throttle confidence: %f, wants %f", msg.Confidence, 1.0)
	}
}
