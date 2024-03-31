package lotmoe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

/*
URL: https://api.lot.moe

POST /auth/login = requires username and password form data.
POST /auth/refresh = requires refresh token and username fields.

GET /files = requires Authorization token, returns all saved files.
POST /files = requires Authorization token and a file in multipart form.
*/

var hostURL = "https://api.lot.moe"

type Client struct {
	httpClient *http.Client

	username     string
	accessToken  string
	refreshToken string
}

type File struct {
	Filename string `json:"filename,omitempty"`
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
}

type getFilesResponse struct {
	Files []*File `json:"files"`
}

func NewClient(username, password string) (*Client, error) {
	httpClient := &http.Client{}

	form := url.Values{}
	form.Add("username", username)
	form.Add("password", password)

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%v/auth/login", hostURL),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	return &Client{
		httpClient:   httpClient,
		username:     username,
		accessToken:  res.AccessToken,
		refreshToken: res.RefreshToken,
	}, nil
}

func (lm *Client) Upload(filename string, reader io.Reader) (string, error) {
	return lm.upload(filename, reader)
}

func (lm *Client) Files() ([]*File, error) {
	res, err := lm.files()
	if err != nil {
		return nil, err
	}

	return res.Files, nil
}

/*func (lm *Client) refresh() error {
	return nil
}*/

func (lm *Client) files() (*getFilesResponse, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%v/files", hostURL),
		nil,
	)
	if err != nil {
		return nil, err
	}

	lm.addAuthorization(req)
	resp, err := lm.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res getFilesResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (lm *Client) upload(filename string, reader io.Reader) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	fw, err := w.CreateFormFile("file[]", filename)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(fw, reader); err != nil {
		return "", err
	}

	w.Close()
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%v/files", hostURL),
		&buf,
	)
	if err != nil {
		return "", err
	}

	lm.contentType(req, w.FormDataContentType())
	lm.addAuthorization(req)
	resp, err := lm.httpClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (lm *Client) addAuthorization(req *http.Request) {
	req.Header.Add(
		"Authorization",
		fmt.Sprintf("Bearer %v", lm.accessToken),
	)
}

func (lm *Client) contentType(req *http.Request, t string) {
	req.Header.Set("Content-Type", t)
}

func (lm *Client) FindFile(files []*File, filename string) (*File, bool) {
	for _, file := range files {
		if file.Filename == filename {
			return file, true
		}
	}

	return nil, false
}
