package service

import (
	"github.com/gofiber/fiber/v2"

	"channelog/config"
	"channelog/rabbit"
)

// LivenessService responds with 200 OK if the channelog can connect to RabbitMQ.
func LivenessService(c *fiber.Ctx, cfg *config.Config) error {
	if err := rabbit.CheckRabbitMQ(cfg.AMQPURL); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("unhealthy")
	}
	return c.SendString("ok")
}
