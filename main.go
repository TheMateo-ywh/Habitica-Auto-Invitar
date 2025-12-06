package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Response struct {
	Success bool   `json:"success"`
	User    []User `json:"data"`
}

type User struct {
	Id string `json:"_id"`
	Auth Auth `json:"auth"`
	Preferences Preferences `json:"preferences"`
	Stats Stats `json:"stats"`
}

type Auth struct {
	Timestamps Timestamps `json:"timestamps"`
}

type Timestamps struct {
	Created time.Time `json:"created"`
	LoggedIn time.Time `json:"loggedin"`
	Updated time.Time `json:"updated"`
}

type Preferences struct {
	Language string `json:"language"`
}

type Stats struct {
	Level int `json:"lvl"`
}

type InviteRequest struct {
	Uuids []string `json:"uuids"`
}

var apiUser string
var apiKey string
var minLvl int
var fetchInterval uint64
var language string
var onlyActive bool
var maxCycles int
var singleRun bool

func main() {
	flag.StringVar(&apiUser, "api-user", "", "Habitica API user")
	flag.StringVar(&apiKey, "api-key", "", "Habitica API key")
	flag.IntVar(&minLvl, "min-lvl", 0, "Min level of users to invite to party. Default is 0.")
	flag.Uint64Var(&fetchInterval, "fetch-interval", 120, "Interval for fetching users in seconds. Default is 120.")
	flag.StringVar(&language, "language", "", "Language of users to invite to party. Default is all languages.")
	flag.BoolVar(&onlyActive, "only-active", false, "Only invite active users to party. Default is false.")
	flag.IntVar(&maxCycles, "max-cycles", 1, "Number of cycles to run. Default is 1 (single run).")
	flag.BoolVar(&singleRun, "single-run", false, "Run once and exit (overrides max-cycles). Default is false.")
	flag.Parse()

	if apiUser == "" || apiKey == "" {
		log.Fatal("Please provide Habitica API user and key. (Use -api-user and -api-key flags)")
	}

	fmt.Println("Welcome to PartyUp! The script will now start fetching users and inviting them to party.")
	
	// Si single-run es true, ejecutar solo una vez
	if singleRun {
		fmt.Println("Single-run mode: Executing one cycle...")
		fetchUsersAndInvite()
		fmt.Println("Single-run completed. Exiting.")
		return
	}
	
	// Ejecutar en ciclos según maxCycles
	if maxCycles <= 0 {
		maxCycles = 1
	}
	
	fmt.Printf("Running %d cycles with %d seconds interval...\n", maxCycles, fetchInterval)
	
	for i := 1; i <= maxCycles; i++ {
		fmt.Printf("\n=== Cycle %d/%d ===\n", i, maxCycles)
		fetchUsersAndInvite()
		
		// No esperar después del último ciclo
		if i < maxCycles {
			fmt.Printf("Waiting %d seconds for next cycle...\n", fetchInterval)
			time.Sleep(time.Duration(fetchInterval) * time.Second)
		}
	}
	
	fmt.Printf("\nCompleted all %d cycles. Exiting.\n", maxCycles)
}

func fetchUsersAndInvite() {
	fmt.Println("Fetching users and inviting them to party...")
	url := "https://habitica.com/api/v3/looking-for-party"

	habiticaClient := http.Client{
		Timeout: time.Second * 120,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-client", fmt.Sprintf("%s-PartyUp", apiUser))
	req.Header.Set("x-api-user", apiUser)
	req.Header.Set("x-api-key", apiKey)

	res, getErr := habiticaClient.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	var response Response
	jsonErr := json.Unmarshal(body, &response)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	if !response.Success {
		log.Fatal("Request failed, please check your API user and key.")
	}

	usersUuid := make([]string, 0)
	for _, user := range response.User {
		if isValidUser(user) {
			usersUuid = append(usersUuid, user.Id)
		}
	}

	if len(usersUuid) != 0 {
		fmt.Printf("Found %d valid users to invite.\n", len(usersUuid))
		inviteUsers(habiticaClient, usersUuid)
	} else {
		fmt.Println("No users to invite at this time.")
	}
}

func inviteUsers(client http.Client, uuids []string) {
	inviteUrl := "https://habitica.com/api/v3/groups/party/invite"

	inviteRequest := InviteRequest{
		Uuids: uuids,
	}

	inviteBody, jsonErr := json.Marshal(inviteRequest)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	req, err := http.NewRequest(http.MethodPost, inviteUrl, bytes.NewBuffer(inviteBody))
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-client", fmt.Sprintf("%s-PartyUp", apiUser))
	req.Header.Set("x-api-user", apiUser)
	req.Header.Set("x-api-key", apiKey)

	res, postErr := client.Do(req)
	if postErr != nil {
		log.Fatal(postErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	_, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	fmt.Printf("Successfully invited %d users!\n", len(uuids))
}

func isValidUser(user User) bool {
	if user.Id == "" {
		return false
	}

	if user.Stats.Level < minLvl {
		return false
	}

	if language != "" && user.Preferences.Language != language {
		return false
	}

	if onlyActive {
		oneMonthAgo := time.Now().AddDate(0, -1, 0)
		recently := time.Now().AddDate(0, 0, -4)

		return user.Auth.Timestamps.Created.Before(oneMonthAgo) &&
			user.Auth.Timestamps.LoggedIn.After(recently)
	}

	return true
}