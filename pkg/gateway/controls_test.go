package gateway

import (
	"github.com/cyrilix/robocar-protobuf/go/events"
	"github.com/cyrilix/robocar-simulator/pkg/simulator"
	"testing"
)

func TestGateway_WriteSteering(t *testing.T) {

	cases := []struct {
		name        string
		msg         *events.SteeringMessage
		previousMsg *simulator.ControlMsg
		expectedMsg simulator.ControlMsg
	}{
		{"First Message",
			&events.SteeringMessage{Steering: 0.5, Confidence: 1},
			nil,
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.50",
				Throttle: "0.0",
				Brake:    "0.0",
			}},
		{"Update steering",
			&events.SteeringMessage{Steering: -0.5, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.2",
				Throttle: "0.0",
				Brake:    "0.0",
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: "-0.50",
				Throttle: "0.0",
				Brake:    "0.0",
			}},
		{"Update steering shouldn't erase throttle value",
			&events.SteeringMessage{Steering: -0.3, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.2",
				Throttle: "0.6",
				Brake:    "0.1",
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: "-0.30",
				Throttle: "0.6",
				Brake:    "0.1",
			}},
	}

	simulatorMock := Gw2SimMock{}
	err := simulatorMock.listen()
	if err != nil {
		t.Errorf("unable to start mock gw: %v", err)
	}
	defer func() {
		if err := simulatorMock.Close(); err != nil {
			t.Errorf("unable to stop simulator mock: %v", err)
		}
	}()

	gw := New(simulatorMock.Addr())
	if err != nil {
		t.Fatalf("unable to init simulator gateway: %v", err)
	}
	go gw.Start()
	defer gw.Close()

	for _, c := range cases {
		gw.lastControl = c.previousMsg

		gw.WriteSteering(c.msg)

		ctrlMsg := <-simulatorMock.Notify()
		if *ctrlMsg != c.expectedMsg {
			t.Errorf("[%v] bad messge received: %#v, wants %#v", c.name, ctrlMsg, c.expectedMsg)
		}
	}
}

func TestGateway_WriteThrottle(t *testing.T) {

	cases := []struct {
		name        string
		msg         *events.ThrottleMessage
		previousMsg *simulator.ControlMsg
		expectedMsg simulator.ControlMsg
	}{
		{"First Message",
			&events.ThrottleMessage{Throttle: 0.5, Confidence: 1},
			nil,
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.0",
				Throttle: "0.50",
				Brake:    "0.0",
			}},
		{"Update Throttle",
			&events.ThrottleMessage{Throttle: 0.6, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0",
				Throttle: "0.4",
				Brake:    "0",
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0",
				Throttle: "0.60",
				Brake:    "0.0",
			}},
		{"Update steering shouldn't erase throttle value",
			&events.ThrottleMessage{Throttle: 0.3, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.2",
				Throttle: "0.6",
				Brake:    "0.0",
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.2",
				Throttle: "0.30",
				Brake:    "0.0",
			}},
		{"Throttle to brake",
			&events.ThrottleMessage{Throttle: -0.7, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.2",
				Throttle: "0.6",
				Brake:    "0.0",
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.2",
				Throttle: "0.0",
				Brake:    "0.70",
			}},
		{"Update brake",
			&events.ThrottleMessage{Throttle: -0.2, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.2",
				Throttle: "0.0",
				Brake:    "0.5",
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.2",
				Throttle: "0.0",
				Brake:    "0.20",
			}},
		{"Brake to throttle",
			&events.ThrottleMessage{Throttle: 0.9, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.2",
				Throttle: "0.0",
				Brake:    "0.4",
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: "0.2",
				Throttle: "0.90",
				Brake:    "0.0",
			}},
	}

	simulatorMock := Gw2SimMock{}
	err := simulatorMock.listen()
	if err != nil {
		t.Errorf("unable to start mock gw: %v", err)
	}
	defer func() {
		if err := simulatorMock.Close(); err != nil {
			t.Errorf("unable to stop simulator mock: %v", err)
		}
	}()

	gw := New(simulatorMock.Addr())
	if err != nil {
		t.Fatalf("unable to init simulator gateway: %v", err)
	}

	for _, c := range cases {
		gw.lastControl = c.previousMsg

		gw.WriteThrottle(c.msg)

		ctrlMsg := <-simulatorMock.Notify()
		if *ctrlMsg != c.expectedMsg {
			t.Errorf("[%v] bad messge received: %#v, wants %#v", c.name, ctrlMsg, c.expectedMsg)
		}
	}
}
