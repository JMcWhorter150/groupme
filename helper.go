package main

import (
    "database/sql"
    "encoding/json"
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

