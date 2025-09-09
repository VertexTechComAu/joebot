package server

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/harmonicinc-com/joebot/models"
	"github.com/harmonicinc-com/joebot/repository"
	"github.com/harmonicinc-com/joebot/sshconnect"
	"github.com/harmonicinc-com/joebot/utils"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
)

type Server struct {
	logger *logrus.Logger

	portsManager *utils.PortsManager
	gostTunnels  []*GostTunnel
	tcpListener  net.Listener
	tlsCert string
	tlsKey string

	sync.RWMutex         // Mutex lock for creating tunnel
	gostTunnelStartIndex int
	db                   *sqlx.DB

	clients         []*Client
	clientsListLock chan bool

	ctx  context.Context
	stop context.CancelFunc

	UserRepo repository.UserRepository
}

func NewServer(logger *logrus.Logger, tlsCert string, tlsKey string) *Server {
	if logger == nil {
		logger = logrus.New()
	}

	server := &Server{}
	server.logger = logger
	server.portsManager = utils.NewPortsManager()
	server.gostTunnels = []*GostTunnel{}
	server.gostTunnelStartIndex = 0
	server.tlsCert = tlsCert
	server.tlsKey = tlsKey
	server.InitDB()

	server.clientsListLock = make(chan bool, 1)
	server.clientsListLock <- true

	server.ctx, server.stop = context.WithCancel(context.Background())

	logger.Info("Init new server")
	return server
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

type UserInfoResponse struct {
	Username      string `json:"username"`
	IsAdmin       bool   `json:"is_admin"`
	IpWhitelisted string `json:"ip_whitelisted"`
}

func (server *Server) GetAllUser() ([]*UserInfoResponse, error) {
	userRepo := repository.New(server.db)
	userResp, err := userRepo.GetAll(context.Background())
	if err != nil {
		return []*UserInfoResponse{}, err
	}

	userInfoResponses := make([]*UserInfoResponse, 0, len(userResp))
	for i := range userResp {
		userInfoResponses = append(userInfoResponses, &UserInfoResponse{
			Username:      userResp[i].Username,
			IsAdmin:       userResp[i].IsAdmin,
			IpWhitelisted: userResp[i].IpWhitelisted,
		})
	}

	return userInfoResponses, nil
}

func (server *Server) GetUserByToken(token string) (*UserInfoResponse, error) {
	userRepo := repository.New(server.db)
	userResp, err := userRepo.GetUserByToken(context.Background(), token)
	if err != nil {
		return nil, err
	}

	return &UserInfoResponse{
		Username:      userResp.Username,
		IsAdmin:       userResp.IsAdmin,
		IpWhitelisted: userResp.IpWhitelisted,
	}, nil
}

func (server *Server) GetUserIPWhitelisted(token string) ([]string, error) {
	userRepo := repository.New(server.db)
	userResp, err := userRepo.GetUserByToken(context.Background(), token)
	if err != nil {
		return nil, err
	}

	ipWhitelisted := userResp.IpWhitelisted
	if ipWhitelisted == "" {
		return nil, errors.New("No IP Whitelisted")
	}

	ipList := []string{}
	for _, ip := range utils.SplitString(ipWhitelisted, ",") {
		ipList = append(ipList, ip)
	}

	return ipList, nil
}

func (server *Server) CreateUser(user models.UserInfo) error {
	userRepo := repository.New(server.db)
	userResp, err := userRepo.GetUserByUsername(context.Background(), user.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// User not found, proceed to create
			if err := userRepo.Create(context.Background(), &user); err != nil {
				return errors.Wrap(err, "Failed to create user")
			}
			server.logger.Info("User Created: " + user.Username)
			return nil
		} else {
			return errors.Wrap(err, "Error fetching user")
		}
	} else {
		// User already exists
		return errors.New("User already exists: " + userResp.Username)
	}
}

func (server *Server) UserLogin(ipAddress string, username, password string) (models.UserResponse, error) {
	userRepo := repository.New(server.db)
	userResp, err := userRepo.GetUserByUserPassword(context.Background(), username, password)
	if err != nil {
		return models.UserResponse{}, err
	}

	err = userRepo.UpdateUserIPWhitelist(context.Background(), userResp.Username, ipAddress)
	if err != nil {
		return models.UserResponse{}, err
	}

	return *userResp, nil
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

func (server *Server) Start(port int) error {
	var err error

	//Setup 100 Gost SSH Tunnel Services
	for i := 0; i < 30; i++ {
		freePort, err := server.portsManager.ReservePort()
		if err != nil {
			err = errors.Wrap(err, "Unable to find port for Gost Tunnel Server")
			server.logger.Error(err)
			return err
		}
		gostTunnel := NewGostTunnel(freePort)
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

	rawListener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		err = errors.Wrap(err, "Unable to start server")
		server.logger.Error(err)
		return err
	}

	// ----- TLS upgrade (only when flgas are provided) ----
	if server.tlsCert != "" && server.tlsKey != "" {
		cert, err := tls.LoadX509KeyPair(server.tlsCert, server.tlsKey)
		if err != nil {
			return errors.Wrap(err, "invalid TLS cert/key")
		}

		cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
		server.tcpListener = tls.NewListener(rawListener, cfg)
		server.logger.Infof("Control channel listening with TLS on :%d", port)
	} else {
		server.tcpListener = rawListener
		server.logger.Infof("Control channel listening without TLS on :%d", port)
	}

	go func() {
		for {
			select {
			case <-server.ctx.Done():
				server.logger.Info("Stop Accepting New Incomming Connection")
				return
			default:
				conn, err := server.tcpListener.Accept()
				if err != nil {
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
	// sshResults := []sshconnect.SSHResult{}
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

func (server *Server) InitDB() {
	db, err := sqlx.Open("sqlite", "joebot.db")
	if err != nil {
		return
	}
	server.db = db

	schema := `
		CREATE TABLE IF NOT EXISTS user_info (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			is_admin BOOLEAN NOT NULL,
			token TEXT,
			ip_whitelisted TEXT
		);`

	_, err = db.Exec(schema)
	if err != nil {
		return
	}

	// init admin user if not exist
	adminUser := models.UserInfo{
		Username:      "joebot",
		Password:      "joebot123",
		IsAdmin:       true,
		Token:         "",
		IpWhitelisted: "",
	}
	userRepo := repository.New(db)
	server.UserRepo = *userRepo
	_, err = userRepo.GetUserByUsername(context.Background(), adminUser.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("[INFO] Admin user '%s' not found. Proceeding to create...", adminUser.Username)
			if err := userRepo.Create(context.Background(), &adminUser); err != nil {
				log.Fatalf("[ERROR] Failed to create admin user '%s': %v", adminUser.Username, err)
			}
			log.Printf("[INFO] Admin user '%s' created successfully", adminUser.Username)
		} else {
			log.Fatalf("[ERROR] Unexpected error while fetching admin user '%s': %v", adminUser.Username, err)
		}
	} else {
		log.Printf("[INFO] Admin user '%s' already exists", adminUser.Username)
	}

}
