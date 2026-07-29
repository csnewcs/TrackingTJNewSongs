package tjapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type TJSongItem struct {
	Rownumber    int    `json:"rownumber"`
	Pro          int    `json:"pro"`
	IndexTitle   string `json:"indexTitle"`
	IndexSong    string `json:"indexSong"`
	Word         string `json:"word"`
	Com          string `json:"com"`
	ThumbnailImg string `json:"thumbnailImg"`
	IconGubun    string `json:"icongubun"`
	MvYn         string `json:"mv_yn"`
	PublishDate  string `json:"publishdate"`
}

type TJNewSongResponse struct {
	ResultCode string `json:"resultCode"`
	ResultMsg  string `json:"resultMsg"`
	ResultData struct {
		ItemsTotalCount int          `json:"itemsTotalCount"`
		Items           []TJSongItem `json:"items"`
	} `json:"resultData"`
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    "https://www.tjmedia.com/legacy/api/newSongOfMonth",
	}
}

func (c *Client) FetchNewSongs(searchYm string) ([]TJSongItem, error) {
	formData := url.Values{}
	formData.Set("searchYm", searchYm)

	resp, err := c.httpClient.PostForm(c.baseURL, formData)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp TJNewSongResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	if apiResp.ResultCode != "99" {
		return nil, fmt.Errorf("API error result code %s: %s", apiResp.ResultCode, apiResp.ResultMsg)
	}

	return apiResp.ResultData.Items, nil
}
