package llm

import (
	"bytes"
	"encoding/json"
	"github.com/spf13/viper"
	"net/http"
)

type YiyanLLM struct {
	ApiKey      string
	SecretKey   string
	AccessToken string
	ApiUrl      string
	TokenUrl    string
}

func NewYiyanLLM() *YiyanLLM {
	apiKey := viper.GetString("yiyan.apiKey")
	apiSecret := viper.GetString("yiyan.apiSecret")
	tokenUrl := viper.GetString("yiyan.tokenUrl")
	apiUrl := viper.GetString("yiyan.apiUrl")
	return &YiyanLLM{
		ApiKey:    apiKey,
		SecretKey: apiSecret,
		TokenUrl:  tokenUrl,
		ApiUrl:    apiUrl,
	}
}

func (y *YiyanLLM) InitAccessToken() {
	type AccessTokenResp struct {
		RefreshToken  string `json:"refresh_token"`
		ExpiresIn     int    `json:"expires_in"`
		SessionKey    string `json:"session_key"`
		AccessToken   string `json:"access_token"`
		Scope         string `json:"scope"`
		SessionSecret string `json:"session_secret"`
		Err           string `json:"error"`
	}
	req, err := http.NewRequest("POST", y.TokenUrl+"?grant_type=client_credentials&client_id="+y.ApiKey+"&client_secret="+y.SecretKey, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var accessTokenResp AccessTokenResp
	err = json.NewDecoder(resp.Body).Decode(&accessTokenResp)
	if err != nil {
		panic(err)
	}
	if accessTokenResp.Err != "" {
		panic(accessTokenResp.Err)
	}
	y.AccessToken = accessTokenResp.AccessToken
}

func (y *YiyanLLM) Ask(questions []string) (string, error) {
	y.InitAccessToken()
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type AskReq struct {
		Messages []Message `json:"messages"`
	}
	type AskResponse struct {
		Id               string `json:"id"`
		Object           string `json:"object"`
		Created          int    `json:"created"`
		Result           string `json:"result"`
		IsTruncated      bool   `json:"is_truncated"`
		NeedClearHistory bool   `json:"need_clear_history"`
		Usage            struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	msgs := make([]Message, len(questions))
	for i := 0; i < len(questions); i++ {
		if i%2 == 0 {
			msgs[i] = Message{
				Role:    "user",
				Content: questions[i],
			}
		} else {
			msgs[i] = Message{
				Role:    "assistant",
				Content: questions[i],
			}
		}
	}
	askReq := AskReq{
		Messages: msgs,
	}
	requestBody, err := json.Marshal(askReq)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", y.ApiUrl+"?access_token="+y.AccessToken, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var askResp AskResponse
	err = json.NewDecoder(resp.Body).Decode(&askResp)
	if err != nil {
		return "", err
	}
	return askResp.Result, nil
}
