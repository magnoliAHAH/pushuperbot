package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/telebot.v3"
)

// UserData теперь хранится в SQLite
type PushupRecord struct {
	Date         string // "YYYY-MM-DD"
	PushupsCount int
}

var db *sql.DB

func main() {
	// Подключаем SQLite
	var err error
	db, err = sql.Open("sqlite3", "./pushups.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Создаём таблицу, если её нет
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS pushups (
			user_id INTEGER,
			date TEXT,
			count INTEGER,
			PRIMARY KEY (user_id, date)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Настройка бота
	bot, err := telebot.NewBot(telebot.Settings{
		Token:  "YOUR_TELEGRAM_BOT_TOKEN", // Замени на свой токен
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Команды
	bot.Handle("/start", onStart)
	bot.Handle("/add", onAddPushups)
	bot.Handle("/progress", onProgress)
	bot.Handle("/calendar", onCalendar)

	// Напоминание каждый день в 20:00
	go sendDailyReminders(bot)

	log.Println("Бот запущен!")
	bot.Start()
}

// Обработчики команд
func onStart(c telebot.Context) error {
	return c.Send(
		"🏋️‍♂️ <b>Трекер 100 отжиманий в день</b>\n\n"+
			"Доступные команды:\n"+
			"/add <число> — добавить отжимания\n"+
			"/progress — текущий прогресс\n"+
			"/calendar — история за неделю\n\n"+
			"<i>Цель: 100 отжиманий ежедневно!</i>",
		telebot.ModeHTML,
	)
}

func onAddPushups(c telebot.Context) error {
	userID := c.Sender().ID
	count := 0

	if len(c.Args()) == 0 {
		return c.Send("Используй: /add <число>")
	}
	if _, err := fmt.Sscan(c.Args()[0], &count); err != nil || count <= 0 {
		return c.Send("❌ Введи число больше 0!")
	}

	// Обновляем данные в SQLite
	today := time.Now().Format("2006-01-02")
	_, err := db.Exec(`
		INSERT INTO pushups (user_id, date, count)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, date) DO UPDATE SET count = count + ?
	`, userID, today, count, count)
	if err != nil {
		log.Println("DB error:", err)
		return c.Send("❌ Ошибка базы данных!")
	}

	// Проверяем, достигнута ли цель
	var totalToday int
	db.QueryRow(`
		SELECT count FROM pushups 
		WHERE user_id = ? AND date = ?
	`, userID, today).Scan(&totalToday)

	if totalToday >= 100 {
		return c.Send(fmt.Sprintf("🔥 <b>Ты выполнил 100 отжиманий сегодня!</b> Так держать!", totalToday),
			telebot.ModeHTML)
	}
	return c.Send(fmt.Sprintf("✅ +%d отжиманий! Всего сегодня: %d/100\nОсталось: %d",
		count, totalToday, 100-totalToday))
}

func onProgress(c telebot.Context) error {
	userID := c.Sender().ID
	today := time.Now().Format("2006-01-02")
	var count int

	err := db.QueryRow(`
		SELECT count FROM pushups 
		WHERE user_id = ? AND date = ?
	`, userID, today).Scan(&count)

	if err == sql.ErrNoRows {
		return c.Send("📌 Сегодня ты ещё не делал отжиманий. Вперёд к цели!")
	} else if err != nil {
		return c.Send("❌ Ошибка базы данных!")
	}

	progressBar := getProgressBar(count, 100)
	return c.Send(fmt.Sprintf(
		"📊 <b>Прогресс за сегодня:</b>\n\n%s\n%d из 100 отжиманий\nОсталось: %d",
		progressBar, count, 100-count,
	), telebot.ModeHTML)
}

func onCalendar(c telebot.Context) error {
	userID := c.Sender().ID
	rows, err := db.Query(`
		SELECT date, count FROM pushups 
		WHERE user_id = ? 
		ORDER BY date DESC 
		LIMIT 7
	`, userID)
	if err != nil {
		return c.Send("❌ Ошибка базы данных!")
	}
	defer rows.Close()

	var calendarText string = "📅 <b>Последние 7 дней:</b>\n\n"
	for rows.Next() {
		var date string
		var count int
		rows.Scan(&date, &count)
		progressBar := getProgressBar(count, 100)
		calendarText += fmt.Sprintf("<i>%s:</i> %s <b>%d</b>/100\n", date, progressBar, count)
	}

	return c.Send(calendarText, telebot.ModeHTML)
}

// Вспомогательные функции
func getProgressBar(current, max int) string {
	const barLength = 10
	filled := int(float64(current) / float64(max) * barLength)
	bar := ""
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar += "■"
		} else {
			bar += "□"
		}
	}
	return bar
}

func sendDailyReminders(bot *telebot.Bot) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 20, 0, 0, 0, now.Location())
		time.Sleep(time.Until(next))

		rows, err := db.Query(`
			SELECT user_id, count FROM pushups 
			WHERE date = ?
		`, now.Format("2006-01-02"))
		if err != nil {
			log.Println("Reminder DB error:", err)
			continue
		}

		for rows.Next() {
			var userID int64
			var count int
			rows.Scan(&userID, &count)

			if count < 100 {
				bot.Send(telebot.ChatID(userID), fmt.Sprintf(
					"⏰ <b>Напоминание</b>\nСегодня ты сделал %d/100 отжиманий.\nОсталось всего %d! 💪",
					count, 100-count,
				), telebot.ModeHTML)
			}
		}
		rows.Close()
	}
}
