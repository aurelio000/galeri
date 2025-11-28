// main.go (diperbaiki agar gambar muncul & path benar)
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite", "photos.db")
	if err != nil {
		log.Fatalf("Gagal membuka database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS photos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		judul TEXT,
		deskripsi TEXT,
		file_path TEXT,
		upload_date DATETIME
	)`)
	if err != nil {
		log.Fatalf("Gagal membuat tabel: %v", err)
	}

	os.MkdirAll("uploads", os.ModePerm)

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/uploads", "./uploads")

	r.GET("/", listPhotos)
	r.POST("/upload", uploadPhoto)
	r.POST("/photos/:id", apiUpdatePhoto)
	r.POST("/photos/delete/:id", apiDeletePhoto)

	r.Run(":8080")
}

func uploadPhoto(c *gin.Context) {
	judul := c.PostForm("judul")
	deskripsi := c.PostForm("deskripsi")
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "File tidak ditemukan")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		c.String(http.StatusBadRequest, "Format file tidak didukung")
		return
	}
	if file.Size > 2<<20 {
		c.String(http.StatusBadRequest, "Ukuran file maksimal 2MB")
		return
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	diskPath := filepath.Join("uploads", filename)
	webPath := "/uploads/" + filename
	c.SaveUploadedFile(file, diskPath)

	fmt.Println(">> File disimpan di:", diskPath)
	fmt.Println(">> URL untuk database:", webPath)

	_, err = db.Exec(`INSERT INTO photos (judul, deskripsi, file_path, upload_date) VALUES (?, ?, ?, ?)`,
		judul, deskripsi, webPath, time.Now())
	if err != nil {
		c.String(http.StatusInternalServerError, "Gagal menyimpan data")
		return
	}
	c.Redirect(http.StatusFound, "/")
}

func listPhotos(c *gin.Context) {
	rows, err := db.Query(`SELECT id, judul, deskripsi, file_path, upload_date FROM photos`)
	if err != nil {
		c.String(http.StatusInternalServerError, "Gagal membaca data")
		return
	}
	defer rows.Close()

	type Photo struct {
		ID         int
		Judul      string
		Deskripsi  string
		FilePath   string
		UploadDate string
	}
	var photos []Photo
	for rows.Next() {
		var p Photo
		var uploadTime time.Time
		err := rows.Scan(&p.ID, &p.Judul, &p.Deskripsi, &p.FilePath, &uploadTime)
		if err == nil {
			p.UploadDate = uploadTime.Format("02 Jan 2006 15:04")
			photos = append(photos, p)
		}
	}
	c.HTML(http.StatusOK, "index.html", gin.H{"photos": photos})
}

func apiUpdatePhoto(c *gin.Context) {
	id := c.Param("id")
	judul := c.PostForm("judul")
	deskripsi := c.PostForm("deskripsi")

	row := db.QueryRow(`SELECT file_path FROM photos WHERE id = ?`, id)
	var oldPath string
	if err := row.Scan(&oldPath); err != nil {
		c.String(http.StatusNotFound, "Foto tidak ditemukan")
		return
	}

	newPath := oldPath
	file, err := c.FormFile("file")
	if err == nil {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			c.String(http.StatusBadRequest, "Format file tidak didukung")
			return
		}
		if file.Size > 2<<20 {
			c.String(http.StatusBadRequest, "Ukuran file maksimal 2MB")
			return
		}

		_ = os.Remove(oldPath[1:])
		filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		diskPath := filepath.Join("uploads", filename)
		webPath := "/uploads/" + filename
		c.SaveUploadedFile(file, diskPath)
		newPath = webPath
	}

	_, err = db.Exec(`UPDATE photos SET judul = ?, deskripsi = ?, file_path = ? WHERE id = ?`, judul, deskripsi, newPath, id)
	if err != nil {
		c.String(http.StatusInternalServerError, "Gagal update")
		return
	}
	c.Redirect(http.StatusFound, "/")
}

func apiDeletePhoto(c *gin.Context) {
	id := c.Param("id")
	row := db.QueryRow(`SELECT file_path FROM photos WHERE id = ?`, id)
	var filePath string
	if err := row.Scan(&filePath); err != nil {
		c.String(http.StatusNotFound, "Foto tidak ditemukan")
		return
	}

	_ = os.Remove(filePath[1:])
	_, err := db.Exec(`DELETE FROM photos WHERE id = ?`, id)
	if err != nil {
		c.String(http.StatusInternalServerError, "Gagal menghapus")
		return
	}
	c.Redirect(http.StatusFound, "/")
}
