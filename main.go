package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	RISK_PERCENT  = 3.0
	DUMMY_BALANCE = 1000.0 // simulasi balance USDT
)

type UserState struct {
	Step    string // WAIT_SIDE | WAIT_PRICE
	Side    string // LONG / SHORT
	Balance float64
	RiskUSD float64
}

var userStates = map[int64]*UserState{}

const token = "8547023132:AAG0JDXB8S9s319s_7DAyNZB0onj9xlOuAI"

func handleText(bot *tgbotapi.BotAPI, chatID int64, userID int64, text string) {
	state, ok := userStates[userID]
	if !ok {
		return
	}

	if state.Step != "WAIT_PRICE" {
		return
	}

	parts := strings.Split(text, ",")
	if len(parts) != 2 {
		send(bot, chatID, "❌ Format salah\nGunakan: entry,sl")
		return
	}

	entry, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	sl, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)

	if err1 != nil || err2 != nil {
		send(bot, chatID, "❌ Angka tidak valid")
		return
	}

	var stopDist float64
	if state.Side == "LONG" {
		stopDist = entry - sl
	} else {
		stopDist = sl - entry
	}

	if stopDist <= 0 {
		send(bot, chatID, "❌ Stop Loss tidak valid untuk "+state.Side)
		return
	}

	positionUSD := state.RiskUSD / (stopDist / entry)
	qty := positionUSD / entry

	msg := fmt.Sprintf(
		"📐 POSITION SIZE\n\n"+
			"Side       : %s\n"+
			"Entry      : %.2f\n"+
			"Stop Loss  : %.2f\n"+
			"⚠️ Risk $  : %.2f USDT\n"+
			"📦 Size $  : %.2f USDT\n"+
			"📊 Qty     : %.6f\n\n"+
			"Leverage bebas (risk tetap %.0f%%)",
		state.Side,
		entry,
		sl,
		state.RiskUSD,
		positionUSD,
		qty,
		RISK_PERCENT,
	)

	send(bot, chatID, msg)
	delete(userStates, userID)
}

func handleCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID

	state, ok := userStates[userID]
	if !ok {
		return
	}

	switch cb.Data {
	case "SIDE_LONG":
		state.Side = "LONG"
	case "SIDE_SHORT":
		state.Side = "SHORT"
	default:
		return
	}

	state.Step = "WAIT_PRICE"

	// Hapus inline button (UX bersih)
	edit := tgbotapi.NewEditMessageReplyMarkup(
		chatID,
		cb.Message.MessageID,
		tgbotapi.InlineKeyboardMarkup{},
	)
	bot.Send(edit)

	// ✅ Jawab callback (hilangkan loading di Telegram)
	callback := tgbotapi.NewCallback(cb.ID, "Dipilih: "+state.Side)
	bot.Request(callback)

	bot.Send(tgbotapi.NewMessage(
		chatID,
		"Masukkan Entry dan Stop Loss\n\nFormat:\nentry,sl",
	))
}

func main() {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = false
	log.Printf("Bot aktif sebagai @%s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {

		// Callback Query (Inline Button)
		if update.CallbackQuery != nil {
			handleCallback(bot, update.CallbackQuery)
			continue
		}

		// Message
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		userID := update.Message.From.ID
		text := update.Message.Text

		// COMMAND
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				send(bot, chatID, "Selamat datang 👋\nGunakan /info atau /calculatesize")
			case "info":
				handleInfo(bot, chatID)
			case "calculatesize":
				handleCalculate(bot, chatID, userID)
			default:
				send(bot, chatID, "Command tidak dikenal")
			}
			continue
		}

		// TEXT FLOW
		handleText(bot, chatID, userID, text)
	}
}

func handleInfo(bot *tgbotapi.BotAPI, chatID int64) {
	riskUSD := DUMMY_BALANCE * RISK_PERCENT / 100

	msg := fmt.Sprintf(
		"📊 ACCOUNT INFO\n\n"+
			"💰 Balance : %.2f USDT\n"+
			"⚠️ Risk    : %.0f%%\n"+
			"💵 Risk $  : %.2f USDT",
		DUMMY_BALANCE,
		RISK_PERCENT,
		riskUSD,
	)

	send(bot, chatID, msg)
}

func handleCalculate(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	riskUSD := DUMMY_BALANCE * RISK_PERCENT / 100

	userStates[userID] = &UserState{
		Step:    "WAIT_SIDE",
		Balance: DUMMY_BALANCE,
		RiskUSD: riskUSD,
	}

	btnLong := tgbotapi.NewInlineKeyboardButtonData("🔵 LONG", "SIDE_LONG")
	btnShort := tgbotapi.NewInlineKeyboardButtonData("🔴 SHORT", "SIDE_SHORT")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(btnLong, btnShort),
	)

	msg := tgbotapi.NewMessage(chatID, "Pilih posisi:")
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
}

func send(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}
