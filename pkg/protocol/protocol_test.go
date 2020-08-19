package protocol_test

import (
	"io"
	"sync"
	"testing"

	"github.com/gohornet/hornet/pkg/protocol"
	"github.com/gohornet/hornet/pkg/protocol/handshake"
	"github.com/gohornet/hornet/pkg/protocol/sting"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
)

type fakeconn struct {
	writer io.WriteCloser
	reader io.ReadCloser
}

func (f fakeconn) Read(p []byte) (n int, err error) {
	return f.reader.Read(p)
}

func (f fakeconn) Write(p []byte) (n int, err error) {
	return f.writer.Write(p)
}

func (f fakeconn) Close() error {
	if err := f.writer.Close(); err != nil {
		return err
	}
	if err := f.reader.Close(); err != nil {
		return err
	}
	return nil
}

func newFakeConn() *fakeconn {
	r, w := io.Pipe()
	return &fakeconn{writer: w, reader: r}
}

func consume(t *testing.T, p *protocol.Protocol, conn io.Reader, expectedLength int) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := make([]byte, expectedLength)
		read, err := conn.Read(data)
		assert.NoError(t, err)
		assert.Equal(t, expectedLength, read)
		p.Receive(data[:read])
	}()
	return &wg
}

func TestProtocol_Handshaked(t *testing.T) {
	conn := newFakeConn()
	defer conn.Close()
	p := protocol.New(conn)
	var handshaked bool
	p.Events.HandshakeCompleted.Attach(events.NewClosure(func() {
		handshaked = true
	}))

	p.Handshaked()
	p.Handshaked()

	assert.True(t, handshaked, "protocol should be handshaked after calling Handshaked() twice")
}

func TestProtocol_Receive(t *testing.T) {
	conn := newFakeConn()
	defer conn.Close()
	p := protocol.New(conn)

	var handshakeMessageReceived bool
	p.Events.Received[handshake.HandshakeMessageDefinition.ID].Attach(events.NewClosure(func(data []byte) {
		handshakeMessageReceived = true
	}))

	handshakeMsg, err := handshake.NewHandshakeMessage(protocol.SupportedFeatureSets, 100, make([]byte, 49), 14)
	assert.NoError(t, err)

	wg := consume(t, p, conn, len(handshakeMsg))
	_, err = conn.Write(handshakeMsg)
	assert.NoError(t, err)

	wg.Wait()
	assert.True(t, handshakeMessageReceived)
}

func TestProtocol_Send(t *testing.T) {
	conn := newFakeConn()
	defer conn.Close()
	p := protocol.New(conn)

	var handshakeMessageSent bool
	p.Events.Sent[handshake.HandshakeMessageDefinition.ID].Attach(events.NewClosure(func() {
		handshakeMessageSent = true
	}))

	handshakeMsg, err := handshake.NewHandshakeMessage(protocol.SupportedFeatureSets, 100, make([]byte, 49), 14)
	assert.NoError(t, err)

	wg := consume(t, p, conn, len(handshakeMsg))
	assert.NoError(t, p.Send(handshakeMsg))

	wg.Wait()
	assert.True(t, handshakeMessageSent)
}

func TestProtocol_Supports(t *testing.T) {
	p := &protocol.Protocol{}
	p.FeatureSet = sting.FeatureSet
	assert.True(t, p.Supports(sting.FeatureSet))
	assert.False(t, p.Supports(243))
}
