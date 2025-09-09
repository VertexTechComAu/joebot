package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/harmonicinc-com/joebot/client"
	joebot_middleware "github.com/harmonicinc-com/joebot/middleware"
	"github.com/harmonicinc-com/joebot/routes"
	"github.com/harmonicinc-com/joebot/server"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("joebot", "Command & Control Server/Client For Managing Machines Via Web Interface")

	serverCommand = app.Command("server", "Server Mode")
	serverPort    = serverCommand.Flag("port", "Port For Listening Slave Machine, Default = 13579").Default("13579").Short('p').Int()
	serverCert = serverCommand.Flag("tls-cert", "TLS certificate file (.pem)").ExistingFile()
	serverKey = serverCommand.Flag("tls-key", "TLS private-key file (.pem)").ExistingFile()
	serverTLSCert = serverCommand.Flag("server-tls-cert", "TLS cert for :13579").ExistingFile()
	serverTLSKey = serverCommand.Flag("server-tls-key", "TLS key for :13579").ExistingFile()
	webPortalPort = serverCommand.Flag("web-portal-port", "Port For The Web Portal, Default = 8080").Default("8080").Short('w').Int()

	clientCommand                = app.Command("client", "Client Mode")
	cServerIP                    = clientCommand.Arg("ip", "Server IP").Required().String()
	cServerPort                  = clientCommand.Flag("port", "Server Port, Default=13579").Default("13579").Short('p').Int()
	cwebPortalPort               = clientCommand.Flag("web-portal-port", "Web Portal Port, Default=8080").Default("8080").Short('w').Int()
	cAllowedPortRangeLBound      = clientCommand.Flag("allowed-port-lower-bound", "Lower Bound Of Allowed Port Range").Default("0").Short('l').Int()
	cAllowedPortRangeUBound      = clientCommand.Flag("allowed-port-upper-bound", "Upper Bound Of Allowed Port Range").Default("65535").Short('u').Int()
	cTags                        = clientCommand.Flag("tag", "Tags").Strings()
	cFilebrowserDefaultDirectory = clientCommand.Flag("dir", "Filebrowser Default Directory, Default=/").Default("/").Short('f').String()
	cUseTLS = clientCommand.Flag("tls", "Use TLS to connect to server").Bool()
	cCA = clientCommand.Flag("ca", "CA File to verify server").ExistingFile()
)

func main() {
	defer func() {
		fmt.Println("Ended")
	}()

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case serverCommand.FullCommand():
		s := server.NewServer(nil, *serverTLSCert, *serverTLSKey)
		s.Start(*serverPort)

		e := echo.New()
		v1 := e.Group("/api")

		webPortalAssetsFS := WebPortalAssetsFS()

		v1.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
		}))
		v1.Use(joebot_middleware.AuthMiddleware)

		// routes
		routes.RegisterAPI(v1, s)
		routes.RegisterStatic(e, webPortalAssetsFS)

		addr := ":" + strconv.Itoa(*webPortalPort)
		if *serverCert != "" && *serverKey != "" {
			// mTLS/HTTPS
			log.Printf("HTTPS server listening on https://localhost%s", addr)
			log.Fatal(e.StartTLS(addr, *serverCert, *serverKey))

		} else {
			// Plain HTTP
			log.Printf("HTTP server listening on http://localhost%s", addr)
			log.Fatal(e.Start(addr))
		}
	case clientCommand.FullCommand():
		wg := &sync.WaitGroup{}
		wg.Add(1)
		c := client.NewClient(*cServerIP, *cServerPort, *cwebPortalPort, *cAllowedPortRangeLBound, *cAllowedPortRangeUBound, *cTags, *cUseTLS, *cCA, nil)
		c.FilebrowserDefaultDir = *cFilebrowserDefaultDirectory
		c.Start()
		wg.Wait()
	}
}


