package log4j2

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/annchain/BlockDB/processors"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"time"
)

type Log4j2SocketProcessorConfig struct {
	IdleConnectionTimeout time.Duration
}

type Log4j2SocketProcessor struct {
	config Log4j2SocketProcessorConfig
}

func NewLog4j2SocketProcessor(config Log4j2SocketProcessorConfig) *Log4j2SocketProcessor {
	return &Log4j2SocketProcessor{
		config: config,
	}
}

func (m *Log4j2SocketProcessor) Start() {
	logrus.Info("Log4j2SocketProcessor started")
}

func (m *Log4j2SocketProcessor) Stop() {
	logrus.Info("Log4j2SocketProcessor stopped")
}

func (m *Log4j2SocketProcessor) ProcessConnection(conn net.Conn) error {
	for {
		conn.SetReadDeadline(time.Now().Add(m.config.IdleConnectionTimeout))
		str, err := bufio.NewReader(conn).ReadString(byte(0))
		if err != nil {
			if err == io.EOF {
				logrus.Info("target closed")
				return nil
			} else if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
				logrus.Info("target timeout")
				conn.Close()
				return nil
			}
			return err
		}
		str = str[:len(str)-1]
		// query command
		fmt.Println(str)
		//fmt.Println(hex.Dump(bytes))
		event := m.ParseCommand([]byte(str))
		if event == nil {
			continue
		}
		event.Ip = conn.RemoteAddr().String()
		fmt.Printf("%+v\n", event)

		// TODO: store it to blockchain
	}
}

func (m *Log4j2SocketProcessor) ParseCommand(bytes []byte) *processors.LogEvent {
	log4j := Log4j2SocketEvent{}
	if err := json.Unmarshal(bytes, &log4j); err != nil {
		logrus.WithError(err).Warn("bad format")
		fmt.Println(hex.Dump(bytes))
		return nil
	}
	cmap := log4j.ContextMap
	cmap["message"] = log4j.Message

	data, err := json.Marshal(cmap)
	if err != nil {
		logrus.WithError(err).Warn("bad format")
		fmt.Println(hex.Dump(bytes))
		return nil
	}
	event := processors.LogEvent{
		Timestamp: log4j.Instant.Timestamp,
		Data:      string(data),
	}
	return &event

}