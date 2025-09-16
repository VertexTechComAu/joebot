package server

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/harmonicinc-com/joebot/utils"

	"github.com/harmonicinc-com/joebot/handler"
	"github.com/harmonicinc-com/joebot/models"
	"github.com/harmonicinc-com/joebot/task"
	"github.com/hashicorp/yamux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Client struct {
	logger *logrus.Logger

	ID      string
	conn    *net.Conn
	server  *Server
	session *yamux.Session
	streams []*yamux.Stream

	ctx  context.Context
	stop context.CancelFunc

	Info models.ClientInfo
}

func NewClient(id string, server *Server, conn *net.Conn, logger *logrus.Logger) *Client {
	if logger == nil {
		logger = logrus.New()
	}

	client := &Client{}
	client.ID = id
	client.logger = logger
	client.conn = conn
	client.server = server
	client.ctx, client.stop = context.WithCancel(context.Background())

	client.Info = models.ClientInfo{}
	client.Info.ID = id
	client.Info.Tags = []string{}
	client.Info.PortTunnels = []models.PortTunnelInfo{}

	logger.Info("Init new client")
	return client
}

func (client *Client) UpdateInfo(info models.ClientInfo) {
	if info.Tags != nil {
		client.Info.Tags = info.Tags
	}
	client.Info.IP = info.IP
	client.Info.HostName = info.HostName
	client.Info.Username = info.Username
}

func (client *Client) ExitIfError(err error, message string) bool {
	if err != nil {
		if message != "" {
			err = errors.Wrap(err, message)
		}

		client.logger.WithField("Client ID", client.ID).Error(err)
		client.Stop()
		client.server.RemoveClient(client.ID)

		return true
	}

	return false
}

func (client *Client) Start() {
	// Setup server side of yamux
	session, err := yamux.Server(*(client.conn), nil)
	if client.ExitIfError(err, "Unable to create yamux session") {
		return
	}
	client.session = session

	inHandler := handler.NewIncomingRequestHandler(client.ctx, client.session, client.logger)
	inHandler.OnError(func(err error) {
		client.ExitIfError(err, "")
	})
	inHandler.RegisterTask(NewClientInfoUpdateTask(client))
	inHandler.Start()

	if _, err := client.CreateSSHTunnel(); err != nil {
		client.logger.Info(errors.Wrap(err, "Failed To Create SSH Tunnel"))
	}
	if _, err = client.CreateGottyWebTerminal(); err != nil {
		client.logger.Info(errors.Wrap(err, "Failed To Create Gotty Web Terminal Tunnel"))
	}
	if _, err = client.CreateNovncWebsocketTunnel(5901); err != nil {
		client.logger.Info(errors.Wrap(err, "Failed To Create NoVNC Websocket Tunnel"))
	}
	if _, err = client.CreateFilebrowser(); err != nil {
		client.logger.Info(errors.Wrap(err, "Failed To Create Web filebrowser"))
	}
}

func (client *Client) CreateSSHTunnel() (models.PortTunnelInfo, error) {
	var err error
	var sshTunnel models.PortTunnelInfo
	if client.Info.SSHTunnel != nil {
		return sshTunnel, errors.New("Failed to create SSH tunnel | tunnel already exists")
	}

	client.logger.WithField("Client ID", client.ID).Info("Creating SSH Tunnel")
	stream, err := task.NewTask(client.ctx, task.SSHTunnelRequest, client.logger).Request(client.session, []byte{})
	if err != nil {
		return sshTunnel, errors.Wrap(err, "SSH Tunnel Request Failed")
	}
	body, err := task.ReceiveObject(stream, 10*time.Second)
	if err != nil {
		return sshTunnel, errors.Wrap(err, "Failed To Locate Client's SSH")
	}
	err = utils.BytesToStruct(body, &sshTunnel)
	if err != nil {
		return sshTunnel, err
	}

	portTunnelInfo, err := client.CreateTunnel(sshTunnel.ClientPort)
	if err != nil {
		return sshTunnel, errors.Wrap(err, "Failed To Create SSH Tunnel")
	}

	client.Info.SSHTunnel = &portTunnelInfo
	return sshTunnel, err
}

func (client *Client) CreateFilebrowser() (models.FilebrowserInfo, error) {
	var err error
	var fbInfo models.FilebrowserInfo
	if client.Info.FilebrowserInfo != nil {
		return fbInfo, errors.New("Failed to create web filebrowser | service already exists")
	}

	client.logger.WithField("Client ID", client.ID).Info("Creating web filebrowser")
	stream, err := task.NewTask(client.ctx, task.FilebrowserRequest, client.logger).Request(client.session, []byte{})
	if err != nil {
		return fbInfo, err
	}

	body, err := task.ReceiveObject(stream, time.Second*10)
	if err != nil {
		return fbInfo, errors.Wrap(err, "Failed To Receive Web filebrowser From Client")
	}
	if err = utils.BytesToStruct(body, &fbInfo); err != nil {
		return fbInfo, err
	}

	portTunnelInfo, err := client.CreateTunnel(fbInfo.FilebrowserPort)
	if err != nil {
		return fbInfo, errors.Wrap(err, "Failed To Create Tunnel To Web filebrowser")
	}
	fbInfo.PortTunnelOnHost = portTunnelInfo

	client.Info.FilebrowserInfo = &fbInfo
	return fbInfo, nil
}

func (client *Client) CreateGottyWebTerminal() (models.GottyWebTerminalInfo, error) {
	var err error
	var wtInfo models.GottyWebTerminalInfo
	if client.Info.GottyWebTerminalInfo != nil {
		return wtInfo, errors.New("Failed to create Gotty web terminal tunnel | service already exists")
	}

	client.logger.WithField("Client ID", client.ID).Info("Creating gotty Web Terminal")
	stream, err := task.NewTask(client.ctx, task.GottyWebTerminalRequest, client.logger).Request(client.session, []byte{})
	if err != nil {
		return wtInfo, err
	}

	body, err := task.ReceiveObject(stream, time.Second*10)
	if err != nil {
		return wtInfo, errors.Wrap(err, "Failed To Receive Web Terminal Info From Client")
	}
	if err = utils.BytesToStruct(body, &wtInfo); err != nil {
		return wtInfo, err
	}

	portTunnelInfo, err := client.CreateTunnel(wtInfo.GottyWebTerminalPort)
	if err != nil {
		return wtInfo, errors.Wrap(err, "Failed To Create Tunnel To Gotty Web Terminal")
	}
	wtInfo.PortTunnelOnHost = portTunnelInfo

	client.Info.GottyWebTerminalInfo = &wtInfo
	return wtInfo, nil
}

func (client *Client) CreateNovncWebsocketTunnel(clientVncPort int) (models.NovncWebsocketInfo, error) {
	var err error
	var novncWebsocketInfo models.NovncWebsocketInfo
	if client.Info.NovncWebsocketInfo != nil {
		return novncWebsocketInfo, errors.New("Failed to create novnc tunnel | service already exists")
	}

	client.logger.WithField("Client ID", client.ID).Infof("Creating novnc Websocket Tunnel | Client VNC Port %d", clientVncPort)

	novncWebsocketInfo.NovncWebsocketPort = clientVncPort
	stream, err := task.NewTask(client.ctx, task.NovncRequest, client.logger).Request(client.session, utils.StructToBytes(novncWebsocketInfo))
	if err != nil {
		return novncWebsocketInfo, err
	}

	body, err := task.ReceiveObject(stream, time.Second*10)
	if err != nil {
		return novncWebsocketInfo, err
	}
	err = utils.BytesToStruct(body, &novncWebsocketInfo)
	if err != nil {
		return novncWebsocketInfo, err
	}

	portTunnelInfo, err := client.CreateTunnel(novncWebsocketInfo.NovncWebsocketPort)
	if err != nil {
		return novncWebsocketInfo, errors.Wrap(err, "Failed To Create Tunnel To NoVNC Websocket")
	}
	novncWebsocketInfo.PortTunnelOnHost = portTunnelInfo

	client.Info.NovncWebsocketInfo = &novncWebsocketInfo
	return novncWebsocketInfo, nil
}

func (client *Client) forwardConnection(browserConn net.Conn, clientPort int) {
	defer browserConn.Close()

	var ip string
	if addr, ok := browserConn.RemoteAddr().(*net.TCPAddr); ok {
		ip = addr.IP.String()
	} else {
		remoteAddr := browserConn.RemoteAddr().String()
		host, _, err := net.SplitHostPort(remoteAddr)
		if err != nil {
			ip = remoteAddr
		} else {
			ip = host
		}
	}

	log.Printf("DEBUG: Checking IP whitelist for tunnel connection from %s", ip)
	isWhitelisted, err := client.server.IsIPWhitelisted(ip)
	if err != nil {
		log.Printf("DEBUG: Error checking IP whitelist for %s: %v", ip, err)
		return
	}

	if !isWhitelisted {
		log.Printf("DEBUG: Blocked unauthorized tunnel request from IP: %s", ip)
		return
	}
	log.Printf("DEBUG: Allowed tunnel connection from whitelisted IP: %s", ip)

	forwardStream, err := client.session.Open()
	if err != nil {
		client.logger.Errorf("Failed to open yamux stream for forwarding: %v", err)
		return
	}
	defer forwardStream.Close()

	// 1. Write Task Type
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, uint32(task.PortTunnelRequest))
	_, err = forwardStream.Write(bs)
	if err != nil {
		client.logger.Errorf("Failed to write task type for forwarding: %v", err)
		return
	}

	// 2. Write Payload (clientPort)
	payload := utils.StructToBytes(clientPort)
	bs = make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, uint64(len(payload)))
	_, err = forwardStream.Write(bs)
	if err != nil {
		client.logger.Errorf("Failed to write payload length for forwarding: %v", err)
		return
	}
	_, err = forwardStream.Write(payload)
	if err != nil {
		client.logger.Errorf("Failed to write payload for forwarding: %v", err)
		return
	}

	// 3. Pipe data
	errc := make(chan error, 1)
	go func() {
		_, err := io.Copy(forwardStream, browserConn)
		errc <- err
	}()
	go func() {
		_, err := io.Copy(browserConn, forwardStream)
		errc <- err
	}()
	<-errc
}

func (client *Client) CreateTunnel(clientPort int) (models.PortTunnelInfo, error) {
	var err error
	var tunnel models.PortTunnelInfo

	client.logger.WithField("Client ID", client.ID).Infof("Creating Tunnel To Client Port %d", clientPort)
	//Check if the client port is already tunnelled
	for _, t := range client.Info.PortTunnels {
		if t.ClientPort == clientPort {
			return t, nil
		}
	}

	serverPort, err := client.server.portsManager.ReservePort()
	if err != nil {
		return tunnel, err
	}

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(serverPort))
	if err != nil {
		client.server.portsManager.ReleasePort(serverPort)
		return tunnel, errors.Wrap(err, "Failed to create listener on server port")
	}
	client.server.AddListener(serverPort, listener)
	client.logger.WithField("Client ID", client.ID).Infof("Server listening on port %d for tunnel to client port %d", serverPort, clientPort)

	go func() {
		defer client.server.RemoveListener(serverPort)
		for {
			browserConn, err := listener.Accept()
			if err != nil {
				// Listener was closed, exit the loop
				return
			}
			go client.forwardConnection(browserConn, clientPort)
		}
	}()

	tunnel.ServerPort = serverPort
	tunnel.ClientPort = clientPort
	client.Info.PortTunnels = append(client.Info.PortTunnels, tunnel)
	return tunnel, nil
}

func (client *Client) Stop() error {
	defer func() {
		for _, t := range client.Info.PortTunnels {
			client.server.portsManager.ReleasePort(t.ServerPort)
		}
		client.Info.PortTunnels = []models.PortTunnelInfo{}
	}()

	client.stop()
	if client.session.IsClosed() {
		return nil
	}

	client.session.GoAway()
	for _, stream := range client.streams {
		err := stream.Close()
		if err != nil {
			err = errors.Wrap(err, "Unable to stop client's stream")
			client.logger.Error(err)
		}
	}
	client.session.Close()
	return (*client.conn).Close()
}
