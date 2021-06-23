package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"text/template"

	"cloud.google.com/go/compute/metadata"
)

type gcpInstance struct {
	Attributes struct {
	} `json:"attributes"`
	CPUPlatform string `json:"cpuPlatform"`
	Description string `json:"description"`
	Disks       []struct {
		DeviceName string `json:"deviceName"`
		Index      int    `json:"index"`
		Interface  string `json:"interface"`
		Mode       string `json:"mode"`
		Type       string `json:"type"`
	} `json:"disks"`
	GuestAttributes struct {
	} `json:"guestAttributes"`
	Hostname             string `json:"hostname"`
	ID                   int64  `json:"id"`
	Image                string `json:"image"`
	LegacyEndpointAccess struct {
		Zero1   int `json:"0.1"`
		V1Beta1 int `json:"v1beta1"`
	} `json:"legacyEndpointAccess"`
	Licenses []struct {
		ID string `json:"id"`
	} `json:"licenses"`
	MachineType       string `json:"machineType"`
	MaintenanceEvent  string `json:"maintenanceEvent"`
	Name              string `json:"name"`
	NetworkInterfaces []struct {
		AccessConfigs []struct {
			ExternalIP string `json:"externalIp"`
			Type       string `json:"type"`
		} `json:"accessConfigs"`
		DNSServers        []string      `json:"dnsServers"`
		ForwardedIps      []interface{} `json:"forwardedIps"`
		Gateway           string        `json:"gateway"`
		IP                string        `json:"ip"`
		IPAliases         []interface{} `json:"ipAliases"`
		Mac               string        `json:"mac"`
		Mtu               int           `json:"mtu"`
		Network           string        `json:"network"`
		Subnetmask        string        `json:"subnetmask"`
		TargetInstanceIps []interface{} `json:"targetInstanceIps"`
	} `json:"networkInterfaces"`
	Preempted        string `json:"preempted"`
	RemainingCPUTime int    `json:"remainingCpuTime"`
	Scheduling       struct {
		AutomaticRestart  string `json:"automaticRestart"`
		OnHostMaintenance string `json:"onHostMaintenance"`
		Preemptible       string `json:"preemptible"`
	} `json:"scheduling"`
	ServiceAccounts struct {
		Three26640127649ComputeDeveloperGserviceaccountCom struct {
			Aliases []string `json:"aliases"`
			Email   string   `json:"email"`
			Scopes  []string `json:"scopes"`
		} `json:"326640127649-compute@developer.gserviceaccount.com"`
		Default struct {
			Aliases []string `json:"aliases"`
			Email   string   `json:"email"`
			Scopes  []string `json:"scopes"`
		} `json:"default"`
	} `json:"serviceAccounts"`
	Tags         []string `json:"tags"`
	VirtualClock struct {
		DriftToken string `json:"driftToken"`
	} `json:"virtualClock"`
	Zone string `json:"zone"`
}

//Index holds fields displayed on the index.html template
type Index struct {
	Hostname string
	Tags     []string
}

//go:embed static
var staticFiles embed.FS

//go:embed templates/index.html
var indexFile string

//go:embed templates/err.html
var errFile string

// This example demonstrates how to use your own transport when using this package.
func main() {

	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		fmt.Print(err.Error())
	}
	http.Handle("/static/",
		http.StripPrefix("/static/",
			http.FileServer(http.FS(fsys))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if metadata.OnGCE() {
			h, err := metadata.Hostname()
			if err != nil {
				fmt.Print(err.Error())
			}
			t, err := metadata.InstanceTags()
			if err != nil {
				fmt.Print(err.Error())
			}

			index := Index{h, t}

			// template := template.Must(template.ParseFiles("templates/index.html"))
			template := template.Must(template.New("index").Parse(indexFile))

			if err := template.ExecuteTemplate(w, "index", index); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			template := template.Must(template.New("err").Parse(errFile))
			// template := template.Must(template.ParseFiles("templates/err.html"))

			if err := template.ExecuteTemplate(w, "err", nil); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	})
	fmt.Println(http.ListenAndServe(":80", nil))
}
