package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

/* ================= CONFIG ================= */

const (
	TOKEN         = "8547023132:AAG0JDXB8S9s319s_7DAyNZB0onj9xlOuAI"
	RISK_PERCENT  = 3.0
	DUMMY_BALANCE = 1000.0
)

/* ================= STATE ================= */

type CalculateState struct {
	Step    string // WAIT_SIDE | WAIT_PRICE
	Side    string
	RiskUSD float64
}

type AddTradeState struct {
	Step   string // SYMBOL | SIDE | RESULT | AMOUNT | NOTE
	Symbol string
	Side   string
	Result string
	Amount float64
	Note   string
}

var calcStates = map[int64]*CalculateState{}
var addTradeStates = map[int64]*AddTradeState{}

var db *sql.DB

/* ================= DB ================= */

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./trades.db")
	if err != nil {
		log.Fatal(err)
	}

	query := `
	CREATE TABLE IF NOT EXISTS trades (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		symbol TEXT,
		side TEXT,
		result TEXT,
		amount REAL,
		note TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal(err)
	}
}

/* ================= HANDLER ================= */

func handleInfo(bot *tgbotapi.BotAPI, chatID int64) {
	riskUSD := DUMMY_BALANCE * RISK_PERCENT / 100
	msg := fmt.Sprintf(
		"📊 *ACCOUNT INFO*\n\n"+
			"💰 Balance : %.2f USDT\n"+
			"⚠️ Risk    : %.0f%%\n"+
			"💵 Risk $  : %.2f USDT",
		DUMMY_BALANCE, RISK_PERCENT, riskUSD,
	)
	send(bot, chatID, msg)
}

/* ===== CALCULATE SIZE ===== */

func handleCalculate(bot *tgbotapi.BotAPI, chatID, userID int64) {
	riskUSD := DUMMY_BALANCE * RISK_PERCENT / 100
	calcStates[userID] = &CalculateState{
		Step:    "WAIT_SIDE",
		RiskUSD: riskUSD,
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔵 LONG", "CALC_LONG"),
			tgbotapi.NewInlineKeyboardButtonData("🔴 SHORT", "CALC_SHORT"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Pilih posisi:")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func handleCalculateText(bot *tgbotapi.BotAPI, chatID, userID int64, text string) {
	state, ok := calcStates[userID]
	if !ok || state.Step != "WAIT_PRICE" {
		return
	}

	parts := strings.Split(text, ",")
	if len(parts) != 2 {
		send(bot, chatID, "❌ Format: entry,sl")
		return
	}

	entry, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	sl, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)

	stopDist := entry - sl
	if state.Side == "SHORT" {
		stopDist = sl - entry
	}
	if stopDist <= 0 {
		send(bot, chatID, "❌ SL tidak valid")
		return
	}

	positionUSD := state.RiskUSD / (stopDist / entry)
	qty := positionUSD / entry

	send(bot, chatID, fmt.Sprintf(
		"📐 *POSITION SIZE*\n\n"+
			"Side : %s\n"+
			"Risk : %.2f USDT\n"+
			"Size : %.2f USDT\n"+
			"Qty  : %.6f",
		state.Side, state.RiskUSD, positionUSD, qty,
	))

	delete(calcStates, userID)
}

/* ===== ADD TRADE ===== */

func handleAddTrade(bot *tgbotapi.BotAPI, chatID, userID int64) {
	addTradeStates[userID] = &AddTradeState{Step: "SYMBOL"}
	send(bot, chatID, "Masukkan *Symbol* (contoh: BTCUSDT)")
}

func handleAddTradeText(bot *tgbotapi.BotAPI, chatID, userID int64, text string) {
	state, ok := addTradeStates[userID]
	if !ok {
		return
	}

	switch state.Step {

	case "SYMBOL":
		state.Symbol = strings.ToUpper(text)
		state.Step = "SIDE"

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("LONG", "ADD_LONG"),
				tgbotapi.NewInlineKeyboardButtonData("SHORT", "ADD_SHORT"),
			),
		)
		msg := tgbotapi.NewMessage(chatID, "Pilih Side:")
		msg.ReplyMarkup = keyboard
		bot.Send(msg)

	case "AMOUNT":
		amt, err := strconv.ParseFloat(text, 64)
		if err != nil {
			send(bot, chatID, "❌ Amount tidak valid")
			return
		}
		state.Amount = amt
		state.Step = "NOTE"
		send(bot, chatID, "Catatan (bebas):")

	case "NOTE":
		state.Note = text

		_, err := db.Exec(
			"INSERT INTO trades(symbol,side,result,amount,note) VALUES(?,?,?,?,?)",
			state.Symbol, state.Side, state.Result, state.Amount, state.Note,
		)
		if err != nil {
			send(bot, chatID, "❌ Gagal simpan trade")
		} else {
			send(bot, chatID, "✅ Trade tersimpan")
		}

		delete(addTradeStates, userID)
	}
}

/* ================= RESET ================= */

func handleReset(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ BATAL", "RESET_CANCEL"),
			tgbotapi.NewInlineKeyboardButtonData("⚠️ YA, HAPUS SEMUA", "RESET_CONFIRM"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "⚠️ *PERINGATAN*\nSemua data trade akan dihapus!")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

/* ===== CALLBACK ===== */

func handleCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID

	// CALCULATE
	if s, ok := calcStates[userID]; ok {
		if cb.Data == "CALC_LONG" || cb.Data == "CALC_SHORT" {
			s.Side = strings.TrimPrefix(cb.Data, "CALC_")
			s.Step = "WAIT_PRICE"
			bot.Request(tgbotapi.NewCallback(cb.ID, "OK"))
			send(bot, chatID, "Masukkan: entry,sl")
			return
		}
	}

	// ADD TRADE
	if s, ok := addTradeStates[userID]; ok {
		switch cb.Data {
		case "ADD_LONG", "ADD_SHORT":
			s.Side = strings.TrimPrefix(cb.Data, "ADD_")
			s.Step = "RESULT"

			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("WIN", "RES_WIN"),
					tgbotapi.NewInlineKeyboardButtonData("LOSS", "RES_LOSS"),
				),
			)
			msg := tgbotapi.NewMessage(chatID, "Hasil Trade:")
			msg.ReplyMarkup = keyboard
			bot.Send(msg)

		case "RES_WIN", "RES_LOSS":
			s.Result = strings.TrimPrefix(cb.Data, "RES_")
			s.Step = "AMOUNT"
			send(bot, chatID, "Masukkan Amount (USDT):")

		case "RESET_CONFIRM":
			db.Exec("DELETE FROM trades")
			send(bot, chatID, "🗑️ Semua data trade dihapus")

		case "RESET_CANCEL":
			send(bot, chatID, "❎ Reset dibatalkan")

		}
	}
}

/* ================= UTIL ================= */

func send(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

/* ================= MAIN ================= */

func main() {
	initDB()

	bot, err := tgbotapi.NewBotAPI(TOKEN)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Bot running...")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {

		if update.CallbackQuery != nil {
			handleCallback(bot, update.CallbackQuery)
			continue
		}

		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		userID := update.Message.From.ID
		text := update.Message.Text

		/* 🔑 COMMAND = RESET STATE */
		if update.Message.IsCommand() {
			delete(calcStates, userID)
			delete(addTradeStates, userID)

			switch update.Message.Command() {
			case "start":
				send(bot, chatID, "Gunakan /info /calculatesize /addtrade")
			case "info":
				handleInfo(bot, chatID)
			case "calculatesize":
				handleCalculate(bot, chatID, userID)
			case "addtrade":
				handleAddTrade(bot, chatID, userID)
			case "reset":
				handleReset(bot, chatID)
			}
			continue
		}

		handleCalculateText(bot, chatID, userID, text)
		handleAddTradeText(bot, chatID, userID, text)
	}
}
