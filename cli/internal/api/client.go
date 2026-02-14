package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client wraps HTTP calls to the DocShare API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a Client from a base URL (e.g. http://localhost:8080) and bearer token.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/") + "/api",
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute, // generous for large uploads
		},
	}
}

// --- generic response types matching the backend envelope ---

// Response is the standard { success, data, error } envelope.
type Response[T any] struct {
	Success    bool        `json:"success"`
	Data       T           `json:"data"`
	Error      string      `json:"error,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

// APIError is returned when the server sends a non-2xx status.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api: %d — %s", e.Status, e.Message)
}

// --- low-level helpers ---

func (c *Client) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	u := c.BaseURL + path
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, nil
}

func (c *Client) doJSON(req *http.Request, out interface{}) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		// Try to extract the server's error message.
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"` // OAuth2 endpoints
		}
		if json.Unmarshal(data, &errResp) == nil && (errResp.Error != "" || errResp.ErrorDescription != "") {
			msg := errResp.Error
			if errResp.ErrorDescription != "" {
				msg = errResp.ErrorDescription
			}
			return &APIError{Status: resp.StatusCode, Message: msg}
		}
		return &APIError{Status: resp.StatusCode, Message: string(data)}
	}

	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// Get sends a GET request and decodes the JSON body into out.
func (c *Client) Get(path string, params url.Values, out interface{}) error {
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	req, err := c.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	return c.doJSON(req, out)
}

// Post sends a POST with a JSON body.
func (c *Client) Post(path string, body interface{}, out interface{}) error {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	req, err := c.newRequest(http.MethodPost, path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return c.doJSON(req, out)
}

// PostForm sends a POST with form-encoded body (for OAuth2 device flow endpoints).
func (c *Client) PostForm(path string, values url.Values, out interface{}) error {
	req, err := c.newRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	return c.doJSON(req, out)
}

// Put sends a PUT with a JSON body.
func (c *Client) Put(path string, body interface{}, out interface{}) error {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	req, err := c.newRequest(http.MethodPut, path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return c.doJSON(req, out)
}

// Delete sends a DELETE.
func (c *Client) Delete(path string, out interface{}) error {
	req, err := c.newRequest(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	return c.doJSON(req, out)
}

// Upload sends a multipart file upload.
func (c *Client) Upload(path, fieldName, filePath string, extraFields map[string]string, out interface{}) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		for k, v := range extraFields {
			_ = writer.WriteField(k, v)
		}

		part, err := writer.CreateFormFile(fieldName, fi.Name())
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, f); err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	req, err := c.newRequest(http.MethodPost, path, pr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	return c.doJSON(req, out)
}

// DownloadToFile streams a GET response body directly to a file on disk.
func (c *Client) DownloadToFile(rawURL, dest string) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Message: string(body)}
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (c *Client) UploadTransferFile(path string, file *os.File, size int64) error {
	req, err := c.newRequest(http.MethodPost, path, file)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", size))
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Message: string(body)}
	}

	return nil
}

func (c *Client) DownloadTransferFile(path string, dest *os.File) error {
	req, err := c.newRequest(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Message: string(body)}
	}

	_, err = io.Copy(dest, resp.Body)
	return err
}
