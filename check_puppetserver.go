package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/xorpaul/go-nagios"
)

var debug bool

type QueryResponse struct {
	ServerResponse []byte
	Time           float64
}

type PuppetServerResponse struct {
	Is_alive bool   `json:"is_alive"`
	Version  string `json:"version"`
}

// Debugf is a helper function for debug logging if mainCfgSection["debug"] is set
func Debugf(s string) {
	if debug {
		log.Print("DEBUG " + s)
	}
}

func sendQuery(url string, client *http.Client) QueryResponse {

	var resp *http.Response
	var out []byte

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		log.Println(err)
		os.Exit(3)
	}
	req.Header.Add("Accept", "*/*")
	before := time.Now()
	resp, err = client.Do(req)
	duration := time.Since(before).Seconds()
	Debugf("Sending query " + url + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")

	if err != nil {
		Debugf("Error while sending request to " + url + "err: " + err.Error())
		nr := nagios.NagiosResult{ExitCode: 3, Text: "Error while sending request: " + err.Error(), Perfdata: "time=" + strconv.FormatFloat(duration, 'f', 5, 64) + "s"}
		nagios.NagiosExit(nr)
	}
	defer resp.Body.Close()
	out, err = ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Println(err)
		os.Exit(3)
	}
	Debugf("Response is: " + string(out))
	return QueryResponse{ServerResponse: out, Time: duration}
}

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)

	hostnameOut, err := exec.Command("hostname", "-f").CombinedOutput()
	if _, ok := err.(*exec.ExitError); ok { // there is error code
		Debugf("WARNING: hostname -f failed with output:" + string(hostnameOut))
	}

	fqdn := strings.TrimSpace(string(hostnameOut))

	var (
		hostFlag        = flag.String("H", "localhost", "Hostname to query, defaults to localhost")
		portFlag        = flag.Int("p", 8140, "Port to send the query to, defaults to 8140")
		environmentFlag = flag.String("e", "production", "Puppet environment to ask for, defaults to production")
		warningFlag     = flag.Float64("w", 5, "WARNING threshold in seconds, defaults to 5 seconds")
		criticalFlag    = flag.Float64("c", 15, "CRITICAL threshold in seconds, defaults to 15 seconds")
		debugFlag       = flag.Bool("debug", false, "log debug output, defaults to false")
		// tls flags
		certFile = flag.String("cert", "/etc/puppetlabs/puppet/ssl/certs/"+fqdn+".pem", "A PEM eoncoded client certificate file, defaults to /etc/puppetlabs/puppet/ssl/certs/"+fqdn+".pem")
		keyFile  = flag.String("key", "/etc/puppetlabs/puppet/ssl/private_keys/"+fqdn+".pem", "A PEM encoded private key file for the client certificate, defaults to /etc/puppetlabs/puppet/ssl/private_keys/"+fqdn+".pem")
	)

	flag.Parse()

	if len(os.Getenv("VIMRUNTIME")) > 0 {
		*hostFlag = "localhost"
		*certFile = "/home/andpaul/dev/go/src/github.com/xorpaul/check_puppetserver/cert.pem"
		*keyFile = "/home/andpaul/dev/go/src/github.com/xorpaul/check_puppetserver/key.pem"
		*debugFlag = true
		*criticalFlag = 0.02
	}

	if *hostFlag == "" {
		log.Println("Hostname parameter -H is mandatory!")
		os.Exit(1)
	}
	if *certFile == "" {
		log.Println("Client certificate parameter -cert is mandatory!")
		os.Exit(1)
	}
	if *keyFile == "" {
		log.Println("Client certificate key file parameter -key is mandatory!")
		os.Exit(1)
	}

	debug = *debugFlag

	// TLS stuff
	tlsConfig := &tls.Config{}
	tlsConfig.InsecureSkipVerify = true

	// initialize http client with defaults
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}

	var certFilenames = map[string]string{
		"cert": *certFile,
		"key":  *keyFile,
	}

	for _, filename := range certFilenames {
		if filename != "" {
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				// generate certs
				log.Println("Certificate file: " + filename + " not found! Exiting...\n")
				os.Exit(1)
			} else {
				Debugf("Certificate file: " + filename + " found.\n")
			}
		}
	}

	Debugf("Trying to load cert file: " + *certFile + " and key file: " + *keyFile)
	mycert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		panic(err)
	}

	tlsConfig.Certificates = make([]tls.Certificate, 1)
	tlsConfig.Certificates[0] = mycert

	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.BuildNameToCertificate()

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client = &http.Client{Transport: transport}

	url := "https://" + *hostFlag + ":" + strconv.Itoa(*portFlag) + "/puppet/v3/status/whatever?environment=" + *environmentFlag
	out := sendQuery(url, client)
	time := strconv.FormatFloat(out.Time, 'f', 5, 64)
	nr := nagios.NagiosResult{ExitCode: 3, Text: "uncatched case", Perfdata: "time=" + time + "s"}

	if len(out.ServerResponse) > 0 {
		var r PuppetServerResponse
		err := json.Unmarshal(out.ServerResponse, &r)
		if err != nil {
			Debugf("Error while decoding JSON response")
			nr.ExitCode = 1
			nr.Text = "Failed parsing Puppet Server JSON response, checked for Puppet environment " + *environmentFlag + " output: " + string(out.ServerResponse)
		} else {
			if r.Is_alive {
				nr.ExitCode = 0
				nr.Text = "Puppet Server (Version: " + r.Version + ") looks good, checked for Puppet environment " + *environmentFlag + " in " + time + "s"
			} else {
				nr.ExitCode = 1
				nr.Text = "Puppet Server (Version: " + r.Version + ") did not answer with is_alive true, checked for Puppet environment " + *environmentFlag
			}
		}

	} else {
		nr.ExitCode = 1
		nr.Text = "Received empty response for request against " + url
	}

	if out.Time >= *criticalFlag {
		nr.ExitCode = 2
		nr.Text = "Response time " + time + "s >= " + strconv.FormatFloat(*criticalFlag, 'f', 2, 64) + "s - " + nr.Text
	} else if out.Time >= *warningFlag {
		nr.ExitCode = 1
		nr.Text = "Response time " + time + "s >= " + strconv.FormatFloat(*warningFlag, 'f', 2, 64) + "s - " + nr.Text
	}

	nagios.NagiosExit(nr)
}
