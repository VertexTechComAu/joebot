package routes

import (
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/harmonicinc-com/joebot/handlers"
	"github.com/harmonicinc-com/joebot/server"
	"github.com/labstack/echo"
)

func RegisterAPI(g *echo.Group, s *server.Server) {
	g.GET("/clients", handlers.GetClients(s))
	g.GET("/users", handlers.GetUsers(s))
	g.GET("/users/ip-whitelisted", handlers.GetIPWhiteListed(s))
	g.POST("/users", handlers.CreateUser(s))
	g.POST("/login", handlers.Login(s))
	g.POST("/client/:id", handlers.CreateTunnel(s))
	g.POST("/bulk-install", handlers.BuldInstall(s))
}

func RegisterStatic(e *echo.Echo, assets fs.FS) {
	e.GET("/", func(c echo.Context) error {
		f, err := assets.Open("index.html")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		b, err := io.ReadAll(f)
		if err != nil {
			log.Fatal(err)
		}
		return c.HTML(200, string(b))
	})
	e.GET("/*", echo.WrapHandler(http.FileServer(http.FS(assets))))
}