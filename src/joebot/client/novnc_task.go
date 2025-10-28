package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/ginuerzh/gost"
	"github.com/harmonicinc-com/joebot/models"
	"github.com/harmonicinc-com/joebot/task"
	"github.com/harmonicinc-com/joebot/utils"
	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"
)

// NovncTask implements the task.Tasker interface for NoVNC requests.
type NovncTask struct {
	client *Client
	logger *logrus.Logger
	ctx    context.Context
	noTLS  bool
}

// NewNovncTask creates a new task handler for NoVNC, returning the correct task.Tasker interface.
func NewNovncTask(client *Client) task.Tasker {
	return &NovncTask{
		client: client,
		logger: client.logger,
		ctx:    client.ctx,
		noTLS:  client.noTLS,
	}
}

// GetType returns the task type.
func (t *NovncTask) GetType() task.TaskType {
	return task.NovncRequest
}

// The following methods are part of the Tasker interface but are not used by this handler.
func (t *NovncTask) SetRequestParam(param interface{}) task.Tasker { return t }
func (t *NovncTask) SetHandleParam(param interface{}) task.Tasker  { return t }

// Request is part of the Tasker interface but is not implemented by a handler task.
func (t *NovncTask) Request(session *yamux.Session, payload []byte) (net.Conn, error) {
	return nil, fmt.Errorf("Request() not implemented for NovncTask handler")
}

// handle is the core logic that creates and runs the secure WebSocket proxy.
func (t *NovncTask) handle(tunnelInfo models.NovncWebsocketInfo) {
	// 2. Create the handler that will manage the proxying.
	//    In your version of gost, the handler is created directly with the chain.
	handler := gost.TCPDirectForwardHandler(fmt.Sprintf("127.0.0.1:%d", tunnelInfo.VncServerPort))

	var ln net.Listener
	var err error

	if t.noTLS {
		// 4. Create an unencrypted WebSocket listener.
		ln, err = gost.WSListener(
			fmt.Sprintf(":%d", tunnelInfo.NovncWebsocketPort),
			nil,
		)
		if err != nil {
			t.logger.Errorf("NoVNC: Failed to create WS listener: %v", err)
			return
		}
	} else {
		// 3. Manually create a TLS configuration using the client's certificates.
		cert, err := tls.LoadX509KeyPair(t.client.clientCertPath, t.client.clientKeyPath)
		if err != nil {
			t.logger.Errorf("NoVNC: Failed to load key pair: %v", err)
			return
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		// 4. Create the secure WebSocket listener.
		ln, err = gost.WSSListener(
			fmt.Sprintf(":%d", tunnelInfo.NovncWebsocketPort),
			tlsConfig,
			nil,
		)
		if err != nil {
			t.logger.Errorf("NoVNC: Failed to create WSS listener: %v", err)
			return
		}
	}

	// 5. Create and run the gost server.
	//    The Server struct takes the handler as its field.
	gostServer := &gost.Server{Listener: ln}
	t.client.AddTunnel(gostServer)          // Use the correct AddTunnel method from client.go
	defer t.client.RemoveTunnel(gostServer) // Use the correct RemoveTunnel method

	t.logger.Infof("Starting NoVNC Websocket Tunnel on port %d -> tcp://127.0.0.1:%d", tunnelInfo.NovncWebsocketPort, tunnelInfo.VncServerPort)

	// Serve will block until the listener is closed.
	if err := gostServer.Serve(handler); err != nil {
		t.logger.Errorf("NoVNC: Server error: %v", err)
	}

	t.logger.Infof("Closed NoVNC Websocket Tunnel on port %d", tunnelInfo.NovncWebsocketPort)
}

// Handle is the entry point for the task, called by the main client's request handler.
func (t *NovncTask) Handle(body []byte, stream net.Conn) error {
	defer stream.Close()

	var tunnelInfo models.NovncWebsocketInfo
	if err := utils.BytesToStruct(body, &tunnelInfo); err != nil {
		return err
	}

	t.logger.Infof("Received NoVNC WebSocket tunnel request: %+v", tunnelInfo)
	// Start the WebSocket server in a new goroutine so it doesn't block the handler.
	go t.handle(tunnelInfo)

	// Send a confirmation back to the server immediately with the tunnel info.
	return task.SendObject(utils.StructToBytes(tunnelInfo), stream, 5*time.Second)
}
