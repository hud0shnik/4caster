package main

import (
	"fmt"
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"

	"github.com/hud0shnik/4caster/internal/handler"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on system env")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is empty. Put it in .env or export it.")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = false
	fmt.Println("authorized as", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		switch {
		case update.Message != nil && update.Message.IsCommand() && update.Message.Command() == handler.CmdStart:
			handler.Start(update, bot)
		case update.Message != nil && update.Message.Text != "":
			handler.HandleMessage(update, bot)
		}
	}
}
