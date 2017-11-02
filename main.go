package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Person struct {
	FamilyName string `json:"family_name"`
	GivenName  string `json:"given_name"`
	ID         string `json:"__id__"`
	Identifier []struct {
		Scheme string `json:"scheme"`
		Value  string `json:"value"`
	} `json:"identifier"`
}

type AccessToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type ORCIDResponse struct {
	NumFound int `json:"num-found"`
}

var clientid = flag.String("client_id", "", "Client ID for ORCID API")
var clientsecret = flag.String("client_secret", "", "Client Secret for ORCID API")

func findFilesToProcess() []string {
	if len(flag.Args()) == 0 {
		log.Println("No file names provided, trying to find files ending with Person-export.json in current working directory.")
		workingDir, err := os.Getwd()
		if err != nil {
			log.Fatalln("Error getting working directory. ", err)
		}
		matches, err := filepath.Glob(filepath.Join(workingDir, "*Person-export.json"))
		if err != nil {
			log.Fatalln("Error finding matching files. ", err)
		}
		return matches
	} else {
		return flag.Args()
	}
}

func getORCIDSearchToken() string {

	v := url.Values{}
	v.Set("client_id", *clientid)
	v.Set("client_secret", *clientsecret)
	v.Set("grant_type", "client_credentials")
	v.Set("scope", "/read-public")

	resp, err := http.PostForm("https://orcid.org/oauth/token", v)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Unable to get access token from API.")
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		log.Fatalf("%s, %s", resp.StatusCode, bodyBytes)
	}

	var token AccessToken

	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		log.Fatalln(err)
	}

	return token.AccessToken
}

func processFile(filename, token string) {

	file, err := os.Open(filename)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	fileScanner := bufio.NewScanner(file)

	counter := 0

	for fileScanner.Scan() {

		counter++
		log.Println(counter)

		var person Person

		err := json.Unmarshal(fileScanner.Bytes(), &person)
		if err != nil {
			log.Fatalln(err)
		}

		for _, identifier := range person.Identifier {

			if identifier.Scheme == "scopus" {

				request, err := http.NewRequest("GET", "https://pub.orcid.org/v2.0/search/?q=eid-self:"+identifier.Value, nil)
				if err != nil {
					log.Fatal(err)
				}
				request.Header.Set("Accept", "application/vnd.orcid+json")
				request.Header.Set("Authorization", "Bearer "+token)

				resp, err := http.DefaultClient.Do(request)
				if err != nil {
					log.Fatalln(err)
				}
				defer resp.Body.Close()

				bodyBytes, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Fatalln(err)
				}

				if resp.StatusCode != http.StatusOK {
					log.Println("Bad HTTP status from API.")
					log.Fatalf("%s, %s", resp.StatusCode, bodyBytes)
				}

				var response ORCIDResponse

				err = json.Unmarshal(bodyBytes, &response)
				if err != nil {
					log.Fatalln(err)
				}

				time.Sleep(1 * time.Millisecond)

				if response.NumFound > 0 {
					fmt.Printf("This person:\n %v\nhas their Scopus ID in their ORCID profile.", person)
					fmt.Printf("%s\n", bodyBytes)
				}
			}
		}
	}
}

func main() {
	flag.Parse()

	filesToProcess := findFilesToProcess()
	if len(filesToProcess) == 0 {
		log.Fatalln("Could not find any files to process.")
	}

	if *clientid == "" {
		log.Fatalln("You need to provide a client_id")
	}

	if *clientsecret == "" {
		log.Fatalln("You need to provide a client_secret")
	}

	token := getORCIDSearchToken()

	for _, fileName := range filesToProcess {
		processFile(fileName, token)
	}

}
