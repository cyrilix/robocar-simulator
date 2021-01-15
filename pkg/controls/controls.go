package controls

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
)

type SteeringController interface {
 	WriteSteering(message *events.SteeringMessage)
}

type ThrottleController interface {
	WriteThrottle(message *events.ThrottleMessage)
}

func New(address string) (*Gateway, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to %v", address)
	}
	return &Gateway{
		conn:  conn,
	}, nil
}

/* Simulator interface to publish command controls from mqtt topic */
type Gateway struct {
	cancel           chan interface{}

	muControl   sync.Mutex
	lastControl *simulator.ControlMsg
	conn        io.WriteCloser
}

func (g *Gateway) Start() {
	log.Info("connect to simulator")
	g.cancel = make(chan interface{})
}

func (g *Gateway) Stop() {
	log.Info("close simulator gateway")
	close(g.cancel)

	if err := g.Close(); err != nil {
		log.Printf("unexpected error while simulator connection is closed: %v", err)
	}
}

func (g *Gateway) Close() error {
	if g.conn == nil {
		log.Warnln("no connection to close")
		return nil
	}
	if err := g.conn.Close(); err != nil {
		return fmt.Errorf("unable to close connection to simulator: %v", err)
	}
	return nil
}

func (g *Gateway) WriteSteering(message *events.SteeringMessage) {
	g.muControl.Lock()
	defer g.muControl.Unlock()
	g.initLastControlMsg()

	g.lastControl.Steering = message.Steering
	g.writeContent()
}

func (g *Gateway) WriteThrottle(message *events.ThrottleMessage) {
	g.muControl.Lock()
	defer g.muControl.Unlock()
	g.initLastControlMsg()

	if message.Throttle > 0 {
		g.lastControl.Throttle = message.Throttle
		g.lastControl.Brake = 0.
	} else {
		g.lastControl.Throttle = 0.
		g.lastControl.Brake = -1 * message.Throttle
	}

	g.writeContent()
}

func (g *Gateway) writeContent() {
	w := bufio.NewWriter(g.conn)
	content, err := json.Marshal(g.lastControl)
	if err != nil {
		log.Errorf("unable to marshall control msg \"%#v\": %v", g.lastControl, err)
		return
	}

	_, err = w.Write(append(content, '\n'))
	if err != nil {
		log.Errorf("unable to write control msg \"%#v\" to simulator: %v", g.lastControl, err)
		return
	}
	err = w.Flush()
	if err != nil {
		log.Errorf("unable to flush control msg \"%#v\" to simulator: %v", g.lastControl, err)
		return
	}
}

func (g *Gateway) initLastControlMsg() {
	if g.lastControl != nil {
		return
	}
	g.lastControl = &simulator.ControlMsg{
		MsgType:  "control",
		Steering: 0.,
		Throttle: 0.,
		Brake:    0.,
	}
}

