package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"

	"mmchat/internal/common"
	"mmchat/internal/component"
	"mmchat/internal/config"

	"github.com/redis/go-redis/v9"
)

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         struct {
		ID uint `json:"id"`
	} `json:"user"`
}

type createGroupResponse struct {
	ID uint `json:"id"`
}

func prepareLoadUsers(ctx context.Context, opts options) error {
	httpClient := &http.Client{Timeout: opts.RequestTimeout}
	redisClient, err := component.InitRedis(&config.GetConfig().RedisConfig)
	if err != nil {
		return fmt.Errorf("init Redis: %w", err)
	}
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping Redis: %w", err)
	}

	users := make([]loadUser, opts.PrepareGroups*opts.GroupMembers)
	for index := range users {
		phone, err := randomPhone()
		if err != nil {
			return err
		}
		login, err := registerAndLogin(
			ctx,
			httpClient,
			redisClient,
			opts.BaseURL,
			phone,
			opts.Password,
			fmt.Sprintf("压测用户%04d", index+1),
		)
		if err != nil {
			return fmt.Errorf("prepare user %d: %w", index+1, err)
		}
		users[index] = loadUser{
			UserID:       login.User.ID,
			AccessToken:  login.AccessToken,
			RefreshToken: login.RefreshToken,
		}
		fmt.Printf("prepared user %d/%d: user_id=%d\n", index+1, len(users), login.User.ID)
	}

	for groupIndex := range opts.PrepareGroups {
		start := groupIndex * opts.GroupMembers
		end := start + opts.GroupMembers
		memberIDs := make([]uint, opts.GroupMembers-1)
		for index := start + 1; index < end; index++ {
			memberIDs[index-start-1] = users[index].UserID
		}

		groupName := opts.GroupName
		if opts.PrepareGroups > 1 {
			groupName = fmt.Sprintf("%s-%02d", groupName, groupIndex+1)
		}
		group, err := postJSON[createGroupResponse](
			ctx,
			httpClient,
			opts.BaseURL+"/api/v1/conversations/group",
			users[start].AccessToken,
			struct {
				Name    string `json:"name"`
				UserIDs []uint `json:"user_ids"`
			}{Name: groupName, UserIDs: memberIDs},
		)
		if err != nil {
			return fmt.Errorf("create group %d: %w", groupIndex+1, err)
		}
		for index := start; index < end; index++ {
			users[index].ConversationIDs = []uint{group.ID}
		}
		fmt.Printf(
			"prepared group %d/%d: conversation_id=%d members=%d\n",
			groupIndex+1,
			opts.PrepareGroups,
			group.ID,
			opts.GroupMembers,
		)
	}
	if err := writeLoadUsers(opts.UsersFile, users); err != nil {
		return err
	}

	fmt.Printf(
		"prepared: users=%d conversations=%d output=%s\n",
		len(users),
		opts.PrepareGroups,
		opts.UsersFile,
	)
	return nil
}

func registerAndLogin(
	ctx context.Context,
	client *http.Client,
	redisClient *redis.Client,
	baseURL string,
	phone string,
	password string,
	nickname string,
) (loginResponse, error) {
	_, err := postJSON[struct{}](
		ctx,
		client,
		baseURL+"/api/v1/auth/register-code",
		"",
		struct {
			Phone string `json:"phone"`
		}{Phone: phone},
	)
	if err != nil {
		return loginResponse{}, fmt.Errorf("send register code: %w", err)
	}
	code, err := redisClient.Get(ctx, registerCodeKey(phone)).Result()
	if err != nil {
		return loginResponse{}, fmt.Errorf("read register code: %w", err)
	}
	_, err = postJSON[struct{}](
		ctx,
		client,
		baseURL+"/api/v1/auth/register",
		"",
		struct {
			Phone    string `json:"phone"`
			Password string `json:"password"`
			Nickname string `json:"nickname"`
			Code     string `json:"code"`
		}{
			Phone:    phone,
			Password: password,
			Nickname: nickname,
			Code:     code,
		},
	)
	if err != nil {
		return loginResponse{}, fmt.Errorf("register: %w", err)
	}
	login, err := postJSON[loginResponse](
		ctx,
		client,
		baseURL+"/api/v1/auth/login",
		"",
		struct {
			Phone    string `json:"phone"`
			Password string `json:"password"`
		}{Phone: phone, Password: password},
	)
	if err != nil {
		return loginResponse{}, fmt.Errorf("login: %w", err)
	}
	if login.User.ID == 0 || login.AccessToken == "" || login.RefreshToken == "" {
		return loginResponse{}, fmt.Errorf("login response is invalid")
	}
	return login, nil
}

func randomPhone() (string, error) {
	number, err := cryptorand.Int(cryptorand.Reader, big.NewInt(100_000_000))
	if err != nil {
		return "", fmt.Errorf("generate phone: %w", err)
	}
	return fmt.Sprintf("199%08d", number.Int64()), nil
}

func postJSON[T any](
	ctx context.Context,
	client *http.Client,
	requestURL string,
	accessToken string,
	body any,
) (T, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		var result T
		return result, err
	}
	return requestJSON[T](
		ctx,
		client,
		http.MethodPost,
		requestURL,
		accessToken,
		payload,
	)
}

func writeLoadUsers(path string, users []loadUser) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open users output: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(users); err != nil {
		return fmt.Errorf("write users output: %w", err)
	}
	return nil
}

func registerCodeKey(phone string) string {
	return common.RedisKeys.RegisterCodeKey(strings.TrimSpace(phone))
}
