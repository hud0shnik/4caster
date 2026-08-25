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
	CmdStart      = "start"
	BtnCalc       = "Рендер"
	BtnByFrames   = "По фреймам"
	BtnByDuration = "По длительности сцены"
	BtnNoTiles    = "Без тайлов"
	BtnTiles      = "С тайлами"
	BtnBack       = "Назад"

	MsgHello           = "Привет! Выбери действие."
	MsgPickMode        = "Давай посчитаю. Как задаём параметры?"
	MsgTilesPrompt     = "Окей, а что по поводу тайлов?"
	MsgTilesCount      = "Сколько тайлов в одном фрейме?"
	MsgFramesCount     = "Сколько фреймов в сцене?"
	MsgFps             = "Сколько fps?"
	MsgSceneDuration   = "Сколько длится сцена?\nФормат: 6с, 12s, 1м9с, 9m8s"
	MsgFrameRender     = "Сколько длится рендер одного фрейма?\nФормат: 6с, 12s, 1м9с, 9m8s"
	MsgTileRender      = "Сколько длится рендер одного тайла?\nФормат: 6с, 12s, 1м9с, 9m8s"
	MsgBadDuration     = "Не понял, сколько времени?\nПримеры: 6с, 12s, 1м9с, 9m8s"
	MsgNotNumber       = "Это не число. Попробуй ещё раз."
	MsgSecondNumber    = "Введи второе число:"
	MsgMenuTitle       = "Что считаем?"
	MsgChooseAction    = "Выбери действие: «%s»."
	MsgResult          = "Результат: %s"
	MsgFPositiveStrict = "F должно быть положительным целым числом. Попробуй ещё раз."
	MsgFPositive       = "F должно быть положительным целым. Попробуй ещё раз."
	MsgTPositive       = "T должно быть положительным целым. Попробуй ещё раз."
	MsgFpsPositive     = "fps должно быть положительным целым. Попробуй ещё раз."
)

// step — позиция конечного автомата (FSM) для конкретного чата.
type step int

const (
	// stepIdle означает, что у пользователя нет активного сценария; доступны только кнопки меню.
	stepIdle step = iota
	// stepMenu оставлен для совместимости со старой маршрутизацией.
	stepMenu

	// stepSumFirst ожидает первое слагаемое устаревшего сценария суммирования.
	stepSumFirst
	// stepSumSecond ожидает второе слагаемое устаревшего сценария суммирования.
	stepSumSecond

	// stepCalcPickMode ожидает выбор режима: по фреймам или по длительности.
	stepCalcPickMode
	// stepCalcFrames ожидает общее число фреймов в сцене.
	stepCalcFrames
	// stepCalcTime ожидает длительность рендера одного фрейма.
	stepCalcTime

	// stepTilesT ожидает количество тайлов в одном фрейме.
	stepTilesT
	// stepTilesF ожидает общее число фреймов в сцене (ветка с тайлами).
	stepTilesF
	// stepTilest ожидает длительность рендера одного тайла.
	stepTilest

	// stepDurPeriod ожидает длительность сцены в ветке расчёта по длительности.
	stepDurPeriod
	// stepDurFps ожидает частоту кадров (fps) в ветке расчёта по длительности.
	stepDurFps
	// stepDurTime ожидает длительность рендера одного фрейма в ветке расчёта по длительности.
	stepDurTime
)

// calcState хранит позицию FSM и накопленные значения для конкретного чата.
// a и b используются в разных сценариях как float64: длительности хранятся
// в долях секунды, а целочисленные счётчики приводятся к float64 ради
// единообразия арифметики.
type calcState struct {
	step step
	a    float64
	b    float64
}

var (
	// statesMu защищает карту states от конкурентного доступа.
	statesMu sync.Mutex
	// states сопоставляет ID чата в Telegram с текущим состоянием FSM калькулятора.
	states = map[int64]*calcState{}
)

// Start обрабатывает команду /start: сбрасывает состояние чата и показывает
// главное меню.
func Start(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	reset(update.Message.Chat.ID)
	sendMenu(chatIDOf(update), bot, MsgHello)
}

// chatIDOf возвращает ID чата из update, либо 0, если update не содержит
// сообщения (например, inline-запрос или callback).
func chatIDOf(update tgbotapi.Update) int64 {
	if update.Message != nil {
		return update.Message.Chat.ID
	}
	return 0
}

// sendMenu отправляет сообщение с главным меню и единственной кнопкой «Рендер».
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

// sendCalcMenu отправляет выбор режима калькулятора: по фреймам или по длительности.
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

// sendTilesPickMenu отправляет выбор режима тайлов: с тайлами или без.
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

// send отправляет простое текстовое сообщение без клавиатуры.
func send(chatID int64, bot *tgbotapi.BotAPI, text string) {
	if _, err := bot.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		fmt.Println("send error:", err)
	}
}

// HandleMessage маршрутизирует входящее текстовое сообщение через FSM:
// сначала обрабатывает кнопки верхнего уровня, затем продвигает текущий
// шаг чата.
func HandleMessage(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)

	switch text {
	case BtnCalc:
		setStep(chatID, stepCalcPickMode)
		sendCalcMenu(chatID, bot, MsgPickMode)
		return
	case BtnByFrames:
		setStep(chatID, stepCalcFrames)
		sendTilesPickMenu(chatID, bot, MsgTilesPrompt)
		return
	case BtnNoTiles:
		setStep(chatID, stepCalcFrames)
		send(chatID, bot, MsgFramesCount)
		return
	case BtnTiles:
		setStep(chatID, stepTilesT)
		send(chatID, bot, MsgTilesCount)
		return
	case BtnByDuration:
		setStep(chatID, stepDurPeriod)
		send(chatID, bot, MsgSceneDuration)
		return
	case BtnBack:
		setStep(chatID, stepIdle)
		sendMenu(chatID, bot, MsgMenuTitle)
		return
	}

	st := getState(chatID)

	switch st.step {
	case stepSumFirst:
		n, err := parseNumber(text)
		if err != nil {
			send(chatID, bot, MsgNotNumber)
			return
		}
		st.a = n
		st.step = stepSumSecond
		send(chatID, bot, MsgSecondNumber)

	case stepSumSecond:
		n, err := parseNumber(text)
		if err != nil {
			send(chatID, bot, MsgNotNumber)
			return
		}
		result := st.a + n
		reply := fmt.Sprintf("%g + %g = %g", st.a, n, result)
		reset(chatID)
		sendMenu(chatID, bot, reply)

	case stepCalcFrames:
		F, err := parseInt(text)
		if err != nil || F <= 0 {
			send(chatID, bot, MsgFPositiveStrict)
			return
		}
		st.a = float64(F)
		st.step = stepCalcTime
		send(chatID, bot, MsgFrameRender)

	case stepCalcTime:
		t, err := parseDuration(text)
		if err != nil {
			send(chatID, bot, MsgBadDuration)
			return
		}
		F := st.a
		totalSec := F*t + F*8
		reply := fmt.Sprintf(MsgResult, formatDuration(totalSec))
		reset(chatID)
		sendMenu(chatID, bot, reply)

	case stepTilesT:
		T, err := parseInt(text)
		if err != nil || T <= 0 {
			send(chatID, bot, MsgTPositive)
			return
		}
		st.a = float64(T)
		st.step = stepTilesF
		send(chatID, bot, MsgFramesCount)

	case stepTilesF:
		F, err := parseInt(text)
		if err != nil || F <= 0 {
			send(chatID, bot, MsgFPositive)
			return
		}
		st.b = float64(F)
		st.step = stepTilest
		send(chatID, bot, MsgTileRender)

	case stepTilest:
		t, err := parseDuration(text)
		if err != nil {
			send(chatID, bot, MsgBadDuration)
			return
		}
		T := st.a
		F := st.b
		totalSec := (T*t)*F + F*8
		reply := fmt.Sprintf(MsgResult, formatDuration(totalSec))
		reset(chatID)
		sendMenu(chatID, bot, reply)

	case stepDurPeriod:
		P, err := parseDuration(text)
		if err != nil || P <= 0 {
			send(chatID, bot, MsgBadDuration)
			return
		}
		st.a = P
		st.step = stepDurFps
		send(chatID, bot, MsgFps)

	case stepDurFps:
		fps, err := parseInt(text)
		if err != nil || fps <= 0 {
			send(chatID, bot, MsgFpsPositive)
			return
		}
		st.b = float64(fps)
		st.step = stepDurTime
		send(chatID, bot, MsgFrameRender)

	case stepDurTime:
		t, err := parseDuration(text)
		if err != nil {
			send(chatID, bot, MsgBadDuration)
			return
		}
		P := st.a
		fps := st.b
		F := P * fps
		totalSec := F*t + F*8
		reply := fmt.Sprintf(MsgResult, formatDuration(totalSec))
		reset(chatID)
		sendMenu(chatID, bot, reply)

	default:
	}
}

// parseNumber разбирает произвольное десятичное число, принимая ',' как
// десятичный разделитель в дополнение к '.'.
func parseNumber(s string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
}

// parseInt разбирает целое десятичное число после TrimSpace.
func parseInt(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

// durationRe разбирает компоненты длительности вида "1h", "2m", "30s"
// или их кириллические аналоги "ч", "м", "с".
var durationRe = regexp.MustCompile(`(?i)(\d+)\s*([hmsчмс])`)

// parseDuration разбирает строку длительности в секунды.
//
// Поддерживает составные формы (например, "1h2m", "1м30с", "9m8s") через
// durationRe и, если их нет, трактует вход как простое число (опционально
// с суффиксом "s" или "с") в секундах.
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

// formatDuration форматирует количество секунд в человекочитаемую строку
// длительности на русском; часы опускаются, если их ноль.
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

// getState возвращает состояние FSM чата, создавая свежий нулевой объект,
// если чат ещё не встречался.
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

// setStep переводит чат на указанный шаг FSM и обнуляет накопленные a и b,
// чтобы значения не утекали между сценариями.
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

// reset полностью удаляет состояние FSM чата; следующий getState создаст
// новое с нуля.
func reset(chatID int64) {
	statesMu.Lock()
	defer statesMu.Unlock()
	delete(states, chatID)
}
