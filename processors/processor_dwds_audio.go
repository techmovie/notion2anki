package processors

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-resty/resty/v2"
)

type DWDSAudioProcessor struct {
	client *resty.Client
}

const (
	baseURL = "https://www.dwds.de"
)

type AudioInfo struct {
	URL      string
	Format   string
	Found    bool
	ErrorMsg string
}

func (p *DWDSAudioProcessor) Name() string {
	return "dwds_audio"
}

func (p *DWDSAudioProcessor) Process(noteData *map[string]string, config ProcessorConfig) error {
	fmt.Println(config)
	sourceField := config.SourceField
	targetField := config.TargetField
	if sourceField == "" || targetField == "" {
		return errors.New("dwds_audio processor requires 'source_field' and 'target_field' in its config")
	}
	source, exist := (*noteData)[sourceField]
	if !exist || source == "" {
		return nil
	}
	log.Printf("[%s] Processing source: '%s'", p.Name(), source)
	audioInfo, err := p.GetAudioURL(source)
	if err != nil {
		log.Printf("Could not fetch audio for '%s': %v", source, err)
		return nil
	}
	if audioInfo.Found {
		(*noteData)[targetField] = audioInfo.URL
	}
	return nil
}

func NewDWDSAudioProcessor() *DWDSAudioProcessor {
	client := resty.New()

	client.SetTimeout(15 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second).
		SetHeaders(map[string]string{
			"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36",
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
			"Accept-Language":           "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7",
			"Accept-Encoding":           "gzip, deflate, br",
			"Connection":                "keep-alive",
			"Upgrade-Insecure-Requests": "1",
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Cache-Control":             "max-age=0",
		})

	return &DWDSAudioProcessor{
		client: client,
	}
}

func (p *DWDSAudioProcessor) GetAudioURL(word string) (AudioInfo, error) {

	dwdsURL := fmt.Sprintf("%s/wb/%s", baseURL, url.QueryEscape(strings.ToLower(word)))

	resp, err := p.client.R().
		SetHeader("Referer", fmt.Sprintf("%s/", baseURL)).
		Get(dwdsURL)

	if err != nil {
		return AudioInfo{ErrorMsg: "fail to fetch audio URL"}, err
	}

	if resp.StatusCode() != 200 {
		return AudioInfo{
			ErrorMsg: fmt.Sprintf("HTTP error: %d", resp.StatusCode()),
		}, fmt.Errorf("HTTP status code: %d", resp.StatusCode())
	}

	html := resp.String()

	return p.extractAudioURL(html, word)
}

func (p *DWDSAudioProcessor) extractAudioURL(html, word string) (AudioInfo, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return AudioInfo{ErrorMsg: "HTML parsing failed"}, err
	}

	audioInfo := p.findAudioElements(doc)
	if audioInfo.Found {
		return audioInfo, nil
	}

	return AudioInfo{
		Found:    false,
		ErrorMsg: "No audio link found",
	}, nil
}

func (p *DWDSAudioProcessor) findAudioElements(doc *goquery.Document) AudioInfo {
	var audioURL string

	doc.Find("audio").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists && src != "" {
			audioURL = src
			return
		}
		s.Find("source").Each(func(j int, source *goquery.Selection) {
			if src, exists := source.Attr("src"); exists && src != "" {
				audioURL = src
				return
			}
		})
	})

	if audioURL != "" {
		cleanURL := p.cleanAudioURL(audioURL)
		if cleanURL != "" {
			return AudioInfo{
				URL:    cleanURL,
				Format: p.detectAudioFormat(cleanURL),
				Found:  true,
			}
		}
	}

	return AudioInfo{Found: false}
}

func (p *DWDSAudioProcessor) cleanAudioURL(rawURL string) string {
	url := strings.ReplaceAll(rawURL, "&amp;", "&")
	url = strings.ReplaceAll(url, "&#x2F;", "/")
	url = strings.ReplaceAll(url, "&#47;", "/")

	url = strings.TrimSpace(url)
	url = strings.Trim(url, `"'`)

	if !strings.HasPrefix(url, "http") {
		if strings.HasPrefix(url, "/") {
			url = baseURL + url
		} else if strings.HasPrefix(url, "//") {
			url = "https:" + url
		} else if url != "" && !strings.Contains(url, "javascript:") {
			url = baseURL + "/" + url
		} else {
			return ""
		}
	}

	if !p.isAudioURL(url) {
		return ""
	}

	return url
}

func (p *DWDSAudioProcessor) isAudioURL(url string) bool {
	if url == "" {
		return false
	}

	lowerURL := strings.ToLower(url)

	audioExtensions := []string{".mp3", ".ogg", ".wav", ".m4a", ".aac"}
	for _, ext := range audioExtensions {
		if strings.Contains(lowerURL, ext) {
			return true
		}
	}

	audioPaths := []string{"audio", "sound", "pronunciation", "media", "mp3"}
	for _, path := range audioPaths {
		if strings.Contains(lowerURL, path) {
			return true
		}
	}

	return false
}

func (p *DWDSAudioProcessor) detectAudioFormat(url string) string {
	lowerURL := strings.ToLower(url)

	if strings.Contains(lowerURL, ".mp3") {
		return "mp3"
	} else if strings.Contains(lowerURL, ".ogg") {
		return "ogg"
	} else if strings.Contains(lowerURL, ".wav") {
		return "wav"
	} else if strings.Contains(lowerURL, ".m4a") {
		return "m4a"
	} else if strings.Contains(lowerURL, ".aac") {
		return "aac"
	}

	return "unknown"
}

func (p *DWDSAudioProcessor) ValidateAudioURL(audioURL string) bool {
	resp, err := p.client.R().
		SetHeader("Referer", fmt.Sprintf("%s/", baseURL)).
		Head(audioURL)

	if err != nil {
		return false
	}

	return resp.StatusCode() == 200
}
