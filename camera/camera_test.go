package camera

import (
	"bufio"
	"fmt"
	"github.com/cyrilix/robocar-protobuf/go/events"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"testing"
)

type MockPublisher struct {
	notifyChan     chan []byte
	initNotifyChan sync.Once
}

func (p *MockPublisher) Close() error {
	if p.notifyChan != nil {
		close(p.notifyChan)
	}
	return nil
}

func (p *MockPublisher) Publish(payload []byte) {
	p.notifyChan <- payload
}

func (p *MockPublisher) Notify() <-chan []byte {
	p.initNotifyChan.Do(func() { p.notifyChan = make(chan []byte) })
	return p.notifyChan
}

func TestPart_ListenEvents(t *testing.T) {
	simulatorMock := SimulatorMock{}
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
			t.Fatalf("unable to start camera simulator: %v", err)
		}
	}()
	defer func() {
		if err := part.Close(); err != nil {
			t.Errorf("unable to close camera simulator: %v", err)
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

		byteMsg := <-publisher.Notify()
		var msg events.FrameMessage
		err = proto.Unmarshal(byteMsg, &msg)
		if err != nil {
			t.Errorf("unable to unmarshal frame msg: %v", err)
			continue
		}
		if msg.GetId() == nil {
			t.Error("frame msg has not Id")
		}
		if len(msg.Frame) < 10 {
			t.Errorf("[%v] invalid frame image: %v", msg.Id, msg.GetFrame())
		}
	}
}

type SimulatorMock struct {
	ln            net.Listener
	muConn        sync.Mutex
	conn          net.Conn
	writer        *bufio.Writer
	newConnection chan net.Conn
	logger 		  *log.Entry
}

func (c *SimulatorMock) EmitMsg(p string) (err error) {
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

func (c *SimulatorMock) WaitConnection() {
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

func (c *SimulatorMock) Start() error {
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

func (c *SimulatorMock) Addr() string {
	return c.ln.Addr().String()
}

func (c *SimulatorMock) Close() error {
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
