package service

import (
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"log"
)

func StopService(name string, client mqtt.Client, topics ...string) {
	log.Printf("Stop %s service", name)
	token := client.Unsubscribe(topics...)
	token.Wait()
	if token.Error() != nil {
		log.Printf("unable to unsubscribe service: %v", token.Error())
	}
	client.Disconnect(50)
}

func RegisterCallback(client mqtt.Client, topic string, callback mqtt.MessageHandler) error {
	log.Printf("Register callback on topic %v", topic)
	token := client.Subscribe(topic, 0, callback)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("unable to register callback on topic %s: %v", topic, token.Error())
	}
	return nil
}

type Part interface {
	Start() error
	Stop()
}
