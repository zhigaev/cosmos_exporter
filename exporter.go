package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"encoding/json"
        "time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ProtocolVersion struct {
  P2p	string `json:"p2p"`
  Block string `json:"block"`
  App 	string `json:"app"`
}

type Other struct {
  TxIndex 	string `json:"tx_index"`
  RpcAddress 	string `json:"rpc_address"`
}

type PubKey struct {
  Type 	string `json:"type"`
  Value string `json:"value"`
}

type NodeInfo struct {
  ProtoVer 	ProtocolVersion `json:"protocol_version"`
  Id 		string `json:"id"`
  ListenAddr 	string `json:"listen_addr"`
  Network 	string `json:"network"`
  Version 	string `json:"version"`
  Channels 	string `json:"channels"`
  Moniker 	string `json:"moniker"`
  InfoOther 	Other `json:"other"`
}

type SyncInfo struct {
  LatestBlockHash 	string `json:"latest_block_hash"`
  LatestAppHash 	string `json:"latest_app_hash"`
  LatestBlockHeight 	string `json:"latest_block_height"`
  LatestBlockTime 	string `json:"latest_block_time"`
  EarlestBlockHash 	string `json:"earlest_block_hash"`
  EarlestAppHash 	string `json:"earlest_app_hash"`
  EarlestBlockHeight 	string `json:"earlest_block_height"`
  EarlestBlockTime 	string `json:"earlest_block_time"`
  CatchingUp 		bool `json:"catching_up"`
}

type ValidatorInfo struct {
  Address 	string `json:"address"`
  InfoPubKey  	PubKey `json:"pub_key"`
  VotingPower 	string  `json:"voting_power"`
}

type Result struct {
  MessageNodeInfo 	NodeInfo `json:"node_info"`
  MessageSyncInfo 	SyncInfo `json:"sync_info"`
  MessageValidatorInfo  ValidatorInfo `json:"validator_info"`
}

type Message struct {
  Jsonrpc   	string `json:"jsonrpc"`
  Id   		int64  `json:"id"`
  MessageResult Result `json:"result"`
}

const namespace = "cosmos"
const url = "/status"

var (
	client = &http.Client{Timeout: 10 * time.Second}
	listenAddress = flag.String("web.listen-address", ":9141",
		"Address to listen on for telemetry")
	metricsPath = flag.String("web.telemetry-path", "/metrics",
		"Path under which to expose metrics")

	// Metrics
	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last cosmos query successful.",
		nil, nil,
	)
	latestBlockHeight = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "latest_block_height"),
		"Latest block height",
		[]string{"node"}, nil,
	)
	timeDiff = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "time_diff"),
		"Time difference",
		[]string{"node"}, nil,
	)
)

type Exporter struct {
	cosmosEndpoint string
}

func NewExporter(cosmosEndpoint string) *Exporter {
	return &Exporter{
		cosmosEndpoint: cosmosEndpoint,
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- latestBlockHeight
	ch <- timeDiff
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	req, err := http.NewRequest("GET", e.cosmosEndpoint+url, nil)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0,
		)
		log.Println(err)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		up, prometheus.GaugeValue, 1,
	)

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

        message := Message{}
        jsonErr := json.Unmarshal(body, &message)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}

	channellatestBlockHeight, _ := strconv.ParseFloat(message.MessageResult.MessageSyncInfo.LatestBlockHeight, 64)
	ch <- prometheus.MustNewConstMetric(latestBlockHeight, prometheus.GaugeValue, channellatestBlockHeight, "localhost")

        layout := "2006-01-02T15:04:05.999999999Z07:00"
        t, err := time.Parse(layout, message.MessageResult.MessageSyncInfo.LatestBlockTime)
	if err != nil {
		log.Println(err)
	}

        now := time.Now()
        secs := now.Unix()
        diff := secs - t.Unix()

	channeltimeDiff, _ := strconv.ParseFloat(strconv.FormatInt(diff, 10), 64)
        ch <- prometheus.MustNewConstMetric(timeDiff, prometheus.GaugeValue, channeltimeDiff, "localhost")

	log.Println("Endpoint scraped")
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, assume env variables are set.")
	}

	flag.Parse()

	cosmosEndpoint := os.Getenv("COSMOS_ENDPOINT")

	exporter := NewExporter(cosmosEndpoint)
	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Dummy Cosmos Exporter</title></head>
             <body>
             <h1>Dummy Cosmos Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
