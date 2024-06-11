package helper

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

type Meta struct {
	Code int `json:"code"`
}

type OverallResponse struct {
	Meta     Meta     `json:"meta"`
	Response Response `json:"response"`
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

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response OverallResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Response, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		created_at INTEGER,
		user_id TEXT,
		name TEXT,
		text TEXT
	);

	CREATE TABLE IF NOT EXISTS attachments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT,
		type TEXT,
		url TEXT,
		lat TEXT,
		lng TEXT,
		name TEXT,
		placeholder TEXT,
		charmap TEXT,
		FOREIGN KEY(message_id) REFERENCES messages(id)
	);

	CREATE TABLE IF NOT EXISTS favorited_by (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT,
		user_id TEXT,
		FOREIGN KEY(message_id) REFERENCES messages(id)
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
	_, err := db.Exec(`
	INSERT OR REPLACE INTO messages (id, created_at, user_id, name, text)
	VALUES (?, ?, ?, ?, ?);
	`, message.ID, message.CreatedAt, message.UserID, message.Name, message.Text)

	if err != nil {
		return err
	}

	// Insert attachments
	for _, attachment := range message.Attachments {
		charmap, _ := json.Marshal(attachment.Charmap)
		_, err := db.Exec(`
		INSERT INTO attachments (message_id, type, url, lat, lng, name, placeholder, charmap)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);
		`, message.ID, attachment.Type, attachment.URL, attachment.Lat, attachment.Lng, attachment.Name, attachment.Placeholder, string(charmap))
		if err != nil {
			return err
		}
	}

	// Insert favorited_by
	for _, userID := range message.FavoritedBy {
		_, err := db.Exec(`
		INSERT INTO favorited_by (message_id, user_id)
		VALUES (?, ?);
		`, message.ID, userID)
		if err != nil {
			return err
		}
	}

	// Insert into FTS table
	_, err = db.Exec(`
	INSERT OR REPLACE INTO messages_fts (rowid, id, name, text, user_id)
	VALUES ((SELECT rowid FROM messages WHERE id = ?), ?, ?, ?, ?);
	`, message.ID, message.ID, message.Name, message.Text, message.UserID)

	return err
}

func messageExists(db *sql.DB, messageID string) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM messages WHERE id = ?)`, messageID).Scan(&exists)
	return exists, err
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
		if err != nil {
			log.Fatalf("Failed to get messages: %v", err)
		}

		if len(response.Messages) == 0 {
			break
		}

		for _, message := range response.Messages {
			exists, err := messageExists(db, message.ID)
			if err != nil {
				log.Printf("Failed to check if message exists: %v", err)
			}
			if exists {
				log.Printf("Message ID %s already exists in the database. Stopping fetch.", message.ID)
				return
			}

			err = saveMessage(db, message)
			if err != nil {
				log.Printf("Failed to save message: %v", err)
			}
			beforeID = &message.ID
		}

		time.Sleep(2 * time.Second)
	}
}

