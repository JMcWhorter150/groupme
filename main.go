package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

type Message struct {
	ID          string       `json:"id"`
	SourceGUID  string       `json:"source_guid"`
	CreatedAt   int64        `json:"created_at"`
	UserID      string       `json:"user_id"`
	GroupID     string       `json:"group_id"`
	Name        string       `json:"name"`
	AvatarURL   string       `json:"avatar_url"`
	Text        string       `json:"text"`
	System      bool         `json:"system"`
	FavoritedBy []string     `json:"favorited_by"`
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

type Response struct {
	Count    int       `json:"count"`
	Messages []Message `json:"messages"`
}

func loadEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
}

func getMessages(token, groupID string, beforeID *string) (*Response, error) {
	url := fmt.Sprintf("https://api.groupme.com/v3/groups/%s/messages?token=%s&limit=100", groupID, token)
	if beforeID != nil {
		url = fmt.Sprintf("%s&before_id=%s", url, *beforeID)
	}

    fmt.Println(url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}
    fmt.Println(response)

	return &response, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		source_guid TEXT,
		created_at INTEGER,
		user_id TEXT,
		group_id TEXT,
		name TEXT,
		avatar_url TEXT,
		text TEXT,
		system BOOLEAN,
		attachments TEXT,
		favorited_by TEXT
	);
	CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
		id,
		name,
		text,
		user_id,
		content='messages',
		content_rowid='rowid'
	);
    `)
	return err
}

func saveMessage(db *sql.DB, message Message) error {
	attachments, err := json.Marshal(message.Attachments)
	if err != nil {
		return err
	}

	favoritedBy, err := json.Marshal(message.FavoritedBy)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	INSERT OR REPLACE INTO messages (id, source_guid, created_at, user_id, group_id, name, avatar_url, text, system, attachments, favorited_by)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`, message.ID, message.SourceGUID, message.CreatedAt, message.UserID, message.GroupID, message.Name, message.AvatarURL, message.Text, message.System, string(attachments), string(favoritedBy))

	if err != nil {
		return err
	}

	_, err = db.Exec(`
	INSERT OR REPLACE INTO messages_fts (rowid, id, name, text, user_id)
	VALUES ((SELECT rowid FROM messages WHERE id = ?), ?, ?, ?, ?);
	`, message.ID, message.ID, message.Name, message.Text, message.UserID)

	return err
}

func main() {
	loadEnv()
	token := os.Getenv("GROUPME_TOKEN")
	groupID := os.Getenv("GROUPME_GROUP_ID")

	db, err := sql.Open("sqlite3", "./groupme.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	err = createTables(db)
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	var beforeID *string
	for {
		response, err := getMessages(token, groupID, beforeID)
        fmt.Println(response)
		if err != nil {
			log.Fatalf("Failed to get messages: %v", err)
		}

		if len(response.Messages) == 0 {
			break
		}

		for _, message := range response.Messages {
			err := saveMessage(db, message)
			if err != nil {
				log.Printf("Failed to save message: %v", err)
			}
			beforeID = &message.ID
		}

		time.Sleep(2 * time.Second)
	}
}
