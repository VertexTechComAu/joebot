package handlers

import (
	"net/http"
	"strconv"

	"github.com/harmonicinc-com/joebot/models"
	"github.com/harmonicinc-com/joebot/server"
	"github.com/labstack/echo"
)

func GetClients(s *server.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, s.GetClientsList)
	}
}

func GetUsers(s *server.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		res, err := s.GetAllUser()
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, res)
	}
}

func GetIPWhiteListed(s *server.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := c.Get("token").(string)
		res, err := s.GetUserIPWhitelisted(token)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, res)
	}
}

func CreateUser(s *server.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		json := models.UserInfo{}
		if err := c.Bind(&json); err != nil {
			return err
		}
		if err := s.CreateUser(json); err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, json)
	}
}

func Login(s *server.Server) echo.HandlerFunc {
	return func(c echo.Context) error {
		username, password := c.FormValue("username"), c.FormValue("password")
		res, err := s.UserLogin(c.RealIP(), username, password)
		if err != nil {
			return c.JSON(http.StatusBadRequest, err)
		}
		return c.JSON(http.StatusOK, res)
	}
}

func CreateTunnel(s *server.Server) echo.HandlerFunc {
	return  func(c echo.Context) error {
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

func BuldInstall(s *server.Server) echo.HandlerFunc {
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