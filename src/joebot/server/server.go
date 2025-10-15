package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/harmonicinc-com/joebot/models"
	"github.com/harmonicinc-com/joebot/sshconnect"
	"github.com/harmonicinc-com/joebot/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
)

type Server struct {
	logger *logrus.Logger
	db     *sql.DB

	portsManager *utils.PortsManager
	gostTunnels  []*GostTunnel
	tcpListener  net.Listener

	sync.RWMutex
	gostTunnelStartIndex int

	clients         []*Client
	clientsListLock chan bool

	listeners     map[int]*http.Server
	listenersLock sync.Mutex

	ctx  context.Context
	stop context.CancelFunc
}

func NewServer(logger *logrus.Logger, db *sql.DB) *Server {
	if logger == nil {
		logger = logrus.New()
	}

	server := &Server{}
	server.logger = logger
	server.db = db
	server.portsManager = utils.NewPortsManager()
	server.gostTunnels = []*GostTunnel{}
	server.gostTunnelStartIndex = 0
	server.listeners = make(map[int]*http.Server)

	server.clientsListLock = make(chan bool, 1)
	server.clientsListLock <- true

	server.ctx, server.stop = context.WithCancel(context.Background())

	logger.Info("Init new server")
	return server
}

func (server *Server) AddListener(port int, listener *http.Server) {
	server.listenersLock.Lock()
	defer server.listenersLock.Unlock()
	server.listeners[port] = listener
}

func (server *Server) RemoveListener(port int) {
	server.listenersLock.Lock()
	defer server.listenersLock.Unlock()
	if listener, ok := server.listeners[port]; ok {
		listener.Shutdown(context.Background())
		delete(server.listeners, port)
	}
}

func (server *Server) IsIPWhitelisted(ip string) (bool, error) {
	var exists bool
	err := server.db.QueryRow("SELECT EXISTS(SELECT 1 FROM whitelisted_ips WHERE ip_address = ?)", ip).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	return exists, nil
}

func (server *Server) RemoveClient(clientID string) (*Client, error) {
	<-server.clientsListLock
	defer func() { server.clientsListLock <- true }()

	var result *Client
	for i, client := range server.clients {
		if client.ID == clientID {
			server.clients = append(server.clients[:i], server.clients[i+1:]...)
			result = client
			client.Stop()
		}
	}

	var err error
	if result == nil {
		err = errors.New("Failed To Remove Client With ID: " + clientID)
		server.logger.Error(err)
	} else {
		server.logger.Info("Server Removed Client With ID: " + clientID)
	}
	return result, err
}

func (server *Server) GetClientsList() models.ClientCollection {
	var clientCollection models.ClientCollection
	clientCollection.Clients = []models.ClientInfo{}
	for _, client := range server.clients {
		clientCollection.Clients = append(clientCollection.Clients, client.Info)
	}
	return clientCollection
}

func (server *Server) GetClientById(id string) (*Client, error) {
	for _, client := range server.clients {
		if client.ID == id {
			return client, nil
		}
	}
	return nil, errors.New("Client ID Not Found: " + id)
}

func (server *Server) AddClient(client *Client) {
	<-server.clientsListLock
	defer func() { server.clientsListLock <- true }()
	server.clients = append(server.clients, client)
	server.logger.Info("Server Added Client With ID: " + client.ID)
}

func (server *Server) Stop() error {
	server.stop()
	for _, client := range server.clients {
		server.RemoveClient(client.ID)
	}
	server.gostTunnels = []*GostTunnel{}
	server.listenersLock.Lock()
	defer server.listenersLock.Unlock()
	for port, listener := range server.listeners {
		listener.Shutdown(context.Background())
		delete(server.listeners, port)
	}
	return server.tcpListener.Close()
}

func (server *Server) GetTunnelService() *GostTunnel {
	server.Lock()
	defer server.Unlock()
	result := server.gostTunnels[server.gostTunnelStartIndex]
	server.gostTunnelStartIndex++
	if server.gostTunnelStartIndex >= len(server.gostTunnels) {
		server.gostTunnelStartIndex = 0
	}
	return result
}

func (server *Server) Start(port int, caCertPath, serverCertPath, serverKeyPath string, noTLS bool) error {
	// Setup Gost SSH Tunnel Services
	for i := 0; i < 30; i++ {
		freePort, err := server.portsManager.ReservePort()
		if err != nil {
			err = errors.Wrap(err, "Unable to find port for Gost Tunnel Server")
			server.logger.Error(err)
			return err
		}
		gostTunnel := NewGostTunnel(freePort, server, noTLS)
		server.gostTunnels = append(server.gostTunnels, gostTunnel)
		go func(server *Server, gostTunnel *GostTunnel) {
			server.logger.Info("Starting Gost Reverse Tunnel On Port: " + strconv.Itoa(gostTunnel.Port))
			err := gostTunnel.Serve()
			if err != nil {
				server.logger.Errorln("Starting Gost Reverse Tunnel On Port: " + strconv.Itoa(gostTunnel.Port))
				gostTunnel.Stop()
				server.Stop()
				server.portsManager.ReleasePort(gostTunnel.Port)
			}
		}(server, gostTunnel)
	}

	var listener net.Listener
	var err error
	if noTLS {
		// Listen for incoming connections without TLS
		listener, err = net.Listen("tcp", ":"+strconv.Itoa(port))
		if err != nil {
			err = errors.Wrap(err, "Unable to start server")
			server.logger.Error(err)
			return err
		}
		log.Printf("Joebot server listening on port %d", port)
	} else {
		// Load server's certificate and private key
		cert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
		if err != nil {
			log.Fatalf("server: loadkeys: %s", err)
		}

		// Create a CA certificate pool and add ca.crt to it
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			log.Fatal("server: read ca cert: ", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		// Create a TLS configuration with client authentication
		config := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    caCertPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}

		// Listen for incoming connections using the TLS config
		listener, err = tls.Listen("tcp", ":"+strconv.Itoa(port), config)
		if err != nil {
			err = errors.Wrap(err, "Unable to start secure server")
			server.logger.Error(err)
			return err
		}
		log.Printf("Joebot server listening securely on port %d", port)
	}
	server.tcpListener = listener

	go func() {
		for {
			select {
			case <-server.ctx.Done():
				server.logger.Info("Stop Accepting New Incoming Connection")
				return
			default:
				conn, err := server.tcpListener.Accept()
				if err != nil {
					// Check if the error is due to the listener being closed.
					if errors.Is(err, net.ErrClosed) {
						server.logger.Info("Listener closed, stopping accept loop.")
						return
					}
					err = errors.Wrap(err, "Unable to accept incoming connection")
					server.logger.Error(err)
					continue
				}

				client := NewClient(uuid.NewV4().String(), server, &conn, server.logger)
				server.AddClient(client)
				go func(client *Client) { client.Start() }(client)
			}
		}
	}()

	return nil
}

func (server *Server) BulkInstallJoebot(info models.BulkInstallInfo) (string, error) {
	sshHosts := []sshconnect.SSHHost{}
	if len(info.Addresses) == 0 {
		return "", errors.New("Empty Address List")
	}
	for _, addr := range info.Addresses {
		sshHosts = append(sshHosts, sshconnect.SSHHost{
			Host:     addr.IP,
			Port:     addr.Port,
			Username: info.Username,
			Password: info.Password,
			Key:      info.Key,
		})
	}

	var cipherList []string
	chLimit := make(chan bool, 10)
	chs := make([]chan sshconnect.SSHResult, len(sshHosts))
	startTime := time.Now()
	log.Println("Multissh start")

	sshResults := []sshconnect.SSHResult{}
	limitFunc := func(chLimit chan bool, ch chan sshconnect.SSHResult, host sshconnect.SSHHost) {
		s1 := rand.NewSource(time.Now().UnixNano())
		r1 := rand.New(s1)
		dstFilePath := "/tmp/joebot_remote_installed_" + strconv.Itoa(r1.Intn(99999))

		cmds := []string{
			"chmod +x " + dstFilePath,
			"nohup " + dstFilePath + " client -p " + strconv.Itoa(info.JoebotServerPort) + " " + info.JoebotServerIP + " &",
		}

		sshconnect.UploadMyself(dstFilePath, host.Username, host.Password, host.Host, host.Key, host.CmdList, host.Port, 120, cipherList, host.LinuxMode, ch)
		sshconnect.Dossh(host.Username, host.Password, host.Host, host.Key, cmds, host.Port, 120, cipherList, false, ch)

		<-chLimit
	}
	for i, host := range sshHosts {
		chs[i] = make(chan sshconnect.SSHResult, 2)
		chLimit <- true
		go limitFunc(chLimit, chs[i], host)
	}
	for _, ch := range chs {
		res := <-ch
		if res.Result != "" {
			sshResults = append(sshResults, res)
		}
		res = <-ch
		if res.Result != "" {
			sshResults = append(sshResults, res)
		}
	}
	endTime := time.Now()
	log.Printf("Multissh finished. Process time %s. Number of active ip is %d", endTime.Sub(startTime), len(sshHosts))

	for _, sshResult := range sshResults {
		fmt.Println("host: ", sshResult.Host)
		fmt.Println("========= Result =========")
		fmt.Println(sshResult.Result)
	}

	return "", nil
}
