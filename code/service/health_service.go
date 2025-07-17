package service

import (
	"github.com/gofiber/fiber/v2"

	"channelog/config"
)

// LivenessService responds with 200 OK if the channelog can connect to RabbitMQ.
func LivenessService(c *fiber.Ctx, cfg *config.Config) error {
	return c.SendString("ok")
}
