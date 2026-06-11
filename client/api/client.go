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

func (g *Game) UnmarshalJSON(data []byte) error {
	type alias Game
	aux := &struct {
		CreatedAt string `json:"created_at"`
		*alias
	}{alias: (*alias)(g)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	t, err := parseFlexTime(aux.CreatedAt)
	if err != nil {
		return err
	}
	g.CreatedAt = t
	return nil
}

type Save struct {
	ID          string    `json:"id"`
	GameID      string    `json:"game_id"`
	MachineName string    `json:"machine_name"`
	UploadedAt  time.Time `json:"uploaded_at"`
	FileSize    int64     `json:"file_size"`
}

func (s *Save) UnmarshalJSON(data []byte) error {
	type alias Save
	aux := &struct {
		UploadedAt string `json:"uploaded_at"`
		*alias
	}{alias: (*alias)(s)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	t, err := parseFlexTime(aux.UploadedAt)
	if err != nil {
		return err
	}
	s.UploadedAt = t
	return nil
}

// parseFlexTime parses timestamps with or without a timezone. The server is
// SQLite-backed and can return naive timestamps like
// "2026-06-11T00:31:32.779899"; these are treated as UTC, which is the zone
// the server actually records (datetime.now(timezone.utc)). Comparing the
// resulting instant against a local file's mod time is correct either way,
// since time comparisons operate on absolute instants.
func parseFlexTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp %q", s)
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
