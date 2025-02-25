# Odoo Efor Takip Uygulaması

Bu uygulama, Odoo üzerindeki zaman çizelgesi (timesheet) kayıtlarını görüntülemek ve analiz etmek için geliştirilmiş bir Go uygulamasıdır. Çalışanların günlük, aylık ve proje bazlı çalışma saatlerini takip eder, detaylı raporlar oluşturur ve isteğe bağlı olarak e-posta ile paylaşır.

## Özellikler

### Zaman Takibi
- Aylık zaman çizelgesi görüntüleme
- Günlük zaman çizelgesi görüntüleme
- Belirli bir güne ait kayıtları filtreleme
- Çalışan bazında filtreleme
- Proje bazında filtreleme

### Raporlama
- Günlük çalışma saati raporları
- Çalışan bazında toplam saat raporları
- Proje bazında toplam saat raporları
- Günlük hedef çalışma saati (8 saat) kontrolü
- Az/çok çalışma durumu analizi
- Detaylı istatistikler ve özetler

### Kullanıcı Deneyimi
- İlerleme çubuğu ile gerçek zamanlı işlem durumu takibi
- Renkli ve okunaklı konsol çıktısı
- Paralel işlem ile hızlı veri işleme
- Hata durumunda detaylı bilgilendirme

### Dosya ve E-posta İşlemleri
- Sonuçları tarih/saat etiketli dosyalara kaydetme
- İsteğe bağlı otomatik e-posta raporlama
- SMTP üzerinden güvenli e-posta gönderimi
- Özelleştirilebilir rapor formatı

### Telegram Bot Entegrasyonu
- Zamanlanmış otomatik raporlar (sabah 8:00 ve akşam 18:00)
- Telegram üzerinden rapor sorgulama
- Telegram üzerinden Odoo'ya zaman kaydı ekleme
- Komut tabanlı etkileşimli arayüz

## Kurulum

### Ön Gereksinimler
- Go 1.21 veya üzeri
- Git
- Odoo erişimi
- SMTP sunucu erişimi (e-posta gönderimi için)
- Telegram Bot Token (Telegram özellikleri için)
- İnternet bağlantısı

### Kurulum Adımları

1. Projeyi klonlayın:

```bash
git clone https://github.com/yourusername/odoo-efor-tracker.git
cd odoo-efor-tracker
```

2. Gerekli bağımlılıkları yükleyin:

```bash
go mod tidy
```

3. Gerekli klasörleri oluşturun:

```bash
mkdir results
```

4. Yapılandırma dosyalarını oluşturun:

   a. Örnek `.env` dosyasını kopyalayın:
   ```bash
   cp .env.example .env.local
   ```

   b. `.env.local` dosyasını düzenleyin:
   ```env
   # Odoo Bağlantı Bilgileri
   ODOO_DB=your_database
   ODOO_USERNAME=your_username
   ODOO_PASSWORD=your_password
   ODOO_BASE_URL=https://your-odoo-instance.com

   # E-posta Ayarları
   SMTP_HOST=smtp.your-mail-server.com    # Örn: smtp.gmail.com, smtp.yandex.com
   SMTP_PORT=587                          # TLS için 587, SSL için 465
   SMTP_USERNAME=your_email@domain.com
   SMTP_PASSWORD=your_app_password        # Gmail için App Password gerekli
   SMTP_FROM=your_email@domain.com
   
   # Telegram Bot Ayarları
   TELEGRAM_BOT_TOKEN=your_bot_token      # BotFather'dan alınan token
   TELEGRAM_CHAT_ID=your_chat_id          # Yetkili kullanıcının chat ID'si
   ```

## Kullanım

### Temel Kullanım

1. Aylık Rapor (Varsayılan):
```bash
go run main.go
```

2. Belirli Bir Çalışanın Raporu:
```bash
go run main.go -employee "Çalışan Adı"
```

### Tarih Filtreleme

1. Bugünün Raporu:
```bash
go run main.go -date daily
```

2. Belirli Bir Günün Raporu:
```bash
go run main.go -date "2025-02-01"
```

### E-posta Gönderimi

1. Raporu E-posta ile Gönderme:
```bash
go run main.go -sendMail
```

2. Filtrelenmiş Raporu E-posta ile Gönderme:
```bash
go run main.go -employee "Çalışan Adı" -date "2025-02-01" -sendMail
```

### Telegram Bot Kullanımı

1. Telegram Bot'unu Başlatma:
```bash
go run main.go -telegram
```

2. Bot Komutları:
   - `/start` - Bot'u başlatır ve karşılama mesajı gönderir
   - `/help` - Yardım menüsünü gösterir
   - `/today` - Bugünün raporunu gösterir
   - `/month` - Bu ayın raporunu gösterir
   - `/add` - Zaman kaydı ekleme formatını gösterir

3. Zaman Kaydı Ekleme:
   Telegram üzerinden aşağıdaki formatta mesaj göndererek yeni zaman kaydı ekleyebilirsiniz:
   ```
   YYYY-MM-DD|Proje|Görev|Açıklama|Saat
   ```
   
   Örnek:
   ```
   2025-02-07|TEKNOSA|CX-7006|Geliştirme yapıldı|3.5
   ```
   
   Görev alanı opsiyoneldir, boş bırakabilirsiniz:
   ```
   2025-02-07|TEKNOSA||Geliştirme yapıldı|3.5
   ```

## Parametreler

### Zorunlu Olmayan Parametreler
- `-employee`: Belirli bir çalışanın kayıtlarını filtreler
  - Örnek: `-employee "Osman Çağrı GENÇ"`
  - Boş bırakılırsa: Tüm çalışanları gösterir

- `-date`: Tarih filtresi
  - `daily`: Bugünün kayıtlarını gösterir
  - `YYYY-MM-DD`: Belirtilen günün kayıtlarını gösterir
  - Boş bırakılırsa: Ayın başından bugüne kadar olan kayıtları gösterir

- `-sendMail`: Raporu e-posta olarak gönderir
  - Parametre değeri gerekmez
  - Kullanılmazsa: E-posta gönderimi yapılmaz

- `-telegram`: Telegram bot'unu başlatır
  - Parametre değeri gerekmez
  - Bot başlatıldığında, zamanlanmış görevler ve mesaj dinleme aktif olur

## Çıktı Formatı

### Zaman Çizelgesi Raporu
- **Tarih**: YYYY-MM-DD formatında
- **Çalışan**: Tam ad
- **Proje**: Proje adı
- **Açıklama**: Yapılan iş açıklaması
- **Saat**: Decimal format (örn: 7.50 saat)

### Özet İstatistikler
1. Genel Özet
   - Toplam çalışma saati
   - Tarih aralığı
   - Kayıt sayısı

2. Çalışan Bazında Özet
   - Her çalışanın toplam saati
   - Günlük çalışma durumu (Az/Tam/Çok)
   - En çok çalışan personel vurgusu

3. Proje Bazında Özet
   - Her projenin toplam saati
   - Proje katılımcıları

## Çıktı Dosyaları

### Dosya Yapısı
- Konum: `results/` klasörü
- İsimlendirme: `result_YYYY-MM-DD_HH-mm-ss.txt`
- Format: UTF-8 metin dosyası

### E-posta Raporu
- Konu: "Günlük Odoo Kayıtları - YYYY-MM-DD"
- Format: Düz metin
- İçerik: Dosya ile aynı içerik

### Telegram Raporu
- Format: Markdown formatında özet
- İçerik: Tarih aralığı, toplam çalışma saati, çalışan ve proje bazında saatler

## Zamanlanmış Görevler

Telegram bot modu aktif edildiğinde, aşağıdaki zamanlanmış görevler otomatik olarak çalışır:

1. **Sabah Raporu**: Her gün sabah 08:00'de, o günün başlangıç raporu gönderilir.
2. **Akşam Raporu**: Her gün akşam 18:00'de, günün özet raporu gönderilir.

Bu raporlar, Telegram üzerinden belirtilen chat ID'ye otomatik olarak gönderilir.

## Telegram Bot Kurulumu

1. Telegram'da [@BotFather](https://t.me/BotFather) ile konuşarak yeni bir bot oluşturun.
2. Bot token'ını alın ve `.env.local` dosyasına ekleyin.
3. Bot ile özel mesaj başlatın ve bir mesaj gönderin.
4. Tarayıcınızda `https://api.telegram.org/bot<TOKEN>/getUpdates` adresini ziyaret edin (TOKEN yerine kendi token'ınızı yazın).
5. Gelen JSON yanıtından chat ID'nizi bulun ve `.env.local` dosyasına ekleyin.

## Hata Ayıklama

### Sık Karşılaşılan Hatalar
1. SMTP Bağlantı Hataları
   - SMTP ayarlarını kontrol edin
   - Port numarasının doğruluğunu kontrol edin
   - Uygulama şifresinin doğruluğunu kontrol edin

2. Odoo Bağlantı Hataları
   - Ağ bağlantısını kontrol edin
   - Kimlik bilgilerinin doğruluğunu kontrol edin
   - Session ID'nin güncelliğini kontrol edin

3. Telegram Bot Hataları
   - Bot token'ının doğruluğunu kontrol edin
   - Chat ID'nin doğruluğunu kontrol edin
   - Botun gerekli izinlere sahip olduğunu kontrol edin

## Bağımlılıklar

### Temel Paketler
- `github.com/joho/godotenv`: Ortam değişkenleri yönetimi
- `github.com/kolo/xmlrpc`: Odoo API iletişimi
- `github.com/schollz/progressbar/v3`: İlerleme çubuğu
- `github.com/go-telegram-bot-api/telegram-bot-api/v5`: Telegram Bot API
- `github.com/robfig/cron/v3`: Zamanlanmış görevler

### Sistem Gereksinimleri
- İşletim Sistemi: Linux, macOS, Windows
- RAM: Minimum 512MB
- Disk: Minimum 100MB boş alan

## Güvenlik

### Hassas Bilgiler
- `.env.local` dosyası `.gitignore`'a eklenmiştir
- Şifreler ve API anahtarları asla kaynak kodda saklanmaz
- SMTP iletişimi TLS/SSL ile şifrelenir
- Telegram bot sadece belirtilen chat ID'den gelen mesajları işler

### En İyi Uygulamalar
- Düzenli olarak Odoo ve SMTP şifrelerini güncelleyin
- Uygulama şifresi kullanın (özellikle Gmail için)
- Hassas dosyaları yedekleyin
- Telegram bot token'ını kimseyle paylaşmayın

## Lisans

Bu proje MIT lisansı altında lisanslanmıştır. Detaylar için `LICENSE` dosyasına bakınız.
