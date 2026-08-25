package handler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	CmdStart = "start"

	BtnCalc = "Рендер"

	BtnByFrames   = "По фреймам"
	BtnByDuration = "По длительности сцены"

	BtnNoTiles = "Без тайлов"
	BtnTiles   = "С тайлами"

	BtnBack = "Назад"
)

type step int

const (
	stepIdle step = iota
	stepMenu

	stepSumFirst
	stepSumSecond

	stepCalcPickMode
	stepCalcFrames
	stepCalcTime

	stepTilesT
	stepTilesF
	stepTilest

	stepDurPeriod
	stepDurFps
	stepDurTime
)

type calcState struct {
	step step
	a    float64
	b    float64
}

var (
	statesMu sync.Mutex
	states   = map[int64]*calcState{}
)

func Start(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	reset(update.Message.Chat.ID)
	sendMenu(chatIDOf(update), bot, "Привет! Выбери действие.")
}

func chatIDOf(update tgbotapi.Update) int64 {
	if update.Message != nil {
		return update.Message.Chat.ID
	}
	return 0
}

func sendMenu(chatID int64, bot *tgbotapi.BotAPI, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnCalc),
		),
	)
	if _, err := bot.Send(msg); err != nil {
		fmt.Println("send error:", err)
	}
}

func sendCalcMenu(chatID int64, bot *tgbotapi.BotAPI, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnByFrames),
			tgbotapi.NewKeyboardButton(BtnByDuration),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnBack),
		),
	)
	if _, err := bot.Send(msg); err != nil {
		fmt.Println("send error:", err)
	}
}

func sendTilesPickMenu(chatID int64, bot *tgbotapi.BotAPI, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnNoTiles),
			tgbotapi.NewKeyboardButton(BtnTiles),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(BtnBack),
		),
	)
	if _, err := bot.Send(msg); err != nil {
		fmt.Println("send error:", err)
	}
}

func send(chatID int64, bot *tgbotapi.BotAPI, text string) {
	if _, err := bot.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		fmt.Println("send error:", err)
	}
}

func HandleMessage(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)

	switch text {
	case BtnCalc:
		setStep(chatID, stepCalcPickMode)
		sendCalcMenu(chatID, bot, "Давай посчитаю. Как задаём параметры?")
		return
	case BtnByFrames:
		setStep(chatID, stepCalcFrames)
		sendTilesPickMenu(chatID, bot, "Окей, а что по поводу тайлов?")
		return
	case BtnNoTiles:
		setStep(chatID, stepCalcFrames)
		send(chatID, bot, "Сколько фреймов в сцене?")
		return
	case BtnTiles:
		setStep(chatID, stepTilesT)
		send(chatID, bot, "Сколько тайлов в одном фрейме?")
		return
	case BtnByDuration:
		setStep(chatID, stepDurPeriod)
		send(chatID, bot,
			"Сколько длится сцена?\n"+
				"Формат: 6с, 12s, 1м9с, 9m8s")
		return
	case BtnBack:
		setStep(chatID, stepIdle)
		sendMenu(chatID, bot, "Что считаем?")
		return
	}

	st := getState(chatID)

	switch st.step {
	case stepSumFirst:
		n, err := parseNumber(text)
		if err != nil {
			send(chatID, bot, "Это не число. Попробуй ещё раз.")
			return
		}
		st.a = n
		st.step = stepSumSecond
		send(chatID, bot, "Введи второе число:")

	case stepSumSecond:
		n, err := parseNumber(text)
		if err != nil {
			send(chatID, bot, "Это не число. Попробуй ещё раз.")
			return
		}
		result := st.a + n
		reply := fmt.Sprintf("%g + %g = %g", st.a, n, result)
		reset(chatID)
		sendMenu(chatID, bot, reply)

	case stepCalcFrames:
		F, err := parseInt(text)
		if err != nil || F <= 0 {
			send(chatID, bot, "F должно быть положительным целым числом. Попробуй ещё раз.")
			return
		}
		st.a = float64(F)
		st.step = stepCalcTime
		send(chatID, bot,
			"Сколько длится рендер одного фрейма?\n"+
				"Формат: 6с, 12s, 1м9с, 9m8s")

	case stepCalcTime:
		t, err := parseDuration(text)
		if err != nil {
			send(chatID, bot,
				"Не понял, сколько времени?\nПримеры: 6с, 12s, 1м9с, 9m8s")
			return
		}
		F := st.a
		totalSec := F*t + F*8
		reply := fmt.Sprintf("Результат: %s", formatDuration(totalSec))
		reset(chatID)
		sendMenu(chatID, bot, reply)

	case stepTilesT:
		T, err := parseInt(text)
		if err != nil || T <= 0 {
			send(chatID, bot, "T должно быть положительным целым. Попробуй ещё раз.")
			return
		}
		st.a = float64(T)
		st.step = stepTilesF
		send(chatID, bot, "Сколько фреймов в сцене?")

	case stepTilesF:
		F, err := parseInt(text)
		if err != nil || F <= 0 {
			send(chatID, bot, "F должно быть положительным целым. Попробуй ещё раз.")
			return
		}
		st.b = float64(F)
		st.step = stepTilest
		send(chatID, bot,
			"Сколько длится рендер одного тайла?\n"+
				"Формат: 6с, 12s, 1м9с, 9m8s")

	case stepTilest:
		t, err := parseDuration(text)
		if err != nil {
			send(chatID, bot,
				"Не понял, сколько времени?\nПримеры: 6с, 12s, 1м9с, 9m8s")
			return
		}
		T := st.a
		F := st.b
		totalSec := (T*t)*F + F*8
		reply := fmt.Sprintf("Результат: %s", formatDuration(totalSec))
		reset(chatID)
		sendMenu(chatID, bot, reply)

	case stepDurPeriod:
		P, err := parseDuration(text)
		if err != nil || P <= 0 {
			send(chatID, bot,
				"Не понял, сколько времени?\nПримеры: 6с, 12s, 1м9с, 9m8s")
			return
		}
		st.a = P
		st.step = stepDurFps
		send(chatID, bot, "Сколько fps?")

	case stepDurFps:
		fps, err := parseInt(text)
		if err != nil || fps <= 0 {
			send(chatID, bot, "fps должно быть положительным целым. Попробуй ещё раз.")
			return
		}
		st.b = float64(fps)
		st.step = stepDurTime
		send(chatID, bot,
			"Сколько длится рендер одного фрейма?\n"+
				"Формат: 6с, 12s, 1м9с, 9m8s")

	case stepDurTime:
		t, err := parseDuration(text)
		if err != nil {
			send(chatID, bot,
				"Не понял, сколько времени?\nПримеры: 6с, 12s, 1м9с, 9m8s")
			return
		}
		P := st.a
		fps := st.b
		F := P * fps
		totalSec := F*t + F*8
		reply := fmt.Sprintf("Результат: %s", formatDuration(totalSec))
		reset(chatID)
		sendMenu(chatID, bot, reply)

	default:
		sendMenu(chatID, bot,
			"Выбери действие: «"+BtnCalc+"».")
	}
}

func parseNumber(s string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

var durationRe = regexp.MustCompile(`(?i)(\d+)\s*([hmsчмс])`)

func parseDuration(s string) (float64, error) {
	s = strings.TrimSpace(s)

	if matches := durationRe.FindAllStringSubmatch(s, -1); len(matches) > 0 {
		var total float64
		matched := 0
		for _, m := range matches {
			val, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				return 0, fmt.Errorf("bad number %q", m[1])
			}
			switch strings.ToLower(m[2]) {
			case "h", "ч":
				total += val * 3600
			case "m", "м":
				total += val * 60
			case "s", "с":
				total += val
			}
			matched += len(m[0])
		}
		if matched != len(s) {
			return 0, fmt.Errorf("unexpected chars in %q", s)
		}
		return total, nil
	}

	clean := strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(s), "с"), "s")
	clean = strings.TrimSpace(clean)
	n, err := strconv.ParseFloat(strings.ReplaceAll(clean, ",", "."), 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func formatDuration(sec float64) string {
	total := int(sec + 0.5)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 3600 % 60
	if h > 0 {
		return fmt.Sprintf("%dч %dм %dс", h, m, s)
	}
	return fmt.Sprintf("%dм %dс", m, s)
}

func getState(chatID int64) *calcState {
	statesMu.Lock()
	defer statesMu.Unlock()
	st, ok := states[chatID]
	if !ok {
		st = &calcState{}
		states[chatID] = st
	}
	return st
}

func setStep(chatID int64, s step) {
	statesMu.Lock()
	defer statesMu.Unlock()
	st, ok := states[chatID]
	if !ok {
		st = &calcState{}
		states[chatID] = st
	}
	st.step = s
	st.a = 0
	st.b = 0
}

func reset(chatID int64) {
	statesMu.Lock()
	defer statesMu.Unlock()
	delete(states, chatID)
}
