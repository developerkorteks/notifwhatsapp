package scraper

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

// APIV2Response reflects the structure of the JSON payload.
type APIV2Response struct {
	Success bool   `json:"success"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Data    struct {
		Ready []APIV2Product `json:"ready"`
		Empty []APIV2Product `json:"empty"`
	} `json:"data"`
}

// APIV2Product defines individual product details.
type APIV2Product struct {
	KodeProduk string `json:"kode_produk"`
	NamaProduk string `json:"nama_produk"`
	Deskripsi  string `json:"deskripsi"`
	HargaFinal int    `json:"harga_final"`
	Stok       int    `json:"stok"`
}

// FetchAPIV2Stock connects to the external endpoint and returns the available products.
func FetchAPIV2Stock() []APIV2Product {
	req, err := http.NewRequest("GET", "https://api.ics-store.my.id/api/products?type=all", nil)
	if err != nil {
		log.Println("Error creating API V2 request:", err)
		return nil
	}

	// Apply necessary headers exactly as specified by the cURL request.
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "en-US,en;q=0.7")
	req.Header.Set("authorization", "Bearer 382e3f11c83608c218ebb029f3f30ced468e910ef1f980295548045417de2fce")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("origin", "https://www.ics-store.my.id")
	req.Header.Set("referer", "https://www.ics-store.my.id/")
	req.Header.Set("sec-ch-ua", `"Not:A-Brand";v="99", "Brave";v="145", "Chromium";v="145"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Linux"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-site")
	req.Header.Set("sec-gpc", "1")
	req.Header.Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error executing API V2 request:", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("API V2 returned non-200 status: %d\n", resp.StatusCode)
		return nil
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading API V2 response body:", err)
		return nil
	}

	var payload APIV2Response
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		log.Println("Error parsing API V2 JSON:", err)
		return nil
	}

	if !payload.Success {
		log.Println("API V2 responded with success: false")
		return nil
	}

	// Return strictly the ready logic. We don't care about the empty items for broadcasting diffs.
	return payload.Data.Ready
}
