package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
)

type Tag struct {
	Name       string `json:"name"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
}

func getTagsFromApi(owner string, repo string) []Tag {
	// https://docs.github.com/en/rest/reference/repos#list-repository-tags
	pathToTags := fmt.Sprintf("https://api.github.com/repos/%v/%v/tags", owner, repo)
	resp, err := http.Get(pathToTags)
	if err != nil {
		log.Fatalln(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	sb := string(body)
	var tagsForEachRelease []Tag
	json.Unmarshal([]byte(sb), &tagsForEachRelease)
	return tagsForEachRelease
}

func getChangeLogInfo(scanner *bufio.Scanner) ([]int, []string, int, []string) {
	// regexp for: ## 1.0.0 (2021-12-13)
	r, err := regexp.Compile(`## \d+\.\d+\.\d+`)
	if (err) != nil {
		log.Fatal(err)
	}
	lineCount := 0
	counterForChangeLogLength := 0
	var versionLineNumbers []int
	var changeLogText []string
	var versionTitlesForEachRelease []string
	for scanner.Scan() {
		counterForChangeLogLength++
		changeLogText = append(changeLogText, scanner.Text())
		if r.MatchString(scanner.Text()) {
			// ## 2.3.0 (2022-02-24) => 2.3.0 (2022-02-24)
			versionNumberAndDateWithoutHashes := scanner.Text()[3:]
			versionTitlesForEachRelease = append(versionTitlesForEachRelease, versionNumberAndDateWithoutHashes)
			versionLineNumbers = append(versionLineNumbers, lineCount)
		}
		lineCount++
	}
	if scanner.Err() != nil {
		fmt.Println(scanner.Err())
	}
	return versionLineNumbers, changeLogText, counterForChangeLogLength, versionTitlesForEachRelease
}

func makeReleaseSlice(versionLineNumbers []int, changeLogText []string, changeLogLength int) []string {
	// Store contributors and errors from end of changelog.
	contributorsAndErrors := ""
	numberOfLinesForLastRelease := 5
	for i := numberOfLinesForLastRelease + versionLineNumbers[len(versionLineNumbers)-1]; i < changeLogLength; i++ {
		contributorsAndErrors += changeLogText[i] + "\n"
	}
	var releaseNotesForEachVersion []string
	// exclude Last version (## 0.2.0) with - 1
	versionLineNumbersLengthMinusOne := len(versionLineNumbers) - 1
	for i, versionLineNumber := range versionLineNumbers[:] {
		releaseBodyLines := ""
		if i < (versionLineNumbersLengthMinusOne) {
			for j := versionLineNumbers[i] + 1; j < versionLineNumbers[i+1]; j++ {
				releaseBodyLines += changeLogText[j] + "\n"
			}
		}
		// Handle last version (## 0.2.0)
		if versionLineNumber == versionLineNumbers[versionLineNumbersLengthMinusOne] {
			for j := 1; j < numberOfLinesForLastRelease; j++ {
				releaseBodyLines += changeLogText[versionLineNumber+j] + "\n"
			}
		}
		releaseNotesForEachVersion = append(releaseNotesForEachVersion, releaseBodyLines+contributorsAndErrors)
	}
	return releaseNotesForEachVersion
}

func sendToGitHubAPI(tagForRelease Tag, releaseNote string, versionTitle string, owner string, repo string) {
	// https://docs.github.com/en/rest/reference/releases
	postBody, _ := json.Marshal(map[string]string{
		"tag_name": tagForRelease.Name,
		"name":     versionTitle,
		"body":     releaseNote,
	})
	responseBody := bytes.NewBuffer(postBody)
	url := fmt.Sprintf("https://api.github.com/repos/%v/%v/releases", owner, repo)
	req, err := http.NewRequest("POST", url, responseBody)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token insert_github_personal_token_here")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(req)
	fmt.Println("response Body:", string(body))
}

func main() {
	fmt.Println("Quick release notes running...")
	file, err := os.Open("./docs/CHANGELOG.md")
	if (err) != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	versionLineNumbers, changeLogText, changeLogLength, versionTitles := getChangeLogInfo(scanner)
	releaseNotesForEachVersion := makeReleaseSlice(versionLineNumbers, changeLogText, changeLogLength)
	// GET /repos/{owner}/{repo}/tags
	owner, repo := "quick-lint", "quick-lint-js"
	tagsForEachRelease := getTagsFromApi(owner, repo)
	// POST /repos/{owner}/{repo}/releases
	owner, repo = "LeeWannacott", "quick-release-notes"
	if len(releaseNotesForEachVersion) == len(tagsForEachRelease) && len(releaseNotesForEachVersion) == len(versionTitles) {
		for i, _ := range releaseNotesForEachVersion[:] {
			sendToGitHubAPI(tagsForEachRelease[i], releaseNotesForEachVersion[i], versionTitles[i], owner, repo)
		}
		fmt.Println("Quick release notes finished...")
	} else {
		fmt.Println("Error: Release Note versions in changelog.md and Tags from api are different lengths")
	}
}
