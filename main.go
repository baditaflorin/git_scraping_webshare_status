package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type Feed struct {
	Entries []Entry `xml:"entry"`
}

type Entry struct {
	ID      string `xml:"id"`
	Updated string `xml:"updated"`
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Link    struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

type Incident struct {
	ID      string `json:"id"`
	Updated string `json:"updated"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Link    string `json:"link"`
}

const (
	feedURL  = "https://status.webshare.io/feed.atom"
	dataFile = "data.json"
)

func main() {
	resp, err := http.Get(feedURL)
	handleError(err, "fetching the feed")
	defer safeClose(resp.Body)

	body, err := io.ReadAll(resp.Body)
	handleError(err, "reading response body")

	var feed Feed
	err = xml.Unmarshal(body, &feed)
	handleError(err, "parsing the feed")

	existingIncidents := loadIncidents()

	newIncidents := []Incident{}
	for _, entry := range feed.Entries {
		if _, exists := existingIncidents[entry.ID]; !exists {
			incident := Incident{
				ID:      entry.ID,
				Updated: entry.Updated,
				Title:   entry.Title,
				Summary: entry.Summary,
				Link:    entry.Link.Href,
			}
			existingIncidents[entry.ID] = incident
			newIncidents = append(newIncidents, incident)
		}
	}

	if len(newIncidents) > 0 {
		saveIncidents(existingIncidents)
		commitToGit(newIncidents)
	} else {
		fmt.Println("No new incidents to commit.")
	}
}

func loadIncidents() map[string]Incident {
	file, err := os.Open(dataFile)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]Incident)
	}
	handleError(err, "opening data file")
	defer safeClose(file)

	var incidents map[string]Incident
	err = json.NewDecoder(file).Decode(&incidents)
	handleError(err, "parsing JSON data")
	return incidents
}

func saveIncidents(incidents map[string]Incident) {
	file, err := os.Create(dataFile)
	handleError(err, "creating data file")
	defer safeClose(file)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(incidents)
	handleError(err, "saving JSON data")
}

func commitToGit(newIncidents []Incident) {
	runCommand("git", "config", "--global", "user.email", "github-actions[bot]@users.noreply.github.com")
	runCommand("git", "config", "--global", "user.name", "GitHub Actions")

	runCommand("git", "add", dataFile)

	message := fmt.Sprintf("Update incidents: %d new (%s)", len(newIncidents), time.Now().Format(time.RFC3339))
	runCommand("git", "commit", "-m", message)
	runCommand("git", "push")
}

func runCommand(command string, args ...string) {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	handleError(err, fmt.Sprintf("running command '%s %v'", command, args))
}

// handleError logs and exits if an error is not nil
func handleError(err error, context string) {
	if err != nil {
		fmt.Printf("Error %s: %v\n", context, err)
		os.Exit(1)
	}
}

// safeClose closes a resource and logs if there’s an error
func safeClose(c io.Closer) {
	if err := c.Close(); err != nil {
		fmt.Printf("Error closing resource: %v\n", err)
	}
}
