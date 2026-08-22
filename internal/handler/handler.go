package handler

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	CmdStart  = "start"
	BtnSquare = "Посчитать n²"
)

func Start(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет! Жми кнопку, потом введи число — я возведу его в квадрат.")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnSquare),
		),
	)
	if _, err := bot.Send(msg); err != nil {
		fmt.Println("send error:", err)
	}
}

func SquareMessage(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	text := strings.TrimSpace(update.Message.Text)

	if text == BtnSquare {
		if _, err := bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Введи число:")); err != nil {
			fmt.Println("send error:", err)
		}
		return
	}

	n, err := strconv.ParseFloat(text, 64)
	if err != nil {
		if _, err := bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Это не число. Попробуй ещё раз.")); err != nil {
			fmt.Println("send error:", err)
		}
		return
	}
	reply := fmt.Sprintf("%g² = %g", n, n*n)
	if _, err := bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, reply)); err != nil {
		fmt.Println("send error:", err)
	}
}
