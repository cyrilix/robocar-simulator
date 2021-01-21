package gateway

import (
	"encoding/json"
	"github.com/cyrilix/robocar-protobuf/go/events"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
	"time"
)

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

	gw := New(simulatorMock.Addr())
	go func() {
		err := gw.Start()
		if err != nil {
			t.Fatalf("unable to start gateway simulator: %v", err)
		}
	}()
	defer func() {
		if err := gw.Close(); err != nil {
			t.Errorf("unable to close gateway simulator: %v", err)
		}
	}()
	frameChannel := gw.SubscribeFrame()
	steeringChannel := gw.SubscribeSteering()
	throttleChannel := gw.SubscribeThrottle()

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
		// Expect number events event

		wg.Add(nbEventsExpected)
		finished := make(chan struct{})
		go func() {
			wg.Wait()
			finished <- struct{}{}
		}()

		timeout := time.Tick(100 * time.Millisecond)

		endLoop := false
		var frameRef, steeringIdRef, throttleIdRef *events.FrameRef

		for {
			select {
			case msg := <-frameChannel:
				checkFrame(t, msg)
				eventsType["frame"] = true
				frameRef = msg.Id
				wg.Done()
			case msg := <-steeringChannel:
				checkSteering(t, msg, line)
				eventsType["steering"] = true
				steeringIdRef = msg.FrameRef
				wg.Done()
			case msg := <-throttleChannel:
				checkThrottle(t, msg, line)
				eventsType["throttle"] = true
				throttleIdRef = msg.FrameRef
				wg.Done()
			case <-finished:
				log.Trace("loop ended")
				endLoop = true
			case <-timeout:
				t.Errorf("not all event are published")
				t.FailNow()
			}
			if endLoop {
				if frameRef != steeringIdRef || steeringIdRef == nil {
					t.Errorf("steering msg without frameRef '%#v', wants '%#v'", steeringIdRef, frameRef)
				}
				if frameRef != throttleIdRef || throttleIdRef == nil {
					t.Errorf("throttle msg without frameRef '%#v', wants '%#v'", throttleIdRef, frameRef)
				}
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

func checkFrame(t *testing.T, msg *events.FrameMessage) {
	if msg.GetId() == nil {
		t.Error("frame msg has not Id")
	}
	if len(msg.Frame) < 10 {
		t.Errorf("[%v] invalid frame image: %v", msg.Id, msg.GetFrame())
	}
}
func checkSteering(t *testing.T, msg *events.SteeringMessage, rawLine string) {
	var input map[string]interface{}
	err := json.Unmarshal([]byte(rawLine), &input)
	if err != nil {
		t.Fatalf("unable to parse input data '%v': %v", rawLine, err)
	}
	steering := input["steering_angle"].(float64)
	expectedSteering := float32(steering)

	if msg.GetSteering() != expectedSteering {
		t.Errorf("invalid steering value: %f, wants %f", msg.GetSteering(), expectedSteering)
	}
	if msg.Confidence != 1.0 {
		t.Errorf("invalid steering confidence: %f, wants %f", msg.Confidence, 1.0)
	}
}

func checkThrottle(t *testing.T, msg *events.ThrottleMessage, rawLine string) {
	var input map[string]interface{}
	err := json.Unmarshal([]byte(rawLine), &input)
	if err != nil {
		t.Fatalf("unable to parse input data '%v': %v", rawLine, err)
	}
	throttle := input["throttle"].(float64)
	expectedThrottle := float32(throttle)

	if msg.Throttle != expectedThrottle {
		t.Errorf("invalid throttle value: %f, wants %f", msg.Throttle, expectedThrottle)
	}
	if msg.Confidence != 1.0 {
		t.Errorf("invalid throttle confidence: %f, wants %f", msg.Confidence, 1.0)
	}
}
