//go:build ignore

package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
)

func main() {
	address := os.Getenv("CANYON_TEST_ADDR")
	if address == "" {
		address = "http://localhost:8099"
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test 1: Wrong password
	fmt.Println("=== TEST 1: Wrong password ===")
	resp, _ := client.PostForm(address+"/login", url.Values{"username": {"admin"}, "password": {"wrong"}})
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(body), "Invalid") {
		fmt.Println("PASS: wrong password rejected")
	} else {
		fmt.Printf("FAIL: status=%d body=%s\n", resp.StatusCode, truncate(string(body), 200))
	}

	// Test 2: Correct password
	fmt.Println("\n=== TEST 2: Correct password ===")
	resp, _ = client.PostForm(address+"/login", url.Values{"username": {"admin"}, "password": {"admin"}})
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("status=%d location=%s\n", resp.StatusCode, resp.Header.Get("Location"))
	// Check cookies
	u, _ := url.Parse(address)
	cookies := jar.Cookies(u)
	for _, c := range cookies {
		fmt.Printf("cookie: %s=%s\n", c.Name, c.Value)
	}

	// Test 3: Access /units with cookie
	fmt.Println("\n=== TEST 3: Access /units with cookie ===")
	resp, _ = client.Get(address + "/units")
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("status=%d\n", resp.StatusCode)
	if strings.Contains(string(body), "All Units") {
		fmt.Println("PASS: logged in and accessed units")
	} else {
		fmt.Printf("FAIL: %s\n", truncate(string(body), 200))
	}

	// Test 4: Access /admin with admin cookie
	fmt.Println("\n=== TEST 4: Access /admin ===")
	resp, _ = client.Get(address + "/admin")
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("status=%d\n", resp.StatusCode)
	if strings.Contains(string(body), "Administration") {
		fmt.Println("PASS: admin page accessible")
	} else {
		fmt.Printf("FAIL: %s\n", truncate(string(body), 200))
	}

	// Test 5: Logout
	fmt.Println("\n=== TEST 5: Logout ===")
	resp, _ = client.Post(address+"/logout", "application/x-www-form-urlencoded", nil)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("status=%d location=%s\n", resp.StatusCode, resp.Header.Get("Location"))

	// Test 6: Access /units after logout
	fmt.Println("\n=== TEST 6: Access /units after logout ===")
	resp, _ = client.Get(address + "/units")
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("status=%d location=%s\n", resp.StatusCode, resp.Header.Get("Location"))
	if resp.StatusCode == 302 && resp.Header.Get("Location") == "/login" {
		fmt.Println("PASS: redirected to login after logout")
	} else {
		fmt.Println("FAIL: expected redirect to /login")
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
