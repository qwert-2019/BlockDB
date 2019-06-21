package mongodb

import (
	"fmt"
	"github.com/annchain/BlockDB/common/bytes"
	"github.com/annchain/BlockDB/multiplexer"
	"github.com/annchain/BlockDB/plugins/server/mongodb/message"
	"github.com/annchain/BlockDB/processors"
	"io"
	"sync"
	"time"
)

type ExtractorFactory struct{}

func (ExtractorFactory) GetInstance(context multiplexer.DialogContext) multiplexer.Observer {
	return &Extractor{
		req:  NewRequestExtractor(context),
		resp: NewResponseExtractor(context),
	}
}

type Extractor struct {
	req  ExtractorInterface
	resp ExtractorInterface
}

func (e *Extractor) GetIncomingWriter() io.Writer {
	return e.req
}

func (e *Extractor) GetOutgoingWriter() io.Writer {
	return e.resp
}

type ExtractorInterface interface {
	Write(p []byte) (n int, err error)

	// init is called when message struct is detected.
	init(header *message.MessageHeader)

	// reset() is called when extractor finishes one iteration of extraction.
	// All the variables are set to origin situations to wait next message
	// extraction.
	reset() error
}

type RequestExtractor struct {
	header *message.MessageHeader
	buf    []byte

	extract func(h *message.MessageHeader, b []byte) (*message.Message, error)

	mu      sync.RWMutex
	context multiplexer.DialogContext
}

func NewRequestExtractor(context multiplexer.DialogContext) *RequestExtractor {
	r := &RequestExtractor{}

	r.buf = make([]byte, 0)
	r.extract = extractMessage
	r.context = context

	return r
}

// Writer is the interface that wraps the basic Write method.
//
// Write writes len(p) bytes from p to the underlying data stream.
// It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// Write must return a non-nil error if it returns n < len(p).
// Write must not modify the slice data, even temporarily.
//
// Implementations must not retain p.
func (e *RequestExtractor) Write(p []byte) (int, error) {
	b := make([]byte, len(p))
	copy(b, p)

	e.mu.Lock()
	defer e.mu.Unlock()

	e.buf = append(e.buf, b...)

	// init header
	if e.header == nil && len(e.buf) > message.HeaderLen {
		header, err := decodeHeader(e.buf)
		if err != nil {
			return len(b), err
		}
		e.init(header)
	}
	// case that buf size no larger than header length or buf not matches msg size.
	if e.header == nil || int32(len(e.buf)) < e.header.MessageSize {
		return len(b), nil
	}

	msg, err := e.extract(e.header, e.buf[:e.header.MessageSize])
	if err != nil {
		return len(b), err
	}
	// TODO write msg to blockDB.
	logEvent := &processors.LogEvent{
		Ip:        e.context.Source.RemoteAddr().String(),
		Data:      "YOUR MESSAGE",
		Timestamp: int(time.Now().Unix()),
		Identity:  msg.DBUser,
		Type:      "mongo",
	}
	//TODO: write logEvent to the mongoDB
	fmt.Println(logEvent)

	e.reset()
	return len(b), nil
}

func (e *RequestExtractor) init(header *message.MessageHeader) {
	e.header = header
}

func (e *RequestExtractor) reset() error {
	if e.header == nil {
		e.buf = make([]byte, 0)
		return nil
	}
	e.buf = e.buf[e.header.MessageSize:]
	e.header = nil
	return nil
}

type ResponseExtractor struct {
	context multiplexer.DialogContext
}

func NewResponseExtractor(context multiplexer.DialogContext) *ResponseExtractor {
	return &ResponseExtractor{
		context: context,
	}
}

func (e *ResponseExtractor) Write(p []byte) (int, error) {
	// TODO
	return len(p), nil
}

func (e *ResponseExtractor) init(header *message.MessageHeader) {
	// TODO

}

func (e *ResponseExtractor) reset() error {
	// TODO
	return nil
}

func decodeHeader(b []byte) (*message.MessageHeader, error) {
	if len(b) < message.HeaderLen {
		return nil, fmt.Errorf("not enough length for header decoding, expect %d, get %d", message.HeaderLen, len(b))
	}

	m := &message.MessageHeader{
		MessageSize: bytes.GetInt32(b, 0),
		RequestID:   bytes.GetInt32(b, 4),
		ResponseTo:  bytes.GetInt32(b, 8),
		OpCode:      message.OpCode(bytes.GetInt32(b, 12)),
	}
	return m, nil
}

func extractMessage(header *message.MessageHeader, b []byte) (*message.Message, error) {

	m := &message.Message{}

	switch header.OpCode {
	case message.OpReply:
		m.MongoMsg = message.NewReplyMessage(header, b)
		break
	case message.OpUpdate:
		m.MongoMsg = message.NewUpdateMessage(header, b)
		break
	case message.OpInsert:
		m.MongoMsg = message.NewInsertMessage(header, b)
		break
	case message.Reserved:
		m.MongoMsg = message.NewReservedMessage(header, b)
		break
	case message.OpQuery:
		m.MongoMsg = message.NewQueryMessage(header, b)
		break
	case message.OpGetMore:
		m.MongoMsg = message.NewGetMoreMessage(header, b)
		break
	case message.OpDelete:
		m.MongoMsg = message.NewDeleteMessage(header, b)
		break
	case message.OpKillCursors:
		m.MongoMsg = message.NewKillCursorsMessage(header, b)
		break
	case message.OpCommand:
		m.MongoMsg = message.NewCommandMessage(header, b)
		break
	case message.OpCommandReply:
		m.MongoMsg = message.NewCommandReplyMessage(header, b)
		break
	case message.OpMsg:
		m.MongoMsg = message.NewMsgMessage(header, b)
		break
	default:
		return nil, fmt.Errorf("unknown opcode: %d", header.OpCode)
	}

	return m, nil
}
