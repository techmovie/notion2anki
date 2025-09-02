package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/dstotijn/go-notion"
)

var (
	ErrNotionAuthFailed = errors.New("notion: authentication failed, please check your token")
	ErrNotionDBNotFound = errors.New("notion: database not fount or permission denied")
)

type NotionClient struct {
	Config       NotionConfig
	Client       *notion.Client
	LastSyncTime time.Time
	PollInterval time.Duration
}

type NotionConfig struct {
	DatabaseID string `json:"database_id"`
	Token      string `json:"token"`
}

func get1PasswordSecret(reference string) (string, error) {
	if !strings.HasPrefix(reference, "op://") {
		return reference, nil
	}

	cmd := exec.Command("op", "read", reference)
	output, err := cmd.Output()
	log.Println("Reading secret from 1Password:", reference)
	if err != nil {
		return "", fmt.Errorf("failed to read 1Password secret: %v", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func NewNotion(tokenRef, databaseID string, interval time.Duration) *NotionClient {
	token, err := get1PasswordSecret(tokenRef)
	if err != nil {
		log.Println("Failed to get Notion token from 1Password")
		return nil
	}

	client := notion.NewClient(token)
	return &NotionClient{
		Config: NotionConfig{
			DatabaseID: databaseID,
			Token:      token,
		},
		Client:       client,
		LastSyncTime: time.Now().Add(-100 * time.Hour),
		PollInterval: time.Duration(interval),
	}
}

func (nt *NotionClient) QueryNotionDatabase(ctx context.Context, cursor string) (notion.DatabaseQueryResponse, error) {

	result, err := nt.Client.QueryDatabase(ctx, nt.Config.DatabaseID, &notion.DatabaseQuery{
		Filter: &notion.DatabaseQueryFilter{
			Timestamp: notion.TimestampLastEditedTime,
			DatabaseQueryPropertyFilter: notion.DatabaseQueryPropertyFilter{
				LastEditedTime: &notion.DatePropertyFilter{
					After: &nt.LastSyncTime,
				},
			},
		},
		Sorts: []notion.DatabaseQuerySort{
			{
				Timestamp: notion.TimestampLastEditedTime,
				Direction: notion.SortDirDesc,
			},
		},
		StartCursor: cursor,
	})
	if err != nil {
		var notionErr *notion.APIError
		if errors.As(err, &notionErr) {
			if notionErr.Status == http.StatusUnauthorized {
				return notion.DatabaseQueryResponse{}, ErrNotionAuthFailed
			}
			if notionErr.Status == http.StatusNotFound {
				return notion.DatabaseQueryResponse{}, ErrNotionDBNotFound
			}
		}
		return notion.DatabaseQueryResponse{}, fmt.Errorf("failed to query Notion database: %v", err)
	}

	return result, nil
}

func (nt *NotionClient) QueryAllPages(ctx context.Context) ([]notion.Page, notion.DatabasePageProperties, error) {
	var allPages []notion.Page
	var cursor string

	for {
		result, err := nt.QueryNotionDatabase(ctx, cursor)
		if err != nil {
			return nil, nil, err
		}

		allPages = append(allPages, result.Results...)

		if !result.HasMore {
			break
		}
		cursor = *result.NextCursor
	}

	var pageProperties notion.DatabasePageProperties
	if len(allPages) > 0 {
		page := allPages[0]
		if dbProps, ok := page.Properties.(notion.DatabasePageProperties); ok {
			pageProperties = dbProps
		}
	}

	return allPages, pageProperties, nil
}

func (nt *NotionClient) ExtractPropertiesFromPage(page notion.Page) map[string]string {
	properties := make(map[string]string)
	if page.Properties == nil {
		return properties
	}
	if dbProps, ok := page.Properties.(notion.DatabasePageProperties); ok {
		for name, prop := range dbProps {
			switch prop.Type {
			case notion.DBPropTypeTitle:
				if len(prop.Title) > 0 {
					text := prop.Title[0].PlainText
					if text != "" {
						properties[name] = text
					}
				} else {
					properties[name] = "-"
				}
			case notion.DBPropTypeURL:
				if prop.URL != nil && *prop.URL != "" {
					properties[name] = *prop.URL
				} else {
					properties[name] = "-"
				}
			case notion.DBPropTypeSelect:
				if prop.Select != nil && prop.Select.Name != "" {
					properties[name] = prop.Select.Name
				} else {
					properties[name] = "-"
				}
			case notion.DBPropTypeMultiSelect:
				var multiSelectValues []string
				for _, option := range prop.MultiSelect {
					if option.Name != "" {
						multiSelectValues = append(multiSelectValues, option.Name)
					}
				}
				if len(multiSelectValues) > 0 {
					properties[name] = strings.Join(multiSelectValues, ", ")
				} else {
					properties[name] = "-"
				}
			case notion.DBPropTypeRichText:
				var richTextValues []string
				for _, text := range prop.RichText {
					if text.PlainText != "" {
						richTextValues = append(richTextValues, text.PlainText)
					}
				}
				if len(richTextValues) > 0 {
					properties[name] = strings.Join(richTextValues, ", ")
				} else {
					properties[name] = "-"
				}
			}
		}
	}
	return properties
}

func (nt *NotionClient) UpdatePageOfDatabase(page notion.Page, props map[string]string, pageProperties notion.DatabasePageProperties) error {
	params := notion.UpdatePageParams{
		DatabasePageProperties: notion.DatabasePageProperties{},
	}
	for name, value := range props {
		prop := pageProperties[name]
		property := notion.DatabasePageProperty{}

		switch prop.Type {
		case notion.DBPropTypeTitle:
			property.Title = []notion.RichText{{Text: &notion.Text{Content: value}}}
		case notion.DBPropTypeRichText:
			property.RichText = []notion.RichText{{Text: &notion.Text{Content: value}}}
		case notion.DBPropTypeSelect:
			property.Select = &notion.SelectOptions{Name: value}
		case notion.DBPropTypeMultiSelect:
			options := strings.Split(value, ", ")
			for _, opt := range options {
				property.MultiSelect = append(property.MultiSelect, notion.SelectOptions{Name: opt})
			}
		case notion.DBPropTypeURL:
			property.URL = &value
		}

		params.DatabasePageProperties[name] = property

	}
	_, err := nt.Client.UpdatePage(context.Background(), page.ID, params)
	return err
}
