package main

import (
    "database/sql"
    "log"
    "net/http"
    "encoding/json"

    "github.com/gin-gonic/gin"
    _ "github.com/mattn/go-sqlite3"
)

type Message struct {
    ID          string       `json:"id"`
    CreatedAt   int64        `json:"created_at"`
    UserID      string       `json:"user_id"`
    Name        string       `json:"name"`
    Text        string       `json:"text"`
    LikeCount   int          `json:"like_count"`
    Attachments []Attachment `json:"attachments"`
}

type Attachment struct {
    Type        string  `json:"type"`
    URL         string  `json:"url,omitempty"`
    Lat         string  `json:"lat,omitempty"`
    Lng         string  `json:"lng,omitempty"`
    Name        string  `json:"name,omitempty"`
    Placeholder string  `json:"placeholder,omitempty"`
    Charmap     [][]int `json:"charmap,omitempty"`
}

var db *sql.DB

func main() {
    var err error
    db, err = sql.Open("sqlite3", "./groupme.db")
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer db.Close()

    r := gin.Default()
    r.Static("/static", "./static")
    r.LoadHTMLGlob("templates/*")

    r.GET("/", func(c *gin.Context) {
        c.HTML(http.StatusOK, "index.html", nil)
    })

    r.GET("/search", searchHandler)
    r.GET("/messages/:id", messageHandler)
    r.GET("/messages/:id/before", beforeMessagesHandler)
    r.GET("/messages/:id/after", afterMessagesHandler)

    r.Run(":8080")
}

func searchHandler(c *gin.Context) {
    query := c.Query("q")
    var results []Message
    rows, err := db.Query(`
        SELECT m.id, m.created_at, m.user_id, m.name, m.text, (SELECT COUNT(*) FROM favorited_by WHERE message_id = m.id) AS like_count
        FROM messages_fts fts
        JOIN messages m ON fts.rowid = m.rowid
        WHERE messages_fts MATCH ?
        ORDER BY rank, m.created_at DESC
        LIMIT 10`, query)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    for rows.Next() {
        var message Message
        if err := rows.Scan(&message.ID, &message.CreatedAt, &message.UserID, &message.Name, &message.Text, &message.LikeCount); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        message.Attachments, _ = getAttachments(db, message.ID)
        results = append(results, message)
    }

    c.JSON(http.StatusOK, results)
}

func messageHandler(c *gin.Context) {
    id := c.Param("id")
    var message Message
    err := db.QueryRow(`
        SELECT id, created_at, user_id, name, text, (SELECT COUNT(*) FROM favorited_by WHERE message_id = id) AS like_count
        FROM messages
        WHERE id = ?`, id).Scan(&message.ID, &message.CreatedAt, &message.UserID, &message.Name, &message.Text, &message.LikeCount)
    if err != nil {
        if err == sql.ErrNoRows {
            c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    message.Attachments, _ = getAttachments(db, id)

    beforeMessages, _ := getMessagesBefore(db, id, 50)
    afterMessages, _ := getMessagesAfter(db, id, 50)

    c.JSON(http.StatusOK, gin.H{
        "message":         message,
        "before_messages": beforeMessages,
        "after_messages":  afterMessages,
    })
}

func beforeMessagesHandler(c *gin.Context) {
    id := c.Param("id")

    beforeMessages, err := getMessagesBefore(db, id, 100)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, beforeMessages)
}

func afterMessagesHandler(c *gin.Context) {
    id := c.Param("id")

    afterMessages, err := getMessagesAfter(db, id, 100)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, afterMessages)
}

func getAttachments(db *sql.DB, messageID string) ([]Attachment, error) {
    rows, err := db.Query(`
        SELECT type, url, lat, lng, name, placeholder, charmap
        FROM attachments
        WHERE message_id = ?`, messageID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var attachments []Attachment
    for rows.Next() {
        var attachment Attachment
        var charmap string
        if err := rows.Scan(&attachment.Type, &attachment.URL, &attachment.Lat, &attachment.Lng, &attachment.Name, &attachment.Placeholder, &charmap); err != nil {
            return nil, err
        }
        json.Unmarshal([]byte(charmap), &attachment.Charmap)
        attachments = append(attachments, attachment)
    }

    return attachments, nil
}

func getMessagesBefore(db *sql.DB, messageID string, limit int) ([]Message, error) {
    var id string
    err := db.QueryRow(`SELECT id FROM messages WHERE id < ? ORDER BY created_at DESC LIMIT 1`, messageID).Scan(&id)
    if err != nil {
        return nil, err
    }

    rows, err := db.Query(`
        SELECT id, created_at, user_id, name, text, (SELECT COUNT(*) FROM favorited_by WHERE message_id = id) AS like_count
        FROM messages
        WHERE id < ?
        ORDER BY created_at DESC
        LIMIT ?`, id, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []Message
    for rows.Next() {
        var message Message
        if err := rows.Scan(&message.ID, &message.CreatedAt, &message.UserID, &message.Name, &message.Text, &message.LikeCount); err != nil {
            return nil, err
        }
        message.Attachments, _ = getAttachments(db, message.ID)
        messages = append(messages, message)
    }

    return messages, nil
}

func getMessagesAfter(db *sql.DB, messageID string, limit int) ([]Message, error) {
    var id string
    err := db.QueryRow(`SELECT id FROM messages WHERE id > ? ORDER BY created_at ASC LIMIT 1`, messageID).Scan(&id)
    if err != nil {
        return nil, err
    }

    rows, err := db.Query(`
        SELECT id, created_at, user_id, name, text, (SELECT COUNT(*) FROM favorited_by WHERE message_id = id) AS like_count
        FROM messages
        WHERE id > ?
        ORDER BY created_at ASC
        LIMIT ?`, id, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []Message
    for rows.Next() {
        var message Message
        if err := rows.Scan(&message.ID, &message.CreatedAt, &message.UserID, &message.Name, &message.Text, &message.LikeCount); err != nil {
            return nil, err
        }
        message.Attachments, _ = getAttachments(db, message.ID)
        messages = append(messages, message)
    }

    return messages, nil
}

