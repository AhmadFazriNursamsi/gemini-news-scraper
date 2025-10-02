# News Scraper Katolik

![CATU Logo](assets/catuLogo.png)

README ini berisi panduan lengkap untuk instalasi, konfigurasi, dan penggunaan Gemini News Scraper yang dibangun dengan Go, Colly, PostgreSQL, dan Gemini AI.


# News Scraper (Go + Gemini AI)

Sistem otomatisasi scraping berita dari berbagai situs (contoh: Sesawi.net, Katolikana) dengan dukungan **Gemini AI** untuk merapikan konten. Hasil scrape disimpan ke database PostgreSQL dalam format siap tampil di mobile.

---

## 🚀 Features
- Scraping berita dengan [Colly](https://github.com/gocolly/colly).  
- Konversi HTML → Markdown → HTML rapi.  
- Refinement konten dengan **Gemini AI** (judul, isi, gambar tetap terjaga).  
- Hanya ambil berita terbaru (≤ 7 hari).  
- Insert ke PostgreSQL, bebas duplikasi (`ON CONFLICT DO NOTHING`).  
- Tambahan `newsAuthor` otomatis (penulis / fallback ke sumber).  
- Log progress 0–100% dengan summary.  
- CSS mobile-friendly inline di konten.  
- Scheduler otomatis dengan [robfig/cron](https://github.com/robfig/cron).  

---

## 🛠️ Prerequisites
Sebelum install, pastikan sudah terpasang:

- Go 1.22+  
- PostgreSQL (running local atau server)  
- API Key Google Gemini → [cara dapatkan](https://ai.google.dev/)  

---

## ⚙️ Installation Steps

### 1. Clone repository
```bash
git clone https://github.com/yourusername/news-scraper.git
cd news-scraper
```

### 2. Install dependencies
```bash
go mod tidy
```

### 3. Setup Database
Buat database `testing` di PostgreSQL:

```sql
CREATE DATABASE testing;
```

Tambahkan tabel `news`:

```sql
CREATE TABLE news (
    newsid SERIAL PRIMARY KEY,
    newsjudul TEXT NOT NULL,
    newssubjudul TEXT,
    newscontent TEXT,
    newssumber VARCHAR(100),
    newsauthor VARCHAR(200),
    newsurl TEXT UNIQUE,
    newstanggal TIMESTAMP,
    newscreated_date TIMESTAMP DEFAULT NOW(),
    newsread_count INT DEFAULT 0,
    newsstatus VARCHAR(20),
    newsurlimage TEXT
);
```

### 4. Buat file `.env`
```bash
GEMINI_API_KEY=your_api_key_here
```

### 5. Konfigurasi Sumber Berita (`sites.yaml`)
```yaml
- name: "Katolikana"
  list_url: "https://www.katolikana.com/"
  link_selector: "a"
  title_selector: "h1.entry-title"
  content_sel: "div.entry-content"
  image_sel: "meta[property='og:image']"
  date_meta: "meta[property='article:published_time'"

- name: "sesawi.net"
  list_url: "https://www.sesawi.net/"
  link_selector: "a"
  title_selector: "h1.entry-title"
  content_sel: "div.td-post-content"
  image_sel: "meta[property='og:image']"
  date_meta: "meta[property='article:published_time'"
```

### 6. Run Project
```bash
go run .
```

### 7. Contoh Output Log
```
⏰ Cek berita dari Katolikana: 2025-10-02 16:20:00
📊 Ditemukan 33 artikel baru di Katolikana

🔎 Artikel baru: https://www.katolikana.com/2025/10/01/artikel-terbaru
✅ Insert: Artikel Terbaru

⏹ Stop: artikel sudah lama, hentikan loop.

📊 Ringkasan Katolikana:
   ✅ Inserted: 1
   ⏩ Skipped : 0
   ⏳ Old/Stop: 1
------------------------------------
```

---

## 🚀 Production Deployment (Linux, systemd)

Buat service agar jalan otomatis:

```bash
sudo nano /etc/systemd/system/news-scraper.service
```

Isi config:

```ini
[Unit]
Description=News Scraper Service
After=network.target

[Service]
ExecStart=/usr/local/go/bin/go run /path/to/main.go
WorkingDirectory=/path/to
Restart=always
Environment=GEMINI_API_KEY=your_api_key_here
User=yourusername
Group=yourgroup

[Install]
WantedBy=multi-user.target
```

Enable & start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable news-scraper.service
sudo systemctl start news-scraper.service
```

---

## 📖 License
This project is licensed under the [MIT License](LICENSE).  
