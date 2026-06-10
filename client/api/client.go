package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type Game struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Save struct {
	ID          string    `json:"id"`
	GameID      string    `json:"game_id"`
	MachineName string    `json:"machine_name"`
	UploadedAt  time.Time `json:"uploaded_at"`
	FileSize    int64     `json:"file_size"`
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 2 * time.Minute},
	}
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-API-Key", c.apiKey)
	return c.http.Do(req)
}

func (c *Client) ListGames() ([]Game, error) {
	req, _ := http.NewRequest("GET", c.baseURL+"/games", nil)
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server error %d", resp.StatusCode)
	}
	var out []Game
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

func (c *Client) RegisterGame(id, name string) error {
	body, _ := json.Marshal(map[string]string{"id": id, "name": name})
	req, _ := http.NewRequest("POST", c.baseURL+"/games", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register failed: %s", b)
	}
	return nil
}

func (c *Client) ListSaves(gameID string) ([]Save, error) {
	req, _ := http.NewRequest("GET", c.baseURL+"/games/"+gameID+"/saves", nil)
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server error %d", resp.StatusCode)
	}
	var out []Save
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

func (c *Client) UploadSave(gameID, machineName string, data io.Reader) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("machine_name", machineName)
	fw, err := mw.CreateFormFile("file", gameID+".zip")
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, data); err != nil {
		return err
	}
	mw.Close()

	req, _ := http.NewRequest("POST", c.baseURL+"/games/"+gameID+"/saves", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: %s", b)
	}
	return nil
}

func (c *Client) DownloadLatest(gameID string) (io.ReadCloser, error) {
	req, _ := http.NewRequest("GET", c.baseURL+"/games/"+gameID+"/saves/latest", nil)
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		resp.Body.Close()
		return nil, fmt.Errorf("no saves on server for this game")
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("server error %d", resp.StatusCode)
	}
	return resp.Body, nil
}
