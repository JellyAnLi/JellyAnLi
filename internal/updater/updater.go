package updater

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type VersionInfo struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	ReleaseURL     string `json:"release_url,omitempty"`
	ReleaseNotes   string `json:"release_notes,omitempty"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
}

var (
	cacheInfo  *VersionInfo
	cacheTime  time.Time
	cacheMutex sync.Mutex
)

// IsNewerVersion сравнивает две семантические версии
func IsNewerVersion(latest, current string) bool {
	lClean := strings.TrimPrefix(strings.TrimSpace(latest), "v")
	cClean := strings.TrimPrefix(strings.TrimSpace(current), "v")
	if lClean == "" || cClean == "" || lClean == cClean {
		return false
	}

	lowerC := strings.ToLower(cClean)
	if lowerC == "main" || lowerC == "master" || lowerC == "dev" || lowerC == "latest" {
		return false
	}

	lBase := strings.Split(lClean, "-")[0]
	cBase := strings.Split(cClean, "-")[0]

	lParts := strings.Split(lBase, ".")
	cParts := strings.Split(cBase, ".")

	maxLen := len(lParts)
	if len(cParts) > maxLen {
		maxLen = len(cParts)
	}

	for i := 0; i < maxLen; i++ {
		var lNum, cNum int
		if i < len(lParts) {
			lNum, _ = strconv.Atoi(lParts[i])
		}
		if i < len(cParts) {
			cNum, _ = strconv.Atoi(cParts[i])
		}
		if lNum > cNum {
			return true
		}
		if lNum < cNum {
			return false
		}
	}

	// Если основные числа равны (например 1.0.0 и 1.0.0-beta), стабильная без суффикса новее бета
	if !strings.Contains(lClean, "-") && strings.Contains(cClean, "-") {
		return true
	}

	return false
}

// CheckUpdate проверяет наличие обновлений на GitHub (кэширует ответ на 1 час)
func CheckUpdate(currentVersion string, force ...bool) *VersionInfo {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	forceCheck := len(force) > 0 && force[0]
	if !forceCheck && cacheInfo != nil && time.Since(cacheTime) < 1*time.Hour {
		return cacheInfo
	}

	info := &VersionInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  currentVersion,
		HasUpdate:      false,
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/JellyAnLi/JellyAnLi/releases/latest", nil)
	if err != nil {
		return info
	}
	req.Header.Set("User-Agent", "JellyAnLi/"+currentVersion)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return info
	}
	defer resp.Body.Close()

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil || rel.TagName == "" {
		return info
	}

	info.LatestVersion = rel.TagName
	info.ReleaseURL = rel.HTMLURL
	info.ReleaseNotes = rel.Body
	info.HasUpdate = IsNewerVersion(rel.TagName, currentVersion)

	cacheInfo = info
	cacheTime = time.Now()
	return info
}
