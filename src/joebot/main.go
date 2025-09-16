package main

import (
	"database/sql"
	"fmt"
	"io" // Use io.ReadAll
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
	sCaCertPath     = serverCommand.Flag("ca-cert", "Path to the CA certificate file.").Required().String()
	sServerCertPath = serverCommand.Flag("server-cert", "Path to the server certificate file.").Required().String()
	sServerKeyPath  = serverCommand.Flag("server-key", "Path to the server key file.").Required().String()

	// --- Client Command and Flags ---
	clientCommand                = app.Command("client", "Run in client mode.")
	cServerIP                    = clientCommand.Arg("ip", "Server IP or domain name.").Required().String()
	cServerPort                  = clientCommand.Flag("port", "Server port.").Default("13579").Short('p').Int()
	cAllowedPortRangeLBound      = clientCommand.Flag("allowed-port-lower-bound", "Lower bound of allowed port range for tunnels.").Default("0").Short('l').Int()
	cAllowedPortRangeUBound      = clientCommand.Flag("allowed-port-upper-bound", "Upper bound of allowed port range for tunnels.").Default("65535").Short('u').Int()
	cTags                        = clientCommand.Flag("tag", "Tags for client identification.").Strings()
	cFilebrowserDefaultDirectory = clientCommand.Flag("dir", "Default directory for the File Browser.").Default("/").Short('f').String()
	cCaCertPath                  = clientCommand.Flag("ca-cert", "Path to the CA certificate file.").Required().String()
	cClientCertPath              = clientCommand.Flag("client-cert", "Path to the client certificate file.").Required().String()
	cClientKeyPath               = clientCommand.Flag("client-key", "Path to the client key file.").Required().String()

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
	// --- Server Mode Execution ---
	case serverCommand.FullCommand():
		db, err := initDB(*sDbPath)
		if err != nil {
			log.Fatalf("FATAL: Failed to initialize database: %v", err)
		}
		defer db.Close()
		log.Println("Successfully connected to the database.")

		s := server.NewServer(logrus.New(), db)
		go s.Start(*serverPort, *sCaCertPath, *sServerCertPath, *sServerKeyPath)

		e := echo.New()
		v1 := e.Group("/api")

		// BasicAuth middleware using the SQLite database for user validation.
		v1.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			var passwordHash string
			err := db.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&passwordHash)
			if err != nil {
				if err == sql.ErrNoRows {
					log.Printf("Failed login attempt for non-existent user '%s' from IP: %s", username, c.RealIP())
					return false, nil // User not found
				}
				log.Printf("ERROR: Database query failed for user '%s': %v", username, err)
				return false, err // Database error
			}

			err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
			if err != nil {
				log.Printf("Failed login attempt (wrong password) for user '%s' from IP: %s", username, c.RealIP())
				return false, nil // Passwords do not match
			}

			// Success! Whitelist the IP.
			clientIP := c.RealIP()
			_, dbErr := db.Exec("INSERT INTO whitelisted_ips (ip_address, last_seen) VALUES (?, CURRENT_TIMESTAMP) ON CONFLICT(ip_address) DO UPDATE SET last_seen = CURRENT_TIMESTAMP;", clientIP)
			if dbErr != nil {
				log.Printf("ERROR: Failed to whitelist IP %s: %v", clientIP, dbErr)
			}
			// log.Printf("Successful login for user '%s' from IP: %s", username, clientIP)
			return true, nil
		}))

		v1.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				isWhitelisted, err := s.IsIPWhitelisted(c.RealIP())
				if err != nil || !isWhitelisted {
					log.Printf("Blocked unauthorized API request from IP: %s", c.RealIP())
					return c.String(http.StatusUnauthorized, "Unauthorized: Your IP is not whitelisted.")
				}
				return next(c)
			}
		})

		webPortalAssetsFS := WebPortalAssetsFS()

		v1.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
		}))
		e.GET("/", func(c echo.Context) error {
			f, err := webPortalAssetsFS.Open("index.html")
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			b, err := io.ReadAll(f) // Correctly read from the file handle
			if err != nil {
				log.Fatal(err)
			}
			return c.HTML(200, string(b))
		})
		e.GET("/*", echo.WrapHandler(http.FileServer(http.FS(webPortalAssetsFS))))
		v1.GET("/clients", func(c echo.Context) error {
			return c.JSON(http.StatusOK, s.GetClientsList())
		})
		v1.POST("/client/:id", func(c echo.Context) error {
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
		})
		v1.POST("/bulk-install", func(c echo.Context) error {
			json := models.BulkInstallInfo{}

			if err := c.Bind(&json); err != nil {
				return err
			}
			result, err := s.BulkInstallJoebot(json)
			if err != nil {
				return err
			}

			return c.String(http.StatusOK, result)
		})

		log.Printf("Secure web portal starting on https://0.0.0.0:%d", *webPortalPort)
		if err := e.StartTLS(":"+strconv.Itoa(*webPortalPort), *sServerCertPath, *sServerKeyPath); err != nil {
			log.Fatal("FATAL: Could not start secure web portal: ", err)
		}

	case clientCommand.FullCommand():
		wg := &sync.WaitGroup{}
		wg.Add(1)
		c := client.NewClient(*cServerIP, *cServerPort, *cAllowedPortRangeLBound, *cAllowedPortRangeUBound, *cTags, *cCaCertPath, *cClientCertPath, *cClientKeyPath, nil)
		c.FilebrowserDefaultDir = *cFilebrowserDefaultDirectory
		c.Start()
		wg.Wait()

	case userAddCmd.FullCommand():
		db, err := initDB(*userDbPath)
		if err != nil {
			log.Fatalf("FATAL: Failed to open database: %v", err)
		}
		defer db.Close()

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*userAddPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("FATAL: Failed to hash password: %v", err)
		}

		_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", *userAddUsername, string(hashedPassword))
		if err != nil {
			log.Fatalf("FATAL: Failed to add user to database: %v", err)
		}
		fmt.Printf("Successfully added user: %s\n", *userAddUsername)

	case userDelCmd.FullCommand():
		db, err := initDB(*userDbPath)
		if err != nil {
			log.Fatalf("FATAL: Failed to open database: %v", err)
		}
		defer db.Close()
		res, err := db.Exec("DELETE FROM users WHERE username = ?", *userDelUsername)
		if err != nil {
			log.Fatalf("FATAL: Failed to delete user: %v", err)
		}
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			fmt.Printf("User '%s' not found.\n", *userDelUsername)
		} else {
			fmt.Printf("Successfully deleted user: %s\n", *userDelUsername)
		}
	}
}
