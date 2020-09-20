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
	"time"
)

type MockPublisher struct {
	muPubEvents     sync.Mutex
	publishedEvents []*[]byte
}

func (p *MockPublisher) Publish(payload *[]byte) {
	p.muPubEvents.Lock()
	defer p.muPubEvents.Unlock()
	p.publishedEvents = append(p.publishedEvents, payload)
}

func (p *MockPublisher) Events() []*[]byte {
	p.muPubEvents.Lock()
	defer p.muPubEvents.Unlock()
	eventsMsg := make([]*[]byte, len(p.publishedEvents), len(p.publishedEvents))
	copy(eventsMsg, p.publishedEvents)
	return eventsMsg
}

func TestPart_ListenEvents(t *testing.T) {
	connMock := ConnMock{}
	err := connMock.Listen()
	if err != nil {
		t.Errorf("unable to start mock server: %v", err)
	}
	defer func() {
		if err := connMock.Close(); err != nil {
			t.Errorf("unable to close mock server: %v", err)
		}
	}()


	publisher := MockPublisher{publishedEvents: make([]*[]byte, 0)}

	part := New(&publisher, connMock.Addr())
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

	testContent, err := ioutil.ReadFile("testdata/msg.json")
	lines := strings.Split(string(testContent), "\n")

	for idx, line := range lines {
		time.Sleep(5 * time.Millisecond)
		err = connMock.WriteMsg(line)
		if err != nil {
			t.Errorf("[line %v/%v] unable to write line: %v", idx+1, len(lines), err)
		}
	}

	time.Sleep(10 * time.Millisecond)
	expectedFrames := 16
	if len(publisher.Events()) != expectedFrames {
		t.Errorf("invalid number of frame emmitted: %v, wants %v", len(publisher.Events()), expectedFrames)
	}
	for _, byteMsg := range publisher.Events() {
		var msg events.FrameMessage
		err = proto.Unmarshal(*byteMsg, &msg)
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

type ConnMock struct {
	ln     net.Listener
	conn   net.Conn
	muWriter sync.Mutex
	writer *bufio.Writer
}

func (c *ConnMock) WriteMsg(p string) (err error) {
	c.muWriter.Lock()
	defer c.muWriter.Unlock()
	_, err = c.writer.WriteString(p + "\n")
	if err != nil {
		log.Errorf("unable to write response: %v", err)
	}
	if err == io.EOF {
		log.Info("Connection closed")
		return err
	}
	err = c.writer.Flush()
	return err
}

func (c *ConnMock) Listen() error {
	c.muWriter.Lock()
	defer c.muWriter.Unlock()
	ln, err := net.Listen("tcp", "127.0.0.1:")
	c.ln = ln
	if err != nil {

		return fmt.Errorf("unable to listen on port: %v", err)
	}

	go func() {
		for {
			c.conn, err = c.ln.Accept()
			if err != nil && err == io.EOF {
				log.Infof("connection close: %v", err)
				break
			}
			c.writer = bufio.NewWriter(c.conn)
		}
	}()
	return nil
}

func (c *ConnMock) Addr() string {
	return c.ln.Addr().String()
}

func (c *ConnMock) Close() error {
	log.Infof("close mock server")
	err := c.ln.Close()
	if err != nil {
		return fmt.Errorf("unable to close mock server: %v", err)
	}
	return nil
}
