package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

// File 数据结构
type File struct {
	ID   int    `json:"id"`
	Hash string `json:"hash"`
	Name string `json:"name"`
	File []byte `json:"-"`
}

func main() {
	// 连接 SQLite 数据库
	db, err := sql.Open("sqlite", "./files.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 初始化数据库
	initDB(db)

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	// 上传文件接口
	r.POST("/upload", func(c *gin.Context) {
		// 获取上传的文件
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file is uploaded"})
			return
		}

		// 打开文件读取数据
		fileContent, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
			return
		}
		defer fileContent.Close()

		// 计算文件哈希
		hash := calculateHash(fileContent)

		// 重置文件读取指针
		fileContent.Seek(0, io.SeekStart)

		// 检查文件是否已存在
		exists, err := fileExists(db, hash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check file existence"})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "File already exists"})
			return
		}

		// 读取文件内容到内存
		fileData, err := io.ReadAll(fileContent)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file content"})
			return
		}

		// 插入文件到数据库
		fileInfo := File{
			Hash: hash,
			Name: file.Filename,
			File: fileData,
		}
		if err := addFile(db, fileInfo); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file to database"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "File uploaded successfully",
			"filename": fileInfo.Name,
			"hash":     fileInfo.Hash,
		})
	})

	// 获取所有文件信息接口
	r.GET("/files", func(c *gin.Context) {
		files, err := getAllFiles(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get files"})
			return
		}
		c.JSON(http.StatusOK, files)
	})

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

// 初始化数据库表
func initDB(db *sql.DB) {
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hash TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		file BLOB NOT NULL
	);`
	_, err := db.Exec(createTableQuery)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}
}

// 计算文件哈希
func calculateHash(file io.Reader) string {
	hash := sha256.New()
	_, err := io.Copy(hash, file)
	if err != nil {
		log.Fatal("Failed to calculate hash:", err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// 检查文件是否存在
func fileExists(db *sql.DB, hash string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM files WHERE hash = ?)`
	err := db.QueryRow(query, hash).Scan(&exists)
	return exists, err
}

// 添加文件到数据库
func addFile(db *sql.DB, file File) error {
	insertQuery := `INSERT INTO files (hash, name, file) VALUES (?, ?, ?)`
	_, err := db.Exec(insertQuery, file.Hash, file.Name, file.File)
	return err
}

// 获取所有文件信息
func getAllFiles(db *sql.DB) ([]File, error) {
	rows, err := db.Query("SELECT id, hash, name FROM files")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		if err := rows.Scan(&file.ID, &file.Hash, &file.Name); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}
