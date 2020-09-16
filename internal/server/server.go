package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/VTGare/boe-tea-go/internal/bot"
	"github.com/VTGare/boe-tea-go/internal/server/models"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/websocket/v2"
	"github.com/sirupsen/logrus"
)

func StartServer(pwd string) {
	app := fiber.New()
	s := bot.BoeTea.Session
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	if pwd != "" {
		app.Use(basicauth.New(basicauth.Config{
			Users: map[string]string{
				"admin": pwd,
			},
		}))
	}

	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("Pong!"))
	})

	app.Get("/ws/stats", websocket.New(func(c *websocket.Conn) {
		var (
			msg []byte
			err error
		)

		msg, err = json.Marshal(models.NewStats(s))
		ticker := time.NewTicker(5 * time.Second)

		if err = c.WriteMessage(1, msg); err != nil {
			log.Println("write:", err)
		}

		for range ticker.C {
			msg, err = json.Marshal(models.NewStats(s))
			if err != nil {
				log.Println("Marshal(): ", err)
				break
			}

			if err = c.WriteMessage(1, msg); err != nil {
				log.Println("WriteMessage():", err)
				break
			}
		}
	}))

	logrus.Fatalln(app.Listen(":8080"))
}
