/**
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
**/
package utils

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"reflect"
	"strings"

	"github.com/akraino-edge-stack/icn-sdwan/central-controller/src/scc/pkg/module"
	"gopkg.in/yaml.v3"
)

type CNFValue map[string]interface{}

type SaseCtlConf struct {
	ICNSdewanFilePath      string `yaml:"ICN-Sdewan-File-Path"`
	ICNSdewanRole          string `yaml:"ICN-Sdewan-Role"`
	ICNSdewanCNFChartName  string `yaml:"ICN-Sdewan-CNF-Chart"`
	ICNSdewanCtrlChartName string `yaml:"ICN-Sdewan-Ctrl-Chart"`
}

type CmdInfo struct {
	CmdName string
	CmdArgs []string
	CmdDir  string
}

type ICNSdewanProposalObject struct {
	Metadata      module.ObjectMetaData       `json:"metadata"`
	Specification ICNSdewanProposalObjectSpec `json:"spec"`
}

//ProposalObjectSpec contains the parameters
type ICNSdewanProposalObjectSpec struct {
	Encryption string `json:"encryption"`
	Hash       string `json:"hash"`
	DhGroup    string `json:"dhGroup"`
}

type ICNSccDbConfig struct {
	EtcdIP string `json:"etcd-ip"`
	DBIP   string `json:"database-ip"`
}

type ICNNfnConfig struct {
	DefaultGateway bool   `yaml:"defaultGateway"`
	Interface      string `yaml:"interface"`
	IPAddress      string `yaml:"ipAddress"`
	Name           string `yaml:"name"`
	Separate       string `yaml:"separate"`
	Namespace      string `yaml:"namespace,omitempty"`
}

type ICNCnfcmYaml struct {
	Kind       string `yaml:"kind"`
	ApiVersion string `yaml:"apiVersion"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Data struct {
		Entrypoint string `yaml:"entrypoint.sh"`
	}
}

func (c *ICNNfnConfig) SetField(key string, value interface{}) {
	switch key {
	case "defaultGateway":
		c.DefaultGateway = reflect.ValueOf(value).Bool()
	case "interface":
		c.Interface = reflect.ValueOf(value).String()
	case "ipAddress":
		c.IPAddress = reflect.ValueOf(value).String()
	case "name":
		c.Name = reflect.ValueOf(value).String()
	case "separate":
		c.Separate = reflect.ValueOf(value).String()
	case "namespace":
		c.Namespace = reflect.ValueOf(value).String()
	default:
		log.Println("Invalid field")
	}
}

func LoadSasectlConfig(fp string) (*SaseCtlConf, error) {
	var c SaseCtlConf
	yamlFile, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &c)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return &c, nil
}

func SetClusterRole(fp string, role string, conf *SaseCtlConf) bool {
	conf.ICNSdewanRole = role
	d, err := yaml.Marshal(conf)
	if err != nil {
		log.Fatal(err)
		return false
	}
	err = ioutil.WriteFile(fp, d, 0644)
	if err != nil {
		log.Fatal(err)
		return false
	}
	return true
}

func CheckPodIP(podName string) string {
	listNameCmd := CmdInfo{
		CmdName: "kubectl",
		CmdArgs: []string{"get", "pod", "-n", "sdewan-system", "--no-headers", "-o", "custom-columns=Name:.metadata.name,IP:.status.podIP"},
	}

	cmd := exec.Command(listNameCmd.CmdName, listNameCmd.CmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
		return ""
	}
	soutput := strings.Split(string(output), "\n")
	for _, item := range soutput {
		boutput := strings.Fields(item)
		if len(boutput) > 1 && strings.Contains(boutput[0], podName) {
			return boutput[1]
		}
	}
	return ""
}

func CheckPodFullname(keyword string) string {
	listNameCmd := CmdInfo{
		CmdName: "kubectl",
		CmdArgs: []string{"get", "pod", "-n", "sdewan-system", "--no-headers", "-o", "custom-columns=Name:.metadata.name"},
	}

	cmd := exec.Command(listNameCmd.CmdName, listNameCmd.CmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
		return ""
	}
	soutput := strings.Split(string(output), "\n")
	for _, item := range soutput {
		boutput := strings.Fields(item)
		if len(boutput) > 0 && strings.Contains(boutput[0], keyword) {
			return boutput[0]
		}
	}
	return ""
}

func GetIPIfName(IPaddr string) string {
	cmd := exec.Command("ip", "route", "get", IPaddr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
		return ""
	}
	soutput := strings.Split(string(output), "\n")
	for _, item := range soutput {
		boutput := strings.Fields(item)
		if len(boutput) > 1 {
			for i, v := range boutput {
				if v == "dev" && i+1 < len(boutput) {
					if strings.HasPrefix(boutput[i+1], "cali") {
						return boutput[i+1]
					} else {
						continue
					}
				}
			}
		}
	}
	return ""
}

func LoadCNFValueFile(cnfValueFp string) CNFValue {

	var existedData []*ICNNfnConfig
	cnfValue := make(map[string]interface{})

	data, err := ioutil.ReadFile(cnfValueFp)
	if err != nil {
		log.Fatal("Failed to open cnf value file.")
		log.Fatal(err)
	}

	yaml.Unmarshal(data, &cnfValue)

	// Covert "nfn" field from nested interface{} to struct array.
	if val, ok := cnfValue["nfn"]; ok {
		data := reflect.ValueOf(val)
		switch data.Kind() {
		case reflect.Slice:
			data := reflect.ValueOf(val)
			for i := 0; i < data.Len(); i++ {
				var thisData ICNNfnConfig
				iData := reflect.ValueOf(data.Index(i).Interface())
				iDataIter := iData.MapRange()
				for iDataIter.Next() {
					k := iDataIter.Key().String()
					v := iDataIter.Value().Interface()
					thisData.SetField(k, v)
				}
				existedData = append(existedData, &thisData)
			}
		default:
			log.Fatal("Error type in nfn field.")
		}
	} else {
		log.Fatal("Illegal cnf value file.")
		return nil
	}
	cnfValue["nfn"] = existedData
	return cnfValue
}

func UpdateCNFValueFile(cnfValueFp string, cnfValue CNFValue) {
	var outData []byte

	cnfValueData, err := yaml.Marshal(cnfValue)
	if err != nil {
		log.Fatal("Failed to export cnf value file.")
	}

	outData = append(outData, []byte(CNFValueCopyright)...)
	outData = append(outData, '\n')
	outData = append(outData, cnfValueData...)

	err = ioutil.WriteFile(cnfValueFp, outData, 0666)
	if err != nil {
		log.Print(err.Error())
		log.Fatal("Failed to export CNF value file.")
	}
}

func ResetCNFValueNFN(cnfValue CNFValue) {

	defaultNfnValue := []*ICNNfnConfig{
		{
			DefaultGateway: false,
			Interface:      "net2",
			IPAddress:      "10.10.70.39",
			Name:           "pnetwork",
			Separate:       ",",
		},
		{
			DefaultGateway: false,
			Interface:      "net0",
			IPAddress:      "172.16.70.39",
			Name:           "ovn-network",
			Separate:       "",
		},
	}
	cnfValue["nfn"] = defaultNfnValue
}

func GenerateCMYaml(cmFp string, clusterRole string) {

	cmYamlData := ICNCnfcmYaml{
		Kind:       "ConfigMap",
		ApiVersion: "v1",
	}
	cmYamlData.Metadata.Name = "sdewan-safe-sh"
	cmYamlData.Metadata.Namespace = "sdewan-system"
	entrypointSh := []byte(CNFBaseShell)
	switch clusterRole {
	case "popoverlay":
		break
	default:
		entrypointSh = append(entrypointSh, '\n')
		entrypointSh = append(entrypointSh, []byte(CNFRouterShell)...)
	}
	entrypointSh = append(entrypointSh, []byte(CNFTemplateShell)...)
	cmYamlData.Data.Entrypoint = string(entrypointSh)

	cmYaml, err := yaml.Marshal(cmYamlData)
	if err != nil {
		log.Fatal("Failed to parse cm yaml data for CNF")
	}

	outData := []byte(CNFCMCopyright)
	outData = append(outData, '\n')
	outData = append(outData, cmYaml...)

	err = ioutil.WriteFile(cmFp, outData, 0664)
	if err != nil {
		log.Print(err.Error())
		log.Fatal("Failed to gemerate CNF config map template.")
	}
}

func CallRest(method string, url string, request string) (string, error) {
	log.Printf("%s    %s    %s\n", method, url, request)
	client := &http.Client{}
	req_body := bytes.NewBuffer([]byte(request))
	req, err := http.NewRequest(method, url, req_body)
	if err != nil {
		log.Print("Failed to create request to server.")
		log.Print(err.Error())
	}

	req.Header.Set("Cache-Control", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Print("Failed to parse returned body content.")
		log.Print(err.Error())
	}

	if resp.StatusCode >= 400 {
		return "", errors.New(string(body))
	}

	return string(body), nil
}
