package client

import (
	"io"
	"net"
	"strconv"

	"github.com/harmonicinc-com/joebot/task"
	"github.com/harmonicinc-com/joebot/utils"
	"github.com/pkg/errors"
)

type PortTunnelTask struct {
	handleClient *Client
	*task.Task
}

func NewPortTunnelTask(client *Client) *PortTunnelTask {
	return &PortTunnelTask{
		client,
		task.NewTask(client.ctx, task.PortTunnelRequest, client.logger),
	}
}

func (t *PortTunnelTask) Handle(body []byte, stream net.Conn) error {
	defer stream.Close()

	var clientPort int
	err := utils.BytesToStruct(body, &clientPort)
	if err != nil {
		return errors.Wrap(err, "Unable to decode client port from stream")
	}

	if clientPort <= 0 {
		return errors.New("Invalid client port received")
	}

	localConn, err := net.Dial("tcp", "localhost:"+strconv.Itoa(clientPort))
	if err != nil {
		return errors.Wrap(err, "Failed to connect to local service on port "+strconv.Itoa(clientPort))
	}
	defer localConn.Close()

	errc := make(chan error, 1)
	go func() {
		_, err := io.Copy(localConn, stream)
		errc <- err
	}()
	go func() {
		_, err := io.Copy(stream, localConn)
		errc <- err
	}()

	<-errc
	return nil
}
