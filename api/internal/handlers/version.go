package handlers

import (
	"github.com/docshare/api/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

// Version is the server version, injected at build time:
//
//	go build -ldflags "-X github.com/docshare/api/internal/handlers.Version=1.2.3"
var Version = "dev"

const apiVersion = "v1"

type versionResponse struct {
	Version    string `json:"version"`
	APIVersion string `json:"apiVersion"`
}

func GetVersion(c *fiber.Ctx) error {
	return utils.Success(c, fiber.StatusOK, versionResponse{
		Version:    Version,
		APIVersion: apiVersion,
	})
}
