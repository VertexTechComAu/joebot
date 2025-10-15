package main

import (
	"database/sql"
	"fmt"
	"io" // Use io.ReadAll
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/harmonicinc-com/joebot/client"
	"github.com/harmonicinc-com/joebot/models"
	"github.com/harmonicinc-com/joebot/server"
	"github.com/sirupsen/logrus"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	_ "github.com/mattn/go-sqlite3" // Import the SQLite3 driver
	"golang.org/x/crypto/bcrypt"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("joebot", "A secure command and control server/client for managing machines via a web interface.")

	// --- Server Command and Flags ---
	serverCommand   = app.Command("server", "Run in server mode.")
	serverPort      = serverCommand.Flag("port", "Port for listening to client connections.").Default("13579").Short('p').Int()
	webPortalPort   = serverCommand.Flag("web-portal-port", "Port for the secure web portal.").Default("8080").Short('w').Int()
	sDbPath         = serverCommand.Flag("db-path", "Path to SQLite3 database file.").Default("./joebot.db").String()
	sCaCertPath     = serverCommand.Flag("ca-cert", "Path to the CA certificate file.").String()
	sServerCertPath = serverCommand.Flag("server-cert", "Path to the server certificate file.").String()
	sServerKeyPath  = serverCommand.Flag("server-key", "Path to the server key file.").String()
	sNoTLS          = serverCommand.Flag("no-tls", "Disable TLS encryption.").Bool()
	sVerbose        = serverCommand.Flag("verbose", "Enable verbose logging.").Bool()

	// --- Client Command and Flags ---
	clientCommand                = app.Command("client", "Run in client mode.")
	cServerIP                    = clientCommand.Arg("ip", "Server IP or domain name.").Required().String()
	cServerPort                  = clientCommand.Flag("port", "Server port.").Default("13579").Short('p').Int()
	cAllowedPortRangeLBound      = clientCommand.Flag("allowed-port-lower-bound", "Lower bound of allowed port range for tunnels.").Default("0").Short('l').Int()
	cAllowedPortRangeUBound      = clientCommand.Flag("allowed-port-upper-bound", "Upper bound of allowed port range for tunnels.").Default("65535").Short('u').Int()
	cTags                        = clientCommand.Flag("tag", "Tags for client identification.").Strings()
	cFilebrowserDefaultDirectory = clientCommand.Flag("dir", "Default directory for the File Browser.").Default("/").Short('f').String()
	cCaCertPath                  = clientCommand.Flag("ca-cert", "Path to the CA certificate file.").String()
	cClientCertPath              = clientCommand.Flag("client-cert", "Path to the client certificate file.").String()
	cClientKeyPath               = clientCommand.Flag("client-key", "Path to the client key file.").String()
	cNoTLS                       = clientCommand.Flag("no-tls", "Disable TLS encryption.").Bool()
	cVerbose                     = clientCommand.Flag("verbose", "Enable verbose logging.").Bool()

	// --- User Management Commands ---
	userCommand     = app.Command("user", "Manage web portal users.")
	userDbPath      = userCommand.Flag("db-path", "Path to SQLite3 database file.").Default("./joebot.db").String()
	userAddCmd      = userCommand.Command("add", "Add a new user.")
	userAddUsername = userAddCmd.Arg("username", "Username.").Required().String()
	userAddPassword = userAddCmd.Arg("password", "Password.").Required().String()
	userDelCmd      = userCommand.Command("del", "Delete a user.")
	userDelUsername = userDelCmd.Arg("username", "Username to delete.").Required().String()
)

// initDB connects to the SQLite database and creates the necessary tables if they don't exist.
func initDB(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	// Create users table with a securely hashed password
	usersTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL
	);`
	_, err = db.Exec(usersTableSQL)
	if err != nil {
		return nil, err
	}

	// Create whitelisted_ips table
	whitelistTableSQL := `
	CREATE TABLE IF NOT EXISTS whitelisted_ips (
		ip_address TEXT PRIMARY KEY,
		last_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(whitelistTableSQL)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func main() {
	defer func() {
		fmt.Println("Ended")
	}()

	app.HelpFlag.Short('h')
	command := kingpin.MustParse(app.Parse(os.Args[1:]))

	switch command {
	case serverCommand.FullCommand():
		if err := runServer(); err != nil {
			log.Fatal(err)
		}
	case clientCommand.FullCommand():
		if err := runClient(); err != nil {
			log.Fatal(err)
		}
	case userAddCmd.FullCommand():
		if err := runUserAdd(); err != nil {
			log.Fatal(err)
		}
	case userDelCmd.FullCommand():
		if err := runUserDel(); err != nil {
			log.Fatal(err)
		}
	}
}

func runServer() error {
	if !*sNoTLS && (*sCaCertPath == "" || *sServerCertPath == "" || *sServerKeyPath == "") {
		return fmt.Errorf("FATAL: --ca-cert, --server-cert, and --server-key are required unless --no-tls is specified")
	}
	db, err := initDB(*sDbPath)
	if err != nil {
		return fmt.Errorf("FATAL: Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Println("Successfully connected to the database.")

	logger := logrus.New()
	if *sVerbose {
		logger.SetLevel(logrus.DebugLevel)
	}
	s := server.NewServer(logger, db)
	go s.Start(*serverPort, *sCaCertPath, *sServerCertPath, *sServerKeyPath, *sNoTLS)

	e := buildEchoServer(s, db)
	return startWebPortal(e, *webPortalPort, *sNoTLS, *sServerCertPath, *sServerKeyPath)
}
func buildEchoServer(s *server.Server, db *sql.DB) *echo.Echo {
	e := echo.New()
	v1 := e.Group("/api")

	// BasicAuth middleware using the SQLite database for user validation.
	v1.Use(createAuthMiddleware(db, s))

	// Whitelist middleware
	v1.Use(createWhitelistMiddleware(s))

	webPortalAssetsFS := WebPortalAssetsFS()

	v1.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
	}))
	e.GET("/", makeIndexHandler(webPortalAssetsFS))
	e.GET("/*", echo.WrapHandler(http.FileServer(http.FS(webPortalAssetsFS))))
	v1.GET("/clients", handleGetClients(s))
	v1.POST("/client/:id", handleCreateTunnel(s))
	v1.POST("/bulk-install", handleBulkInstall(s))
	return e
}

func createWhitelistMiddleware(s *server.Server) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			isWhitelisted, err := s.IsIPWhitelisted(c.RealIP())
			if err != nil || !isWhitelisted {
				log.Printf("Blocked unauthorized API request from IP: %s", c.RealIP())
				return c.String(http.StatusUnauthorized, "Unauthorized: Your IP is not whitelisted.")
			}
			return next(c)
		}
	}
}

func makeIndexHandler(webPortalAssetsFS fs.FS) echo.HandlerFunc {
	return func(c echo.Context) error {
		f, err := webPortalAssetsFS.Open("index.html")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		b, err := io.ReadAll(f)
		if err != nil {
			log.Fatal(err)
		}
		return c.HTML(200, string(b))
	}
}

func handleGetClients(s *server.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, s.GetClientsList())
	}
}

func handleCreateTunnel(s *server.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		type msg struct {
			Message string `json:"message"`
		}

		client, err := s.GetClientById(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusNotFound, msg{err.Error()})
		}
		portStr := c.FormValue("target_client_port")
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 {
			return c.JSON(http.StatusBadRequest, msg{"Invalid target_client_port"})
		}

		portTunnelInfo, err := client.CreateTunnel(port)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, msg{err.Error()})
		}

		return c.JSON(http.StatusOK, portTunnelInfo)
	}
}

func handleBulkInstall(s *server.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		json := models.BulkInstallInfo{}

		if err := c.Bind(&json); err != nil {
			return err
		}
		result, err := s.BulkInstallJoebot(json)
		if err != nil {
			return err
		}

		return c.String(http.StatusOK, result)
	}
}

func createAuthMiddleware(db *sql.DB, s *server.Server) echo.MiddlewareFunc {
	return middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
		var passwordHash string
		err := db.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&passwordHash)
		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("Failed login attempt for non-existent user '%s' from IP: %s", username, c.RealIP())
				return false, nil
			}
			log.Printf("ERROR: Database query failed for user '%s': %v", username, err)
			return false, err
		}

		err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
		if err != nil {
			log.Printf("Failed login attempt (wrong password) for user '%s' from IP: %s", username, c.RealIP())
			return false, nil
		}

		clientIP := c.RealIP()
		_, dbErr := db.Exec("INSERT INTO whitelisted_ips (ip_address, last_seen) VALUES (?, CURRENT_TIMESTAMP) ON CONFLICT(ip_address) DO UPDATE SET last_seen = CURRENT_TIMESTAMP;", clientIP)
		if dbErr != nil {
			log.Printf("ERROR: Failed to whitelist IP %s: %v", clientIP, dbErr)
		}
		return true, nil
	})
}

func startWebPortal(e *echo.Echo, port int, noTLS bool, certPath, keyPath string) error {
	log.Printf("Web portal starting on http://0.0.0.0:%d", port)
	if noTLS {
		if err := e.Start(":" + strconv.Itoa(port)); err != nil {
			return fmt.Errorf("FATAL: Could not start web portal: %v", err)
		}
		return nil
	}

	log.Printf("Secure web portal starting on https://0.0.0.0:%d", port)
	if err := e.StartTLS(":"+strconv.Itoa(port), certPath, keyPath); err != nil {
		return fmt.Errorf("FATAL: Could not start secure web portal: %v", err)
	}
	return nil
}

func runClient() error {
	if !*cNoTLS && (*cCaCertPath == "" || *cClientCertPath == "" || *cClientKeyPath == "") {
		return fmt.Errorf("FATAL: --ca-cert, --client-cert, and --client-key are required unless --no-tls is specified")
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)

	logger := logrus.New()
	if *cVerbose {
		logger.SetLevel(logrus.DebugLevel)
	}

	c := client.NewClient(*cServerIP, *cServerPort, *cAllowedPortRangeLBound, *cAllowedPortRangeUBound, *cTags, *cCaCertPath, *cClientCertPath, *cClientKeyPath, *cNoTLS, logger)
	c.FilebrowserDefaultDir = *cFilebrowserDefaultDirectory
	c.Start()
	wg.Wait()
	return nil
}

func runUserAdd() error {
	db, err := initDB(*userDbPath)
	if err != nil {
		return fmt.Errorf("FATAL: Failed to open database: %v", err)
	}
	defer db.Close()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*userAddPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("FATAL: Failed to hash password: %v", err)
	}

	_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", *userAddUsername, string(hashedPassword))
	if err != nil {
		return fmt.Errorf("FATAL: Failed to add user to database: %v", err)
	}
	fmt.Printf("Successfully added user: %s\n", *userAddUsername)
	return nil
}

func runUserDel() error {
	db, err := initDB(*userDbPath)
	if err != nil {
		return fmt.Errorf("FATAL: Failed to open database: %v", err)
	}
	defer db.Close()
	res, err := db.Exec("DELETE FROM users WHERE username = ?", *userDelUsername)
	if err != nil {
		return fmt.Errorf("FATAL: Failed to delete user: %v", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		fmt.Printf("User '%s' not found.\n", *userDelUsername)
	} else {
		fmt.Printf("Successfully deleted user: %s\n", *userDelUsername)
	}
	return nil
}
