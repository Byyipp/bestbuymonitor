package main

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

/*
	COOKIE JAR
*/

var jar, err = cookiejar.New(nil)
var client = http.Client{
	Jar: jar,
}

/*
	SKU DETAILS
*/

type details struct {
	Response []innerdetails `json:"buttonStateResponseInfos"`
}

type innerdetails struct {
	Button      string `json: "buttonState"`
	Displaytext string `json: "displayText"`
	Sku         string `json: "skuId"`
}

/*
	PROXY ROTATION
*/

var proxylist = createsplice("proxies.txt")
var proxyiteration = 0

func rotateproxy(list []string) (string, string, string) {
	if proxyiteration == len(list)-1 {
		proxyiteration = 0
	}
	proxy := list[proxyiteration]
	splice := strings.Split(proxy, ":")
	temp := strings.Fields(splice[3])
	proxyiteration++
	return splice[0] + ":" + splice[1], splice[2], temp[0]
}

func createsplice(file string) []string {
	txt, err := os.Open(file)
	txt.Sync()
	if err != nil {
		log.Fatal(err)
	}
	byte, err := ioutil.ReadAll(txt)
	if err != nil {
		log.Fatal(err)
	}

	return strings.Split(string(byte), "\n")
}

/*
	MONITORING
*/

func monitor(sku string) {
	var prodDetails details
	instock := 0

	link := "https://www.bestbuy.com/button-state/api/v5/button-state?skus=" + sku + "&conditions=&storeId=&destinationZipCode=&context=pdp&consolidated=false&source=buttonView&xboxAllAccess=false"

	if sku == "" {
		println("Invalid SKU")
		return
	}
	proxy, auth, password := rotateproxy(proxylist)

	client.Transport = &http.Transport{
		Proxy: http.ProxyURL(&url.URL{
			Scheme: "http",
			User:   url.UserPassword(auth, password),
			Host:   proxy,
		}),
	}

	req, err := http.NewRequest("GET", link, strings.NewReader(""))

	req.Header.Set("accept", "application/json")
	req.Header.Set("accept-encoding", "gzip, deflate, br")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("dnt", "1")
	req.Header.Set("sec-ch-ua", `" Not A;Brand";v="99", "Chromium";v="90", "Google Chrome";v="90"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.212 Safari/537.36")
	req.Header.Set("x-client-id", "FRV")
	req.Header.Set("x-request-id", "BROWSE")

	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	//println(string(body))

	err = json.Unmarshal([]byte(string(body)), &prodDetails)

	if strings.Contains(strings.ToLower(prodDetails.Response[0].Displaytext), "sold out") {
		println("OOS")
	} else if strings.Contains(strings.ToLower(prodDetails.Response[0].Button), "sold") {
		println("OOS")
	} else {
		println("INSTOCK")
		instock = 1
	}

	check(sku, instock)
}

/*
	DATABASE
*/

var database, _ = sql.Open("sqlite3", "./skulist.db")

func check(sku string, instock int) {
	var dbID int
	var dbstock int

	// CHECK FOR SKU IN DATABASE
	sqlstatement := `SELECT sku, instock FROM skus WHERE sku=$1`
	row := database.QueryRow(sqlstatement, sku)
	err = row.Scan(&dbID, &dbstock)
	if err != nil {
		if err == sql.ErrNoRows { // NO MATCH
			println("SKU Does Not Exist In DB (DB Updated")
			statement, _ := database.Prepare("INSERT INTO skus (sku, instock) VALUES (?, ?)")
			statement.Exec(sku, instock)
			return
		} else { // UNKNOWN ERROR
			println(err)
			return
		}
	}

	//println("testing " + strconv.Itoa(dbID) + ": " + strconv.Itoa(dbstock))

	// UPDATE DATABASE
	if dbstock != instock {
		_, err := database.Exec("UPDATE skus SET instock = ? WHERE sku = ?", instock, sku)
		if err != nil {
			println(err)
			return
		} else {
			println("sku updated")
			return
		}
	}

	// DB IS SAME
	//println("not updated")
}

func main() {
	statement, _ := database.Prepare("CREATE TABLE IF NOT EXISTS skus (sku INT, instock BOOL)")
	statement.Exec()

	for {
		go monitor("6429440")
		go monitor("6439385")
		go monitor("6412595")

		time.Sleep(time.Second * 7)
	}

}
