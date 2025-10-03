package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	h2m "github.com/JohannesKaufmann/html-to-markdown"

	"github.com/google/generative-ai-go/genai"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	// "github.com/google/generative-ai-go/genai"

	// "github.com/google/generative-ai-go/genai"

	// "github.com/yuin/goldmark"

	"github.com/gocolly/colly"
	// "github.com/google/generative-ai-go/genai"
	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
	"google.golang.org/api/option"
	"gopkg.in/yaml.v3"
)

// ===== Struct =====
type News struct {
	NewsJudul    string
	NewsSubJudul string
	NewsContent  string
	NewsSumber   string
	NewsURL      string
	NewsTanggal  time.Time
	NewsURLImage string
	NewsStatus   string
	NewsAuthor   string
}

type SiteConfig struct {
	Name          string `yaml:"name"`
	ListURL       string `yaml:"list_url"`
	LinkSelector  string `yaml:"link_selector"`
	TitleSelector string `yaml:"title_selector"`
	ContentSel    string `yaml:"content_sel"`
	ImageSel      string `yaml:"image_sel"`
	DateMeta      string `yaml:"date_meta"`
}

// ======== LLM Utility ========
type NewsMeta struct {
	NewsJudul     string    `json:"NewsJudul"`
	NewsSubJudul  string    `json:"NewsSubJudul"`
	NewsSumber    string    `json:"Newssumber"`
	NewsAuthor    string    `json:"NewsAuthor"` // ✅ Tambahkan ini
	NewsTanggal   time.Time `json:"Newstanggal"`
	NewsCreated   time.Time `json:"Newscreated_date"`
	NewsReadCount int       `json:"Newsread_count"`
	NewsStatus    int       `json:"NewsStatus"`
	NewsURLImage  string    `json:"Newsurlimage"`
}

// htmlTemplate gabungkan metadata + konten markdown (sudah di-convert ke HTML)
func htmlTemplate(contentHTML string, meta News) string {
	return fmt.Sprintf(`
    <!DOCTYPE html>
    <html>
    <head>
        <meta charset="utf-8" name="viewport" content="initial-scale=1, width=device-width">
        <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500&display=swap"/>
        <style>
        body { margin: 0; font-family: Inter; }
        .container { width: 100%%; background-color: #fff; text-align: left; }
        .container-judul { 
            padding: 24px 16px; 
            display: flex; 
            flex-direction: column; 
            gap: 16px; 
            color: #1d5089; 
        }
        .sumber { line-height: 20px; font-weight: 500; }
        .judul { 
            font-size: 18px; 
            line-height: 26px; 
            font-weight: 500; 
            letter-spacing: -0.02em; 
            color: #000; 
        }
        .tanggal { line-height: 20px; color: rgba(0, 0, 0, 0.35); }
        .section {
            padding: 24px 16px; 
            color: rgba(0, 0, 0, 0.6); 
        }
        img, iframe { width: 100%% !important; object-fit: cover; max-height: 600px !important;}
        .link { text-decoration: underline; color: rgba(0, 0, 0, 0.6); }
        p { margin-block-end: 18px; }
        .paragraph { line-height: 24px; }
        a {color: rgba(0, 0, 0, 0.6);}
        h2 {font-size:18px}
        </style>
    </head>
    <body>
        <div class="container">
            <div class="container-judul">
                <span class="sumber">%s</span>
                <h1 class="judul">%s</h1>
				<span class="author">By: <b>%s</b> %s</span>
            </div>
            <div class="section">
                <div class="paragraph">%s</div>
            </div>
        </div>
    </body>
    </html>`,
		meta.NewsSumber,
		meta.NewsJudul,
		meta.NewsAuthor,
		meta.NewsTanggal.Format("January 2, 2006"),
		contentHTML,
	)
}

// Ambil metadata (judul, tanggal, dsb)
func extractMetaWithGemini(apiKey, rawHTML, siteName string) (NewsMeta, error) {
	// 🚨 Limit panjang HTML biar hemat token
	if len(rawHTML) > 4000 {
		rawHTML = rawHTML[:4000] + "...(dipotong)"
	}

	ctx := context.Background()
	client, _ := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	defer client.Close()

	model := client.GenerativeModel("models/gemini-1.5-flash")
	prompt := fmt.Sprintf(`
Ekstrak metadata artikel dari HTML berikut. 
Jawab dalam format JSON dengan field:
- NewsJudul (judul artikel)
- NewsSubJudul (subtitle: 1 paragraf pertama, maksimal 200 karakter)
- Newssumber = "%s"
- NewsAuthor (nama penulis jika ada, kalau tidak ada isi dengan sumber)
- Newstanggal (format "2006-01-02 15:04:05" atau gunakan waktu sekarang jika tidak ada)
- Newscreated_date = waktu sekarang
- Newsread_count = 0
- NewsStatus = "publish"
- Newsurlimage = URL gambar utama (kalau ada)
`, siteName)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt+"\n\n"+rawHTML))
	if err != nil {
		return NewsMeta{}, err
	}
	var meta NewsMeta
	if err := json.Unmarshal([]byte(resp.Candidates[0].Content.Parts[0].(genai.Text)), &meta); err != nil {
		return NewsMeta{}, err
	}
	return meta, nil
}

// Ambil konten utama jadi Markdown
func extractContentWithGemini(apiKey, rawHTML string) (string, error) {
	// 🚨 Limit panjang HTML biar hemat token
	if len(rawHTML) > 4000 {
		rawHTML = rawHTML[:4000] + "...(dipotong)"
	}

	ctx := context.Background()
	client, _ := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	defer client.Close()

	model := client.GenerativeModel("models/gemini-1.5-flash")

	prompt := `
Isi Artikel seperti Paragraf, Kalimat atau Image dalam FORMAT MARKDOWN! 
Note: Judul tidak termasuk, cukup isinya saja. Output yang diharapkan adalah Markdown file valid.
- Tidak boleh ada gambar di paragraf pertama, hanya mulai paragraf 2 atau ketiga.
- Jangan sertakan bagian author atau penulis (foto, nama, link author, atau bio penulis).
- Jika artikel memiliki blok "About the Author" atau "saboxplugin-wrap", abaikan seluruh isinya.
- Output hanya isi artikel tanpa bagian penulis.
`

	resp, err := model.GenerateContent(ctx, genai.Text(prompt+"\n\n"+rawHTML))
	if err != nil {
		return "", err
	}
	return string(resp.Candidates[0].Content.Parts[0].(genai.Text)), nil
}

// Bersihkan author box (Katolikana)
func cleanAuthorInfo(html string) string {
	// Regex: <p><img ...></p><p><a href="/author/...">...</p><p>...</p>
	re := regexp.MustCompile(`(?s)<p><img[^>]+></p>\s*<p><a[^>]+/author/[^>]*>.*?</a></p>\s*<p>.*?</p>`)
	cleaned := re.ReplaceAllString(html, "")

	return cleaned
}

// ===== Load config dari YAML =====
func loadSitesConfig(path string) ([]SiteConfig, error) {
	var sites []SiteConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, &sites)
	return sites, err
}

// ===== Utility =====

// render progress bar ascii
func printProgress(current, total int) {
	percent := float64(current) / float64(total)
	barWidth := 30 // panjang bar
	filled := int(percent * float64(barWidth))

	bar := strings.Repeat("█", filled) + strings.Repeat("-", barWidth-filled)
	fmt.Printf("\r[%s] %3.0f%% (%d/%d)", bar, percent*100, current, total)

	if current == total {
		fmt.Println() // newline setelah selesai
	}
}

// ===== Cek URL =====
func isLikelyNewsURL(link string, siteName string) bool {
	switch siteName {
	case "Katolikana":
		pattern := regexp.MustCompile(`/\d{4}/\d{2}/\d{2}/`)
		return pattern.MatchString(link)
	case "sesawi.net":
		if strings.Contains(link, "/category/") ||
			strings.Contains(link, "/tag/") ||
			strings.Contains(link, "/author/") ||
			strings.Contains(link, "/wp-") ||
			link == "https://www.sesawi.net/" || link == "https://www.sesawi.net" {
			return false
		}
		return strings.Contains(link, "sesawi.net/")
	default:
		return true
	}
}

// ===== DB =====
func isNewsExist(db *sql.DB, url string) bool {
	var id int
	err := db.QueryRow("SELECT newsid FROM news WHERE newsurl=$1", url).Scan(&id)
	return err == nil
}

func insertNews(db *sql.DB, n News) error {
	if n.NewsJudul == "" || n.NewsContent == "" {
		fmt.Println("⚠️ Skip insert karena judul/konten kosong:", n.NewsURL)
		return nil
	}
	query := `
INSERT INTO news 
(newsjudul, newssubjudul, newscontent, newssumber, newsurl, newstanggal, newscreated_date, newsread_count, newsstatus, newsurlimage, newsauthor)
VALUES ($1,$2,$3,$4,$5,$6,NOW(),0,$7,$8,$9)
ON CONFLICT (newsurl) DO NOTHING;`

	_, err := db.Exec(query,
		n.NewsJudul,
		n.NewsSubJudul,
		n.NewsContent,
		n.NewsSumber,
		n.NewsURL,
		n.NewsTanggal,
		n.NewsStatus,
		n.NewsURLImage,
		n.NewsAuthor,
	)

	fmt.Printf("✅ Insert: %s | %s\n", n.NewsJudul, n.NewsURL)
	return err
}
func markdownToHTML(md string) string {
	var buf bytes.Buffer

	mdParser := goldmark.New(
		// aktifkan extension penting
		goldmark.WithExtensions(
			extension.GFM,           // GitHub Flavored Markdown (table, strikethrough, autolink)
			extension.Linkify,       // auto deteksi link
			extension.Table,         // tabel
			extension.Strikethrough, // ~~strike~~
			extension.TaskList,      // [ ] [x] list task
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(), // newline = <br>
			html.WithXHTML(),     // output xhtml self-closing
			html.WithUnsafe(),    // izinkan raw HTML di dalam markdown
		),
	)

	if err := mdParser.Convert([]byte(md), &buf); err != nil {
		log.Println("❌ Gagal convert markdown:", err)
		return md // fallback: kembalikan markdown mentah
	}
	return buf.String()
}
func htmlToMarkdown(html string) string {
	converter := h2m.NewConverter("", true, nil)
	md, err := converter.ConvertString(html)
	if err != nil {
		log.Println("⚠️ Gagal konversi HTML ke Markdown:", err)
		return html
	}
	return md
}
func scrapeArticle(cfg SiteConfig, articleURL string, apiKey string) (News, error) {
	var news News
	news.NewsStatus = "publish"
	news.NewsURL = articleURL
	news.NewsSumber = cfg.Name

	c := colly.NewCollector(colly.UserAgent("Mozilla/5.0"))
	c.SetRequestTimeout(30 * time.Second)

	// Judul
	c.OnHTML(cfg.TitleSelector, func(e *colly.HTMLElement) {
		news.NewsJudul = strings.TrimSpace(e.Text)
	})

	// Gambar
	c.OnHTML("meta[property='og:image']", func(e *colly.HTMLElement) {
		if news.NewsURLImage == "" {
			news.NewsURLImage = e.Attr("content")
		}
	})

	// Tanggal
	c.OnHTML(cfg.DateMeta, func(e *colly.HTMLElement) {
		if t, err := time.Parse(time.RFC3339, e.Attr("content")); err == nil {
			news.NewsTanggal = t
		}
	})
	// Author
	c.OnHTML("meta[name='author']", func(e *colly.HTMLElement) {
		if news.NewsAuthor == "" {
			news.NewsAuthor = strings.TrimSpace(e.Attr("content"))
		}
	})
	// contoh fallback kalau situs tidak pakai meta author
	c.OnHTML(".author, .post-author, .byline", func(e *colly.HTMLElement) {
		if news.NewsAuthor == "" {
			news.NewsAuthor = strings.TrimSpace(e.Text)
		}
	})
	// Konten
	c.OnHTML(cfg.ContentSel, func(e *colly.HTMLElement) {
		html, _ := e.DOM.Html()

		// === Gunakan 2 tahap ===
		meta, err := extractMetaWithGemini(apiKey, html, cfg.Name)
		if err == nil {
			news.NewsJudul = meta.NewsJudul
			news.NewsSubJudul = meta.NewsSubJudul
			news.NewsURLImage = meta.NewsURLImage
			news.NewsAuthor = meta.NewsAuthor
			news.NewsTanggal = meta.NewsTanggal
		}

		md, err := extractContentWithGemini(apiKey, html)
		if err != nil {
			// fallback: lokal saja
			md = htmlToMarkdown(html)
		}
		contentHTML := markdownToHTML(md)
		// Bersihkan author info khusus Katolikana

		if cfg.Name == "Katolikana" {
			contentHTML = cleanAuthorInfo(contentHTML)
		}

		news.NewsContent = htmlTemplate(contentHTML, news)

	})

	// visit artikel
	if err := c.Visit(articleURL); err != nil {
		return news, err
	}
	c.Wait()
	if news.NewsAuthor == "" {
		news.NewsAuthor = news.NewsSumber
	}

	// Kalau tanggal kosong, isi dengan sekarang
	if news.NewsTanggal.IsZero() {
		news.NewsTanggal = time.Now()
	}

	// 🚨 Filter: hanya artikel 3 hari terakhir
	if news.NewsTanggal.Before(time.Now().AddDate(0, 0, -7)) {
		return news, fmt.Errorf("artikel lebih lama dari 7 hari: %s", news.NewsTanggal.Format("2006-01-02"))
	}

	return news, nil
}

// ===== Scrape List =====
func scrapeListAndInsert(db *sql.DB, cfg SiteConfig, apiKey string) {
	fmt.Printf("⏰ Cek berita dari %s: %s\n", cfg.Name, time.Now().Format("2006-01-02 15:04:05"))

	var inserted, skipped, old int

	// ambil semua link
	newLinks := []string{}
	c := colly.NewCollector(colly.UserAgent("Mozilla/5.0"), colly.Async(true))
	c.SetRequestTimeout(30 * time.Second)

	c.OnHTML("a", func(e *colly.HTMLElement) {
		link := strings.TrimSpace(e.Attr("href"))
		if link == "" || !isLikelyNewsURL(link, cfg.Name) {
			return
		}
		link = e.Request.AbsoluteURL(link)
		if !isNewsExist(db, link) {
			newLinks = append(newLinks, link)
		}
	})

	c.Visit(cfg.ListURL)
	c.Wait()

	total := len(newLinks)
	if total == 0 {
		fmt.Println("📭 Tidak ada artikel baru.")
		return
	}
	fmt.Printf("📊 Ditemukan %d artikel baru di %s\n", total, cfg.Name)

	for i, link := range newLinks {
		fmt.Println("\n🔎 Artikel baru:", link)

		article, err := scrapeArticle(cfg, link, apiKey)
		if err != nil {
			if strings.Contains(err.Error(), "lebih lama") {
				fmt.Println("⏹ Stop: artikel sudah lama, hentikan loop.")
				old++
				break
			}
			fmt.Println("⏩ Lewati:", err)
			skipped++
			continue
		}

		if err := insertNews(db, article); err != nil {
			fmt.Println("❌ Gagal insert:", err)
			skipped++
			continue
		}
		inserted++

		printProgress(i+1, total)
	}

	// 🔹 Ringkasan
	fmt.Printf("\n📊 Ringkasan %s:\n", cfg.Name)
	fmt.Printf("   ✅ Inserted: %d\n", inserted)
	fmt.Printf("   ⏩ Skipped : %d\n", skipped)
	fmt.Printf("   ⏳ Old/Stop: %d\n", old)
	fmt.Println("------------------------------------")
}

// ===== MAIN =====
func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("❌ GEMINI_API_KEY belum diset")
	} else {
		fmt.Println("✅ API key terbaca, panjang:", len(apiKey))
	}

	// Load sites dari YAML
	sites, err := loadSitesConfig("sites.yaml")
	if err != nil {
		log.Fatal("❌ Gagal load sites.yaml:", err)
	}

	// DB connect
	dsn := "postgres://postgres:Fazri18@localhost:5432/testing?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Scheduler
	c := cron.New()
	for _, site := range sites {
		site := site
		c.AddFunc("@hourly", func() { scrapeListAndInsert(db, site, apiKey) })
		scrapeListAndInsert(db, site, apiKey) // jalan sekali di awal
	}
	c.Start()
	select {}
}
