package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/cyrilix/robocar-simulator/pkg/simulator"
	"go.uber.org/zap"
	"io"
	"net"
	"sync"
)

type Gw2SimMock struct {
	initOnce sync.Once

	ln             net.Listener
	notifyCtrlChan chan *simulator.ControlMsg
	notifyCarChan chan *simulator.CarConfigMsg
	notifyCamChan chan *simulator.CamConfigMsg
}

func (c *Gw2SimMock) init(){
	c.notifyCtrlChan = make(chan *simulator.ControlMsg)
	c.notifyCarChan = make(chan *simulator.CarConfigMsg)
	c.notifyCamChan = make(chan *simulator.CamConfigMsg)
}

func (c *Gw2SimMock) NotifyCtrl() <-chan *simulator.ControlMsg {
	c.initOnce.Do(c.init)
	return c.notifyCtrlChan
}
func (c *Gw2SimMock) NotifyCar() <-chan *simulator.CarConfigMsg {
	c.initOnce.Do(c.init)
	return c.notifyCarChan
}
func (c *Gw2SimMock) NotifyCamera() <-chan *simulator.CamConfigMsg {
	c.initOnce.Do(c.init)
	return c.notifyCamChan
}

func (c *Gw2SimMock) listen() error {
	ln, err := net.Listen("tcp", "127.0.0.1:")
	c.ln = ln
	if err != nil {

		return fmt.Errorf("unable to listen on port: %v", err)
	}

	go func() {
		for {
			conn, err := c.ln.Accept()
			if err != nil {
				zap.S().Debugf("connection close: %v", err)
				break
			}
			go c.handleConnection(conn)
		}
	}()
	return nil
}

func (c *Gw2SimMock) Addr() string {
	return c.ln.Addr().String()
}

func (c *Gw2SimMock) handleConnection(conn net.Conn) {
	log := zap.S()
	c.initOnce.Do(c.init)
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		rawMsg, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				log.Debug("connection closed")
				break
			}
			log.Errorf("unable to read request: %v", err)
			return
		}
		var msg simulator.Msg
		err = json.Unmarshal(rawMsg, &msg)
		if err != nil {
			log.Errorf("unable to unmarshal msg \"%v\": %v", string(rawMsg), err)
			continue
		}
		switch msg.MsgType {
		case simulator.MsgTypeControl:
			var msgControl simulator.ControlMsg
			err = json.Unmarshal(rawMsg, &msgControl)
			if err != nil {
				log.Errorf("unable to unmarshal control msg \"%v\": %v", string(rawMsg), err)
				continue
			}
			c.notifyCtrlChan <- &msgControl
		case simulator.MsgTypeCarConfig:
			var msgCar simulator.CarConfigMsg
			err = json.Unmarshal(rawMsg, &msgCar)
			if err != nil {
				log.Errorf("unable to unmarshal car msg \"%v\": %v", string(rawMsg), err)
				continue
			}
			c.notifyCarChan <- &msgCar
			carLoadedMsg := simulator.Msg{MsgType: simulator.MsgTypeCarLoaded}
			resp, err := json.Marshal(&carLoadedMsg)
			if err != nil {
				log.Errorf("unable to generate car loaded response: %v", err)
				continue
			}
			_, err = writer.WriteString(string(resp) + "\n")
			if err != nil {
				log.Errorf("unable to write car loaded response: %v", err)
				continue
			}
			err = writer.Flush()
			if err != nil {
				log.Errorf("unable to flush car loaded response: %v", err)
				continue
			}
		case simulator.MsgTypeCameraConfig:
			var msgCam simulator.CamConfigMsg
			err = json.Unmarshal(rawMsg, &msgCam)
			if err != nil {
				log.Errorf("unable to unmarshal camera msg \"%v\": %v", string(rawMsg), err)
				continue
			}
			c.notifyCamChan <- &msgCam
		}
	}
}

func (c *Gw2SimMock) Close() error {
	zap.S().Debugf("close mock server")
	err := c.ln.Close()
	if err != nil {
		return fmt.Errorf("unable to close mock server: %v", err)
	}
	if c.notifyCtrlChan != nil {
		close(c.notifyCtrlChan)
	}
	if c.notifyCarChan != nil {
		close(c.notifyCarChan)
	}
	if c.notifyCamChan != nil {
		close(c.notifyCamChan)
	}
	return nil
}

type Sim2GwMock struct {
	ln            net.Listener
	muConn        sync.Mutex
	conn          net.Conn
	writer        *bufio.Writer
	newConnection chan net.Conn
	logger        *zap.SugaredLogger
}

func (c *Sim2GwMock) EmitMsg(p string) (err error) {
	c.muConn.Lock()
	defer c.muConn.Unlock()
	_, err = c.writer.WriteString(p + "\n")
	if err != nil {
		c.logger.Errorf("unable to write response: %v", err)
	}
	if err == io.EOF {
		c.logger.Info("Connection closed")
		return err
	}
	err = c.writer.Flush()
	return err
}

func (c *Sim2GwMock) WaitConnection() {
	c.muConn.Lock()
	defer c.muConn.Unlock()
	c.logger.Debug("simulator waiting connection")
	if c.conn != nil {
		return
	}
	c.logger.Debug("new connection")
	conn := <-c.newConnection

	c.conn = conn
	c.writer = bufio.NewWriter(conn)
}

func (c *Sim2GwMock) Start() error {
	c.logger = zap.S().With("simulator", "mock")
	c.newConnection = make(chan net.Conn)
	ln, err := net.Listen("tcp", "127.0.0.1:")
	c.ln = ln
	if err != nil {
		return fmt.Errorf("unable to listen on port: %v", err)
	}
	go func() {
		for {
			conn, err := c.ln.Accept()
			if err != nil && err == io.EOF {
				c.logger.Errorf("connection close: %v", err)
				break
			}
			if c.newConnection == nil {
				break
			}
			c.newConnection <- conn
		}
	}()
	return nil
}

func (c *Sim2GwMock) Addr() string {
	return c.ln.Addr().String()
}

func (c *Sim2GwMock) Close() error {
	c.logger.Debug("close mock server")

	if c == nil {
		return nil
	}
	close(c.newConnection)
	c.newConnection = nil
	err := c.ln.Close()
	if err != nil {
		return fmt.Errorf("unable to close mock server: %v", err)
	}
	return nil
}
