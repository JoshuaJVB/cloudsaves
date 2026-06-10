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

// do builds and sends a request, returning a clear error if the configured
// server URL is unparseable rather than panicking on a nil request.
func (c *Client) do(method, url string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL %q (did you include http://?): %w", c.baseURL, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.http.Do(req)
}

func (c *Client) ListGames() ([]Game, error) {
	resp, err := c.do("GET", c.baseURL+"/games", nil, "")
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
	resp, err := c.do("POST", c.baseURL+"/games", bytes.NewReader(body), "application/json")
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
	resp, err := c.do("GET", c.baseURL+"/games/"+gameID+"/saves", nil, "")
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

	resp, err := c.do("POST", c.baseURL+"/games/"+gameID+"/saves", &buf, mw.FormDataContentType())
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
	resp, err := c.do("GET", c.baseURL+"/games/"+gameID+"/saves/latest", nil, "")
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
