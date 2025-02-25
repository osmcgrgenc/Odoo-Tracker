package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kolo/xmlrpc"
	"github.com/robfig/cron/v3"
)

var (
	bot      *tgbotapi.BotAPI
	chatID   int64
	cronJobs *cron.Cron
)

// Telegram botunu başlat
func InitTelegramBot() error {
	var err error
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN çevre değişkeni ayarlanmamış")
	}

	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr == "" {
		return fmt.Errorf("TELEGRAM_CHAT_ID çevre değişkeni ayarlanmamış")
	}

	chatID, err = strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("geçersiz TELEGRAM_CHAT_ID: %v", err)
	}

	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("bot başlatılamadı: %v", err)
	}

	log.Printf("Telegram botu başlatıldı: %s", bot.Self.UserName)
	return nil
}

// Zamanlanmış görevleri başlat
func StartScheduledJobs() {
	cronJobs = cron.New()

	// Her sabah 8'de rapor gönder
	cronJobs.AddFunc("0 8 * * *", func() {
		sendDailyReport("morning")
	})

	// Her akşam 6'da rapor gönder
	cronJobs.AddFunc("0 18 * * *", func() {
		sendDailyReport("evening")
	})

	cronJobs.Start()
	log.Println("Zamanlanmış görevler başlatıldı")
}

// Günlük rapor gönder
func sendDailyReport(timeOfDay string) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var message string
	if timeOfDay == "morning" {
		message = fmt.Sprintf("🌞 Günaydın! %s tarihli günlük rapor hazırlanıyor...", today.Format("2006-01-02"))
	} else {
		message = fmt.Sprintf("🌙 İyi akşamlar! %s tarihli günlük rapor hazırlanıyor...", today.Format("2006-01-02"))
	}

	// Önce bildirim mesajı gönder
	sendTelegramMessage(message)

	// Raporu oluştur ve gönder
	outputFileName, err := generateReport("daily", "", true)
	if err != nil {
		sendTelegramMessage(fmt.Sprintf("❌ Rapor oluşturulurken hata oluştu: %v", err))
		return
	}

	// Rapor dosyasını oku ve gönder
	reportContent, err := os.ReadFile(outputFileName)
	if err != nil {
		sendTelegramMessage(fmt.Sprintf("❌ Rapor dosyası okunamadı: %v", err))
		return
	}

	// Raporu özetle ve gönder
	summary := generateSummary(string(reportContent))
	sendTelegramMessage(summary)
}

// Rapor özetini oluştur
func generateSummary(content string) string {
	lines := strings.Split(content, "\n")
	summary := "📊 **Günlük Çalışma Raporu Özeti**\n\n"

	// Tarih aralığını bul
	for _, line := range lines {
		if strings.HasPrefix(line, "Tarih Aralığı:") {
			summary += line + "\n\n"
			break
		}
	}

	// Toplam çalışma saatini bul
	totalHoursFound := false
	for _, line := range lines {
		if strings.HasPrefix(line, "Toplam Çalışma Saati:") {
			summary += line + "\n\n"
			totalHoursFound = true
			break
		}
	}

	if !totalHoursFound {
		summary += "Toplam çalışma saati bulunamadı.\n\n"
	}

	// Çalışan bazında saatleri bul
	summary += "👥 **Çalışan Bazında Saatler:**\n"
	employeeSection := false
	employeeCount := 0

	for _, line := range lines {
		if strings.Contains(line, "Çalışan Bazında Toplam Saatler:") {
			employeeSection = true
			continue
		}

		if employeeSection && strings.Contains(line, ":") && !strings.Contains(line, "Proje Bazında") {
			summary += line + "\n"
			employeeCount++
		}

		if employeeSection && strings.Contains(line, "Proje Bazında") {
			break
		}
	}

	if employeeCount == 0 {
		summary += "Çalışan bilgisi bulunamadı.\n"
	}

	// Proje bazında saatleri bul
	summary += "\n🏢 **Proje Bazında Saatler:**\n"
	projectSection := false
	projectCount := 0

	for _, line := range lines {
		if strings.Contains(line, "Proje Bazında Saatler:") {
			projectSection = true
			continue
		}

		if projectSection && strings.Contains(line, ":") && !strings.Contains(line, "Günlük Çalışma") {
			summary += line + "\n"
			projectCount++
		}

		if projectSection && strings.Contains(line, "Günlük Çalışma") {
			break
		}
	}

	if projectCount == 0 {
		summary += "Proje bilgisi bulunamadı.\n"
	}

	summary += "\nDetaylı rapor için uygulamayı çalıştırın."
	return summary
}

// Telegram mesajı gönder
func sendTelegramMessage(message string) {
	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Telegram mesajı gönderilemedi: %v", err)
	}
}

// Odoo'ya yeni zaman kaydı ekle
func addTimeEntry(date, project, task, description string, hours float64) error {
	client, uid, err := authenticateOdoo()
	if err != nil {
		return fmt.Errorf("Odoo kimlik doğrulama hatası: %v", err)
	}

	// Proje ID'sini bul
	projectID, err := findProjectID(client, uid, project)
	if err != nil {
		return fmt.Errorf("proje bulunamadı: %v", err)
	}

	// Görev ID'sini bul (opsiyonel)
	var taskID int64 = 0
	if task != "" {
		taskID, err = findTaskID(client, uid, projectID, task)
		if err != nil {
			return fmt.Errorf("görev bulunamadı: %v", err)
		}
	}

	// Zaman kaydı oluştur
	values := map[string]interface{}{
		"date":        date,
		"name":        description,
		"unit_amount": hours,
		"project_id":  projectID,
	}

	if taskID > 0 {
		values["task_id"] = taskID
	}

	var result int64
	err = client.Call("execute_kw", []interface{}{
		os.Getenv("ODOO_DB"),
		uid,
		os.Getenv("ODOO_PASSWORD"),
		"account.analytic.line",
		"create",
		[]interface{}{values},
	}, &result)

	if err != nil {
		return fmt.Errorf("zaman kaydı oluşturma hatası: %v", err)
	}

	return nil
}

// Proje ID'sini bul
func findProjectID(client *xmlrpc.Client, uid int, projectName string) (int64, error) {
	var records []interface{}
	err := client.Call("execute_kw", []interface{}{
		os.Getenv("ODOO_DB"),
		uid,
		os.Getenv("ODOO_PASSWORD"),
		"project.project",
		"search_read",
		[]interface{}{
			[]interface{}{
				[]interface{}{"name", "ilike", projectName},
			},
		},
		map[string]interface{}{
			"fields": []string{"id", "name"},
			"limit":  1,
		},
	}, &records)

	if err != nil {
		return 0, err
	}

	if len(records) == 0 {
		return 0, fmt.Errorf("proje bulunamadı: %s", projectName)
	}

	record := records[0].(map[string]interface{})

	// ID'yi doğru tipte almaya çalış
	if id, ok := record["id"].(int64); ok {
		return id, nil
	}

	// Eğer float64 ise
	if fid, ok := record["id"].(float64); ok {
		return int64(fid), nil
	}

	return 0, fmt.Errorf("geçersiz ID tipi: %T", record["id"])
}

// Görev ID'sini bul
func findTaskID(client *xmlrpc.Client, uid int, projectID int64, taskName string) (int64, error) {
	var records []interface{}
	err := client.Call("execute_kw", []interface{}{
		os.Getenv("ODOO_DB"),
		uid,
		os.Getenv("ODOO_PASSWORD"),
		"project.task",
		"search_read",
		[]interface{}{
			[]interface{}{
				[]interface{}{"name", "ilike", taskName},
				[]interface{}{"project_id", "=", projectID},
			},
		},
		map[string]interface{}{
			"fields": []string{"id", "name"},
			"limit":  1,
		},
	}, &records)

	if err != nil {
		return 0, err
	}

	if len(records) == 0 {
		return 0, fmt.Errorf("görev bulunamadı: %s", taskName)
	}

	record := records[0].(map[string]interface{})

	// ID'yi doğru tipte almaya çalış
	if id, ok := record["id"].(int64); ok {
		return id, nil
	}

	// Eğer float64 ise
	if fid, ok := record["id"].(float64); ok {
		return int64(fid), nil
	}

	return 0, fmt.Errorf("geçersiz ID tipi: %T", record["id"])
}

// Telegram bot mesajlarını dinle
func ListenForMessages() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Sadece belirli chat ID'den gelen mesajları işle
		if update.Message.Chat.ID != chatID {
			log.Printf("Yetkisiz erişim denemesi: %d", update.Message.Chat.ID)
			continue
		}

		go handleMessage(update.Message)
	}
}

// Gelen mesajları işle
func handleMessage(message *tgbotapi.Message) {
	text := message.Text

	// Komut kontrolü
	if strings.HasPrefix(text, "/") {
		handleCommand(message)
		return
	}

	// Zaman kaydı ekleme formatı: "YYYY-MM-DD|Proje|Görev|Açıklama|Saat"
	// Görev opsiyonel olabilir: "YYYY-MM-DD|Proje||Açıklama|Saat"
	parts := strings.Split(text, "|")
	if len(parts) == 5 {
		date := strings.TrimSpace(parts[0])
		project := strings.TrimSpace(parts[1])
		task := strings.TrimSpace(parts[2])
		description := strings.TrimSpace(parts[3])
		hoursStr := strings.TrimSpace(parts[4])

		hours, err := strconv.ParseFloat(hoursStr, 64)
		if err != nil {
			sendTelegramMessage("❌ Geçersiz saat formatı. Lütfen sayısal bir değer girin.")
			return
		}

		err = addTimeEntry(date, project, task, description, hours)
		if err != nil {
			sendTelegramMessage(fmt.Sprintf("❌ Zaman kaydı eklenemedi: %v", err))
			return
		}

		sendTelegramMessage(fmt.Sprintf("✅ Zaman kaydı başarıyla eklendi!\n\n📅 Tarih: %s\n🏢 Proje: %s\n📝 Görev: %s\n📋 Açıklama: %s\n⏱️ Saat: %.2f", date, project, task, description, hours))
	} else {
		sendTelegramMessage("❓ Anlaşılamayan mesaj formatı. Zaman kaydı eklemek için şu formatı kullanın:\n\n`YYYY-MM-DD|Proje|Görev|Açıklama|Saat`\n\nGörev alanı boş bırakılabilir:\n`YYYY-MM-DD|Proje||Açıklama|Saat`")
	}
}

// Komutları işle
func handleCommand(message *tgbotapi.Message) {
	command := message.Command()

	switch command {
	case "start":
		sendTelegramMessage("👋 Merhaba! Odoo Efor Takip botuna hoş geldiniz.\n\n" +
			"Komutlar:\n" +
			"/help - Yardım menüsünü gösterir\n" +
			"/today - Bugünün raporunu gösterir\n" +
			"/month - Bu ayın raporunu gösterir\n" +
			"/add - Zaman kaydı ekleme formatını gösterir")

	case "help":
		sendTelegramMessage("📚 *Yardım Menüsü*\n\n" +
			"*Komutlar:*\n" +
			"/today - Bugünün raporunu gösterir\n" +
			"/month - Bu ayın raporunu gösterir\n" +
			"/add - Zaman kaydı ekleme formatını gösterir\n\n" +
			"*Zaman Kaydı Ekleme:*\n" +
			"Yeni bir zaman kaydı eklemek için şu formatı kullanın:\n" +
			"`YYYY-MM-DD|Proje|Görev|Açıklama|Saat`\n\n" +
			"Görev alanı opsiyoneldir, boş bırakabilirsiniz:\n" +
			"`YYYY-MM-DD|Proje||Açıklama|Saat`")

	case "today":
		sendTelegramMessage("🔍 Bugünün raporu hazırlanıyor...")
		go func() {
			outputFileName, err := generateReport("daily", "", false)
			if err != nil {
				sendTelegramMessage(fmt.Sprintf("❌ Rapor oluşturulurken hata oluştu: %v", err))
				return
			}

			reportContent, err := os.ReadFile(outputFileName)
			if err != nil {
				sendTelegramMessage(fmt.Sprintf("❌ Rapor dosyası okunamadı: %v", err))
				return
			}

			summary := generateSummary(string(reportContent))
			sendTelegramMessage(summary)
		}()

	case "month":
		sendTelegramMessage("🔍 Bu ayın raporu hazırlanıyor...")
		go func() {
			outputFileName, err := generateReport("", "", false)
			if err != nil {
				sendTelegramMessage(fmt.Sprintf("❌ Rapor oluşturulurken hata oluştu: %v", err))
				return
			}

			reportContent, err := os.ReadFile(outputFileName)
			if err != nil {
				sendTelegramMessage(fmt.Sprintf("❌ Rapor dosyası okunamadı: %v", err))
				return
			}

			summary := generateSummary(string(reportContent))
			sendTelegramMessage(summary)
		}()

	case "add":
		sendTelegramMessage("➕ *Zaman Kaydı Ekleme*\n\n" +
			"Yeni bir zaman kaydı eklemek için şu formatı kullanın:\n" +
			"`YYYY-MM-DD|Proje|Görev|Açıklama|Saat`\n\n" +
			"*Örnek:*\n" +
			"`2025-02-07|TEKNOSA|CX-7006|Geliştirme yapıldı|3.5`\n\n" +
			"Görev alanı opsiyoneldir, boş bırakabilirsiniz:\n" +
			"`2025-02-07|TEKNOSA||Geliştirme yapıldı|3.5`")

	default:
		sendTelegramMessage("❓ Bilinmeyen komut. Yardım için /help yazın.")
	}
}
