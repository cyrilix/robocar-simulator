package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/cyrilix/robocar-simulator/pkg/simulator"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"sync"
)

type Gw2SimMock struct {
	initMsgsOnce sync.Once

	ln             net.Listener
	notifyChan     chan *simulator.ControlMsg
	initNotifyChan sync.Once
}

func (c *Gw2SimMock) Notify() <-chan *simulator.ControlMsg {
	c.initNotifyChan.Do(func() { c.notifyChan = make(chan *simulator.ControlMsg) })
	return c.notifyChan
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
				log.Debugf("connection close: %v", err)
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
	c.initNotifyChan.Do(func() { c.notifyChan = make(chan *simulator.ControlMsg) })
	reader := bufio.NewReader(conn)
	for {
		rawCmd, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				log.Debug("connection closed")
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

func (c *Gw2SimMock) Close() error {
	log.Debugf("close mock server")
	err := c.ln.Close()
	if err != nil {
		return fmt.Errorf("unable to close mock server: %v", err)
	}
	if c.notifyChan != nil {
		close(c.notifyChan)
	}
	return nil
}

type Sim2GwMock struct {
	ln            net.Listener
	muConn        sync.Mutex
	conn          net.Conn
	writer        *bufio.Writer
	newConnection chan net.Conn
	logger        *log.Entry
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
	c.logger = log.WithField("simulator", "mock")
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
