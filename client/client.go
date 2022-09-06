package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func New(baseUrl string, token string, tokenSecret string) (c *ClientWithResponses, err error) {
	accessToken, err := getPresetAcessToken(token, tokenSecret)

	if err != nil {
		return nil, err
	}

	bearerTokenProvider, err := securityprovider.NewSecurityProviderBearerToken(accessToken)
	if err != nil {
		return nil, err
	}

	client, err := NewClientWithResponses(baseUrl, WithRequestEditorFn(bearerTokenProvider.Intercept), WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Referer", req.URL.String())

		var body string

		if req.Body != nil {
			bodyReader, err := req.GetBody()

			if err != nil {
				return err
			}

			bodyBytes, err := ioutil.ReadAll(bodyReader)

			if err != nil {
				return err
			}

			body = string(bodyBytes)
		}

		if err != nil {
			return err
		}

		sanitizedHeaders := req.Header.Clone()
		sanitizedHeaders.Del("Authorization")

		tflog.Debug(ctx, ">> request", map[string]interface{}{
			"method":  req.Method,
			"url":     req.URL.String(),
			"headers": sanitizedHeaders,
			"body":    body,
		})

		return nil
	}))

	if err != nil {
		return nil, err
	}

	return client, nil
}

type PresetAuthTokenResponse struct {
	Payload struct {
		AccessToken string `json:"access_token"`
	} `json:"payload"`
}

func getPresetAcessToken(token string, tokenSecret string) (string, error) {
	requestBody, err := json.Marshal(map[string]string{
		"name":   token,
		"secret": tokenSecret,
	})

	if err != nil {
		return "", err
	}

	resp, err := http.Post("https://manage.app.preset.io/api/v1/auth/", "application/json", bytes.NewBuffer(requestBody))

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Failed to fetch Preset token: %s", body)
	}

	var data PresetAuthTokenResponse
	err = json.Unmarshal(body, &data)

	if err != nil {
		return "", nil
	}

	return data.Payload.AccessToken, nil
}
