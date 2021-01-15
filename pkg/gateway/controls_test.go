package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/cyrilix/robocar-protobuf/go/events"
	"github.com/cyrilix/robocar-simulator/pkg/simulator"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"sync"
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
				Steering: 0.5,
				Throttle: 0,
				Brake:    0,
			}},
		{"Update steering",
			&events.SteeringMessage{Steering: -0.5, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0.2,
				Throttle: 0,
				Brake:    0,
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: -0.5,
				Throttle: 0,
				Brake:    0,
			}},
		{"Update steering shouldn't erase throttle value",
			&events.SteeringMessage{Steering: -0.3, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0.2,
				Throttle: 0.6,
				Brake:    0.1,
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: -0.3,
				Throttle: 0.6,
				Brake:    0.1,
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

	gw := New(nil, simulatorMock.Addr())
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
				Steering: 0,
				Throttle: 0.5,
				Brake:    0,
			}},
		{"Update Throttle",
			&events.ThrottleMessage{Throttle: 0.6, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0,
				Throttle: 0.4,
				Brake:    0,
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0,
				Throttle: 0.6,
				Brake:    0,
			}},
		{"Update steering shouldn't erase throttle value",
			&events.ThrottleMessage{Throttle: 0.3, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0.2,
				Throttle: 0.6,
				Brake:    0.,
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0.2,
				Throttle: 0.3,
				Brake:    0.,
			}},
		{"Throttle to brake",
			&events.ThrottleMessage{Throttle: -0.7, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0.2,
				Throttle: 0.6,
				Brake:    0.,
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0.2,
				Throttle: 0.,
				Brake:    0.7,
			}},
		{"Update brake",
			&events.ThrottleMessage{Throttle: -0.2, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0.2,
				Throttle: 0.,
				Brake:    0.5,
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0.2,
				Throttle: 0.,
				Brake:    0.2,
			}},
		{"Brake to throttle",
			&events.ThrottleMessage{Throttle: 0.9, Confidence: 1},
			&simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0.2,
				Throttle: 0.,
				Brake:    0.4,
			},
			simulator.ControlMsg{
				MsgType:  "control",
				Steering: 0.2,
				Throttle: 0.9,
				Brake:    0.,
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

	gw := New(nil, simulatorMock.Addr())
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

type ConnMock struct {
	initMsgsOnce sync.Once

	ln             net.Listener
	notifyChan     chan *simulator.ControlMsg
	initNotifyChan sync.Once
}

func (c *ConnMock) Notify() <-chan *simulator.ControlMsg {
	c.initNotifyChan.Do(func() { c.notifyChan = make(chan *simulator.ControlMsg) })
	return c.notifyChan
}

func (c *ConnMock) listen() error {
	ln, err := net.Listen("tcp", "127.0.0.1:")
	c.ln = ln
	if err != nil {

		return fmt.Errorf("unable to listen on port: %v", err)
	}

	go func() {
		for {
			conn, err := c.ln.Accept()
			if err != nil {
				log.Infof("connection close: %v", err)
				break
			}
			go c.handleConnection(conn)
		}
	}()
	return nil
}

func (c *ConnMock) Addr() string {
	return c.ln.Addr().String()
}

func (c *ConnMock) handleConnection(conn net.Conn) {
	c.initNotifyChan.Do(func() { c.notifyChan = make(chan *simulator.ControlMsg) })
	reader := bufio.NewReader(conn)
	for {
		rawCmd, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				log.Info("connection closed")
				break
			}
			log.Errorf("unable to read request: %v", err)
			return
		}

		var msg simulator.ControlMsg
		err = json.Unmarshal(rawCmd, &msg)
		if err != nil {
			log.Errorf("unable to unmarchal control msg \"%v\": %v", string(rawCmd), err)
			continue
		}

		c.notifyChan <- &msg
	}
}

func (c *ConnMock) Close() error {
	log.Infof("close mock server")
	err := c.ln.Close()
	if err != nil {
		return fmt.Errorf("unable to close mock server: %v", err)
	}
	if c.notifyChan != nil {
		close(c.notifyChan)
	}
	return nil
}
