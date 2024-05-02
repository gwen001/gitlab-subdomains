package main

import (
	"os"
	"fmt"
	"math/rand"
	"time"
	"flag"
	"sync"
	"regexp"
	"strings"
	"net/url"
	"net/http"
	"crypto/tls"
	"io/ioutil"
	"encoding/json"
	"github.com/logrusorgru/aurora"
	tld "github.com/jpillora/go-tld"
)

type Token struct {
	datoken string
	disabled_ts int64
}

type Search struct {
	signature string
	token string
	domain string
	scope string
	keyword string
	sort string
	order_by string
	TotalCount int
}

type Config struct {
	stop_notoken bool
	quick_mode bool
	domain string
	output string
	fpOutput *os.File
	tokens []Token
	extend bool
	debug bool
	search string
	delay time.Duration
	DomainRegexp *regexp.Regexp
}

var au = aurora.NewAurora(true)
var config = Config{}
var t_subdomains []string
var t_search []Search
var t_scopes = []string{"projects","issues","merge_requests","milestones","snippet_titles","users","blobs","commits","notes","wiki_blobs"}
// var t_scopes = []string{"blobs"}
// var t_search_fields = []string{"description"}


func parseToken( token string ) {

	var t_tokens []string

	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
		if token == "" {
			t_tokens = readFromFile(".tokens")
		} else {
			t_tokens = strings.Split(token, ",")
		}
	} else {
		t_tokens = append(t_tokens, token)
	}

	for _,t := range t_tokens {
		if len(t) > 0 {
			config.tokens = append( config.tokens, Token{datoken:t,disabled_ts:0} )
		}
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(config.tokens), func(i, j int) { config.tokens[i], config.tokens[j] = config.tokens[j], config.tokens[i] })
}


func readFromFile( filename string ) []string {

	var t_lines []string

	b, err := ioutil.ReadFile( filename )
    if err == nil {
		for _,l := range strings.Split(string(b), "\n") {
			l = strings.TrimSpace( l )
			if len(l) > 0 && !inArray(l,t_lines) {
				t_lines = append(t_lines, l)
			}
		}
	}

	return t_lines
}


func getNextToken( token_index int, n_token int ) int {

	token_index = (token_index+1) % n_token

	for k:=token_index ; k<n_token ; k++ {
		if config.tokens[k].disabled_ts == 0 || config.tokens[k].disabled_ts < time.Now().Unix() {
			config.tokens[k].disabled_ts = 0
			return k
		}
	}

	return -1
}


func doSearch(current_search Search) {

	PrintInfos( "debug", fmt.Sprintf("domain:%s, scope:%s, keyword:%s, order_by:%s, sort:%s", current_search.domain, current_search.scope, current_search.keyword, current_search.order_by, current_search.sort) )
	// PrintInfos( "debug", fmt.Sprintf("domain:%s, scope:%s, keyword:%s, order_by:%s, sort:%s", current_search.domain, current_search.scope, current_search.keyword, current_search.order_by, current_search.sort) )

	var page = 1
	var token_index = -1
	var n_token = len(config.tokens)

	for run:=true; run; {

		time.Sleep( config.delay * time.Millisecond )

		token_index = getNextToken( token_index, n_token )

		if token_index < 0 {
			token_index = -1

			if( config.stop_notoken ) {
				PrintInfos("error", "no more token available, exiting")
				os.Exit(-1)
			}

			PrintInfos("error", "no more token available, waiting for another available token...")
			continue
		}

		var url = fmt.Sprintf("https://%s/api/v4/search?scope=%s&search=%s&order_by=%s&sort=%s&page=%d&per_page=200", current_search.domain, current_search.scope, current_search.keyword, current_search.order_by, current_search.sort, page )
		// var url = fmt.Sprintf("https://gitlab.com/api/v4/search?scope=blobs&search=%s&order_by=%s&sort=%s&page=%d", current_search.keyword, current_search.order_by, current_search.sort, page )
		PrintInfos( "debug", url )

		var t_json = doRequest( current_search.domain, config.tokens[token_index].datoken, url )
		var n_results = len(t_json)

		if n_results <= 0 {
			run = false
			break
		}

		doRegexp( current_search.scope, t_json )

		page++
		// run = false
	}
}


func doRequest(domain string, token string, url string) []map[string]interface {} {

	defer func() {
        if r := recover(); r != nil {
            // fmt.Println("Recovered in f", r)
        }
    }()

	tr := &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    }

	client := http.Client{ Timeout: time.Second * 10, Transport: tr }

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		PrintInfos( "error", fmt.Sprintf("%s %s (1)",url,err) )
	}

	if( domain == "gitlab.com" ) {
		req.Header.Set("PRIVATE-TOKEN", token)
		// req.Header.Set("PRIVATE-TOKEN", "fake")
	}

	res, getErr := client.Do(req)
	if getErr != nil {
		PrintInfos( "error", fmt.Sprintf("%s %s (2)",url,getErr) )
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		PrintInfos( "error", fmt.Sprintf("%s %s (3)",url,readErr) )
	}
	// fmt.Printf("%s\n", body)

	var t_json []map[string]interface {}

	var t_json_error map[string]interface {}
	jErr := json.Unmarshal([]byte(body), &t_json_error)
	if jErr == nil {
		PrintInfos( "error", fmt.Sprintf("%s %s (4)",url,t_json_error["message"]) )
		} else {
		jErr2 := json.Unmarshal([]byte(body), &t_json)
		if jErr2 != nil {
			PrintInfos( "error", fmt.Sprintf("%s %s (5)",url,jErr2) )
		}
	}

	return t_json
}


func doRegexp(scope string, t_json []map[string]interface {}) {

	var t_match [][]byte

	for _,t_item := range t_json {
		// var t_item2 = mytoArray(t_item)

		// for key,value := range t_item2 {
		// 	fmt.Printf("%s %s\n",key,value)
		// }

		for _,value := range t_item {

			t_match = performRegexp( fmt.Sprintf("%s",value), config.DomainRegexp )

			if len(t_match) > 0 {
				for _, match := range t_match {
					var str_match = cleanSubdomain( match )
					if !inArray(str_match,t_subdomains) {
						t_subdomains = append( t_subdomains, str_match )
						var url = buildUrl(scope, t_item)
						if len(url) > 0 {
							PrintInfos( "info", buildUrl(scope, t_item) )
						}
						PrintInfos( "found", str_match )
						config.fpOutput.WriteString(str_match+"\n")
						config.fpOutput.Sync()
					}
				}
			}
		}
	}
}


func buildUrl(scope string, t_item map[string]interface {}) string {
	var url = ""

	if scope == "projects" {
		return fmt.Sprintf("%s",t_item["web_url"])
	}

	if scope == "issues" {
		return fmt.Sprintf("%s",t_item["web_url"])
	}

	if scope == "merge_requests" {
		return fmt.Sprintf("%s",t_item["web_url"])
	}

	if scope == "milestones" {
		return fmt.Sprintf("%s",t_item["web_url"])
	}

	if scope == "snippet_titles" {
		return fmt.Sprintf("%s",t_item["web_url"])
	}

	if scope == "users" {
		return fmt.Sprintf("%s",t_item["web_url"])
	}

	if scope == "blobs" {
		var path = strings.Replace( fmt.Sprint(t_item["path"]), "/", "%2f", -1 )
		var project_id,_ = t_item["project_id"]
		var ref,_ = t_item["ref"]
		return fmt.Sprintf("https://gitlab.com/api/v4/projects/%.0f/repository/files/%s/raw?ref=%s",project_id,path,ref)
	}

	if scope == "commits" {
		return fmt.Sprintf("%s",t_item["web_url"])
	}

	if scope == "notes" {
		return ""
		return "notes"
	}

	if scope == "wiki_blobs" {
		return ""
		return "wiki_blobs"
	}

	return url
}

func cleanSubdomain(sub []byte) string {
	var clean_sub = string(sub)
	clean_sub = strings.ToLower( clean_sub )
	clean_sub = strings.TrimLeft( clean_sub, "." )
	if strings.Index(clean_sub,"2f") == 0 {
		clean_sub = clean_sub[2:]
	}
	if strings.Index(clean_sub,"252f") == 0 {
		clean_sub = clean_sub[4:]
	}
	var re = regexp.MustCompile( `^u00[0-9a-f][0-9a-f]` )
	clean_sub = re.ReplaceAllString( clean_sub, "" )

	return clean_sub
}


func main() {
	banner()
	var token string
	var outputFileName string

	flag.StringVar( &config.domain, "d", "", "domain you are looking for (required)" )
	flag.BoolVar( &config.extend, "e", false, "extended mode, also look for <dummy>example.com" )
	flag.BoolVar( &config.debug, "debug", false, "debug mode" )
	flag.StringVar( &token, "t", "", "gitlab token (required), can be:\n  • a single token\n  • a list of tokens separated by comma\n  • a file (.tokens) containing 1 token per line\nif the options is not provided, the environment variable GITLAB_TOKEN is readed, it can be:\n  • a single token\n  • a list of tokens separated by comma" )
	flag.StringVar(&outputFileName, "o", "", "output file name")
	flag.Parse()

	if config.domain == "" {
		flag.Usage()
		fmt.Printf("\ndomain not found\n")
		os.Exit(-1)
	}

	var fp *os.File
	if outputFileName != "" {
		var err error
		fp, err = os.Create(outputFileName)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		defer fp.Close()
		config.fpOutput = fp // Assign the file pointer to config.fpOutput
	}

	u, _ := tld.Parse("http://"+config.domain)
	// fmt.Println(u.Domain)
	// fmt.Println(u.Subdomain)

	if config.extend {
		// extended mode activated
		config.search = u.Domain
		config.DomainRegexp = regexp.MustCompile( `(?i)[0-9a-z\-\.]+\.([0-9a-z\-]+)?`+u.Domain+`([0-9a-z\-\.]+)?\.[a-z]{1,5}`)
	} else {
		if( len(u.Subdomain) == 0 ) {
			// no extended mode and no subdomain
			config.search = u.Domain + "." + u.TLD
			config.DomainRegexp = regexp.MustCompile( `(?i)(([0-9a-z\-\.]+)\.)?` + u.Domain + "\\." + u.TLD )
			} else {
			// no extended mode but subdomain is provided
			config.search = u.Subdomain + "." + u.Domain + "." + u.TLD
			config.DomainRegexp = regexp.MustCompile( `(?i)(([0-9a-z\-\.]+)\.)?` + u.Subdomain + "\\." + u.Domain + "\\." + u.TLD )
		}
	}

	// fmt.Println(config.search)
	// fmt.Println(config.DomainRegexp)
	// os.Exit(-1)

	config.search = "%22" + strings.ReplaceAll(url.QueryEscape(config.search), "-", "%2D") + "%22"

	parseToken( token )

	var n_token = len(config.tokens)
	if n_token == 0 {
		flag.Usage()
		PrintInfos( "error", "token not found" )
		os.Exit(-1)
	}

	var wg sync.WaitGroup
	var max_procs = make(chan bool, 30)

	config.delay = time.Duration( 60.0 / (30*float64(n_token)) * 1000 + 200)

	displayConfig()

	// https://docs.gitlab.com/ee/api/search.html
	// Allowed values are created_at only. If not set, results are sorted by created_at in descending order for basic search, or by the most relevant documents for Advanced Search.
	var order_by = "created_at"
	// Allowed values are asc or desc only. If not set, results are sorted by created_at in descending order for basic search, or by the most relevant documents for Advanced Search.
	var sort = "desc"
	var n_search = 0

	// for _,scope := range t_scopes {
	// 	var current_search = Search{domain:"gitlab.com", scope:scope, keyword:config.search, order_by:order_by, sort:sort}
	// 	PrintInfos( "debug", fmt.Sprintf("scope:%s, keyword:%s, order_by:%s, sort:%s", current_search.scope, current_search.keyword, current_search.order_by, current_search.sort) )

	// 	doSearch( current_search )
	// 	n_search++
	// }

	var skip = false

	if !skip {
		for _,scope := range t_scopes {
			wg.Add(1)
			go func(scope string) {
				defer wg.Done()
				max_procs<-true
				var current_search = Search{domain:"gitlab.com", scope:scope, keyword:config.search, order_by:order_by, sort:sort}
				doSearch( current_search )
				<-max_procs
			}(scope)
		}
		wg.Wait()
	}




	PrintInfos( "", fmt.Sprintf("%d searches performed",n_search) )
	PrintInfos( "", fmt.Sprintf("%d subdomains found",len(t_subdomains)) )
}


func inArray(str string, array []string) bool {
	for _,i := range array {
		if i == str {
			return true
		}
	}
	return false
}


func inArrayKey(str string, array map[string]interface {}) bool {
	for i,_ := range array {
		if i == str {
			return true
		}
	}
	return false
}


func performRegexp(code string, rgxp *regexp.Regexp ) [][]byte {
	return rgxp.FindAll([]byte(code), -1)
}


func resliceTokens(s []Token, index int) []Token {
    return append(s[:index], s[index+1:]...)
}


func displayConfig() {
	PrintInfos( "", fmt.Sprintf("Domain:%s, Output:%s",config.domain,config.output) )
	PrintInfos( "", fmt.Sprintf("Tokens:%d, Delay:%.0fms",len(config.tokens),float32(config.delay)) )
	PrintInfos( "", fmt.Sprintf("Token rehab:%t, Quick mode:%t",!config.stop_notoken,config.quick_mode) )
}


func PrintInfos(infos_type string, str string) {

	if !config.debug && infos_type == "found" {
		fmt.Println( str )
	} else if config.debug {
		str = fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), str )

		switch infos_type {
			case "debug":
				fmt.Println( au.Gray(13,str).Bold() )
			case "info":
				fmt.Println( au.Yellow(str).Bold() )
			case "found":
				fmt.Println( au.Green(str).Bold() )
			case "error":
				fmt.Println( au.Red(str).Bold() )
			default:
				fmt.Println( au.White(str).Bold() )
		}
	}
}


func banner() {
	if !isOutputRedirectedToFile(){
	fmt.Print("\n")
	fmt.Print(`
  	   ▗▐  ▜    ▌          ▌    ▌          ▗
	▞▀▌▄▜▀ ▐ ▝▀▖▛▀▖  ▞▀▘▌ ▌▛▀▖▞▀▌▞▀▖▛▚▀▖▝▀▖▄ ▛▀▖▞▀▘
	▚▄▌▐▐ ▖▐ ▞▀▌▌ ▌  ▝▀▖▌ ▌▌ ▌▌ ▌▌ ▌▌▐ ▌▞▀▌▐ ▌ ▌▝▀▖
	▗▄▘▀▘▀  ▘▝▀▘▀▀   ▀▀ ▝▀▘▀▀ ▝▀▘▝▀ ▘▝ ▘▝▀▘▀▘▘ ▘▀▀
	`)
	fmt.Print("       by @gwendallecoguic                         \n\n")
}}

func isOutputRedirectedToFile() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) == 0
}