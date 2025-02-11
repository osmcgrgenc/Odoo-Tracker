package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/smtp"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/kolo/xmlrpc"
	"github.com/schollz/progressbar/v3"
)

const (
	PageSize     = 100 // Her sayfada kaç kayıt olacağı
	MaxWorkers   = 5   // Paralel çalışacak goroutine sayısı
	MinWorkHours = 8.0
)

type TimeSheetEntry struct {
	Date        string      `xmlrpc:"date"`
	EmployeeID  interface{} `xmlrpc:"employee_id"`
	ProjectID   interface{} `xmlrpc:"project_id"`
	Description string      `xmlrpc:"name"`
	UnitAmount  float64     `xmlrpc:"unit_amount"`
}

// Sayfalama için yardımcı fonksiyon
func fetchTimeSheetPage(client *xmlrpc.Client, uid int, searchCriteria []interface{}, offset int) ([]TimeSheetEntry, error) {
	var records []TimeSheetEntry
	err := client.Call("execute_kw", []interface{}{
		os.Getenv("ODOO_DB"),
		uid,
		os.Getenv("ODOO_PASSWORD"),
		"account.analytic.line",
		"search_read",
		searchCriteria,
		map[string]interface{}{
			"fields": []string{"date", "employee_id", "project_id", "name", "unit_amount"},
			"limit":  PageSize,
			"offset": offset,
		},
	}, &records)
	return records, err
}

// Kayıtları işlemek için worker fonksiyonu
func processRecords(records []TimeSheetEntry, employeeFilter *string, employeeNames map[string]float64,
	resultChan chan<- map[string]interface{}, wg *sync.WaitGroup, writer io.Writer) {
	defer wg.Done()

	localStats := map[string]interface{}{
		"totalHours":    0.0,
		"employeeHours": make(map[string]float64),
		"projectHours":  make(map[string]float64),
		"dailyHours":    make(map[string]map[string]float64),
	}

	for _, entry := range records {
		employeeName := ""
		if emp, ok := entry.EmployeeID.([]interface{}); ok && len(emp) > 1 {
			employeeName = emp[1].(string)
		}

		if *employeeFilter != "" && employeeName != *employeeFilter {
			continue
		} else if _, ok := employeeNames[employeeName]; !ok && *employeeFilter == "" {
			continue
		}

		projectName := ""
		if proj, ok := entry.ProjectID.([]interface{}); ok && len(proj) > 1 {
			projectName = proj[1].(string)
		}

		hours := localStats["totalHours"].(float64)
		localStats["totalHours"] = hours + entry.UnitAmount

		empHours := localStats["employeeHours"].(map[string]float64)
		empHours[employeeName] += entry.UnitAmount
		employeeNames[employeeName] += entry.UnitAmount
		projHours := localStats["projectHours"].(map[string]float64)
		projHours[projectName] += entry.UnitAmount

		dailyHours := localStats["dailyHours"].(map[string]map[string]float64)
		if dailyHours[entry.Date] == nil {
			dailyHours[entry.Date] = make(map[string]float64)
		}
		dailyHours[entry.Date][employeeName] += entry.UnitAmount

		fmt.Fprintf(writer, "Tarih: %s\nÇalışan: %s\nProje: %s\nAçıklama: %s\nSaat: %.2f\n\n",
			entry.Date, employeeName, projectName, entry.Description, entry.UnitAmount)
	}

	resultChan <- localStats
}

func sendEmail(content string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")
	to := "osman.cagri.genc@enoca.com"

	// Gerekli alanları kontrol et
	if host == "" || port == "" || username == "" || password == "" || from == "" {
		return fmt.Errorf("SMTP ayarları eksik. Host: %s, Port: %s, Username: %s, From: %s",
			host, port, username, from)
	}

	subject := "Günlük Odoo Kayıtları"
	date := time.Now().Format("2006-01-02")

	message := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s - %s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", to, subject, date, content))

	auth := smtp.PlainAuth("", username, password, host)
	addr := fmt.Sprintf("%s:%s", host, port)

	fmt.Printf("E-posta gönderiliyor...\n")
	fmt.Printf("SMTP Sunucu: %s:%s\n", host, port)
	fmt.Printf("Gönderen: %s\n", from)
	fmt.Printf("Alıcı: %s\n", to)

	err := smtp.SendMail(addr, auth, from, []string{to}, message)
	if err != nil {
		return fmt.Errorf("E-posta gönderme hatası: %v", err)
	}

	return nil
}

func main() {
	employeeFilter := flag.String("employee", "", "Çalışan adına göre filtrele (boş bırakılırsa tüm çalışanlar)")
	dateFilter := flag.String("date", "", "Tarih filtresi ('daily' bugünü, 'YYYY-MM-DD' belirli bir günü filtreler)")
	sendMailFlag := flag.Bool("sendMail", false, "Raporu e-posta olarak gönder")
	flag.Parse()

	// Çıktı dosyasını oluştur
	now := time.Now()
	if err := os.MkdirAll("results", 0755); err != nil {
		log.Fatal("Results klasörü oluşturulamadı:", err)
	}
	outputFileName := fmt.Sprintf("results/result_%s.txt", now.Format("2006-01-02_15-04-05"))
	outputFile, err := os.Create(outputFileName)
	if err != nil {
		log.Fatal("Çıktı dosyası oluşturulamadı:", err)
	}
	defer outputFile.Close()

	// Çıktıları hem dosyaya hem de konsola yazmak için multiwriter kullan
	writer := io.MultiWriter(os.Stdout, outputFile)

	// Önce .env.local dosyasını dene, yoksa .env dosyasını kullan
	err = godotenv.Load(".env.local")
	if err != nil {
		err = godotenv.Load()
		if err != nil {
			fmt.Fprintln(writer, "Error loading .env file")
			log.Fatal(err)
		}
	}

	commonClient, err := xmlrpc.NewClient(fmt.Sprintf("%s/xmlrpc/2/common", os.Getenv("ODOO_BASE_URL")), nil)
	if err != nil {
		log.Fatal(err)
	}

	var uid int
	err = commonClient.Call("authenticate", []interface{}{
		os.Getenv("ODOO_DB"),
		os.Getenv("ODOO_USERNAME"),
		os.Getenv("ODOO_PASSWORD"),
		map[string]interface{}{},
	}, &uid)
	if err != nil {
		log.Fatal("Authentication error:", err)
	}

	client, err := xmlrpc.NewClient(fmt.Sprintf("%s/xmlrpc/2/object", os.Getenv("ODOO_BASE_URL")), nil)
	if err != nil {
		log.Fatal(err)
	}

	// Tarih filtresini ayarla
	var startDate, endDate time.Time
	if *dateFilter == "daily" {
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endDate = startDate
	} else if *dateFilter != "" {
		startDate, err = time.Parse("2006-01-02", *dateFilter)
		if err != nil {
			log.Fatal("Geçersiz tarih formatı. Doğru format: YYYY-MM-DD")
		}
		endDate = startDate
	} else {
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endDate = now
	}

	fmt.Fprintf(writer, "Tarih Aralığı: %s - %s\n", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	searchCriteria := []interface{}{
		[]interface{}{
			[]interface{}{"date", ">=", startDate.Format("2006-01-02")},
			[]interface{}{"date", "<=", endDate.Format("2006-01-02")},
		},
	}

	// İzin verilen çalışanlar listesi
	employeeNames := map[string]float64{
		"Osman Çağrı GENÇ":  0.0,
		"Ayşegül Şahin":     0.0,
		"Fatih Delice":      0.0,
		"Uğurcan Şen":       0.0,
		"Onur Akın":         0.0,
		"Osman Topuz":       0.0,
		"Ümmühan Keleş":     0.0,
		"Esma Harmancı":     0.0,
		"Esra Çavdar":       0.0,
		"İpek Coşkun":       0.0,
		"Mihriban Evren":    0.0,
		"Ahmet Yağız Özbak": 0.0,
		"Ebrar Betül Akgül": 0.0,
	}

	// Toplam kayıt sayısını al
	var count int
	err = client.Call("execute_kw", []interface{}{
		os.Getenv("ODOO_DB"),
		uid,
		os.Getenv("ODOO_PASSWORD"),
		"account.analytic.line",
		"search_count",
		searchCriteria,
	}, &count)
	if err != nil {
		log.Fatal("Error getting record count:", err)
	}

	fmt.Printf("Toplam kayıt sayısı: %d\n", count)

	// Progress bar oluştur
	bar := progressbar.NewOptions(count,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(false),
		progressbar.OptionSetWidth(15),
		progressbar.OptionSetDescription("[cyan]Veriler işleniyor...[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))

	// Worker havuzu ve sonuç kanalı oluştur
	var wg sync.WaitGroup
	resultChan := make(chan map[string]interface{}, MaxWorkers)
	progressChan := make(chan int, MaxWorkers)

	// Progress bar güncelleme goroutine'i
	go func() {
		for range progressChan {
			bar.Add(1)
		}
	}()

	// Sayfaları paralel işle
	for offset := 0; offset < count; offset += PageSize {
		records, err := fetchTimeSheetPage(client, uid, searchCriteria, offset)
		if err != nil {
			log.Printf("Error fetching page at offset %d: %v", offset, err)
			continue
		}

		wg.Add(1)
		go func(recs []TimeSheetEntry) {
			processRecords(recs, employeeFilter, employeeNames, resultChan, &wg, writer)
			progressChan <- len(recs)
		}(records)
	}

	// Sonuçları topla
	go func() {
		wg.Wait()
		close(resultChan)
		close(progressChan)
		fmt.Println() // Progress bar'dan sonra yeni satır
	}()

	// Global istatistikler
	totalHours := 0.0
	employeeHours := make(map[string]float64)
	projectHours := make(map[string]float64)
	dailyHours := make(map[string]map[string]float64)

	// Sonuçları birleştir
	for result := range resultChan {
		totalHours += result["totalHours"].(float64)

		for emp, hours := range result["employeeHours"].(map[string]float64) {
			employeeHours[emp] += hours
		}

		for proj, hours := range result["projectHours"].(map[string]float64) {
			projectHours[proj] += hours
		}

		for date, empMap := range result["dailyHours"].(map[string]map[string]float64) {
			if dailyHours[date] == nil {
				dailyHours[date] = make(map[string]float64)
			}
			for emp, hours := range empMap {
				dailyHours[date][emp] += hours
			}
		}
	}

	// Özet istatistikleri yazdır
	fmt.Fprintln(writer, "\n=== Özet İstatistikler ===\n")
	fmt.Fprintf(writer, "Toplam Çalışma Saati: %.2f\n\n", totalHours)

	if *employeeFilter != "" {
		// Tek bir çalışan için filtreleme yapıldığında
		fmt.Fprintf(writer, "Çalışan: %s\n\n", *employeeFilter)
	} else {
		// Tüm çalışanların toplam saatlerini listele
		fmt.Fprintln(writer, "Çalışan Bazında Toplam Saatler:")
		for emp, hours := range employeeHours {
			fmt.Fprintf(writer, "%s: %.2f saat\n", emp, hours)
		}
	}

	fmt.Fprintln(writer, "\nProje Bazında Saatler:")
	for proj, hours := range projectHours {
		fmt.Fprintf(writer, "%s: %.2f saat\n", proj, hours)
	}

	// Tarihleri sırala
	var dates []string
	for date := range dailyHours {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	fmt.Fprintln(writer, "\nGünlük Çalışma Saatleri:")
	for _, date := range dates {
		fmt.Fprintf(writer, "\n%s:\n", date)
		employeeHoursList := make([]float64, 0, len(dailyHours[date]))
		for _, hours := range dailyHours[date] {
			employeeHoursList = append(employeeHoursList, hours)
		}
		sort.Float64s(employeeHoursList)

		for emp, hours := range dailyHours[date] {
			if *employeeFilter == "" || emp == *employeeFilter {
				status := "Tam Çalışmış"
				if hours < MinWorkHours {
					status = "Az Çalışmış"
				} else if hours == employeeHoursList[len(employeeHoursList)-1] {
					status = "En ÇOK Çalışmış"
				}
				fmt.Fprintf(writer, "  %s: %.2f saat ====== %s\n", emp, hours, status)
			}
		}
	}
	for employeeName, hours := range employeeNames {
		fmt.Fprintf(writer, "%s: %.2f saat\n", employeeName, hours)
	}

	fmt.Printf("\nRapor %s dosyasına kaydedildi.\n", outputFileName)

	// E-posta gönderme kontrolü
	if *sendMailFlag {
		reportContent, err := os.ReadFile(outputFileName)
		if err != nil {
			log.Printf("Rapor dosyası okunamadı: %v", err)
		} else {
			err = sendEmail(string(reportContent))
			if err != nil {
				log.Printf("E-posta gönderilemedi: %v", err)
			} else {
				fmt.Println("Rapor e-posta olarak gönderildi.")
			}
		}
	}
}
