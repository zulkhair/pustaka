package httpapi

import (
	"os"

	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/config"
)

type VersionHandler struct {
	version string
	apkPath string
}

func NewVersionHandler(cfg config.Config) *VersionHandler {
	return &VersionHandler{version: cfg.AppVersion, apkPath: cfg.APKPath}
}

type versionDTO struct {
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
}

func (h *VersionHandler) Get(c *fiber.Ctx) error {
	return OK(c, versionDTO{Version: h.version, DownloadURL: "/api/version/download"})
}

type putVersionReq struct {
	Version string `json:"version"`
}

func (h *VersionHandler) Put(c *fiber.Ctx) error {
	var req putVersionReq
	if err := c.BodyParser(&req); err != nil || req.Version == "" {
		return Fail(c, fiber.StatusBadRequest, "version required")
	}
	h.version = req.Version
	return OK(c, versionDTO{Version: h.version, DownloadURL: "/api/version/download"})
}

func (h *VersionHandler) Download(c *fiber.Ctx) error {
	if h.apkPath == "" {
		return Fail(c, fiber.StatusNotFound, "no apk configured")
	}
	if _, err := os.Stat(h.apkPath); err != nil {
		return Fail(c, fiber.StatusNotFound, "apk not found")
	}
	return c.Download(h.apkPath, "pustaka.apk")
}
