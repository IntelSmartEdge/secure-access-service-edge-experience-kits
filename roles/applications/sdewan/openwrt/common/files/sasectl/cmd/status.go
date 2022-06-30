/**
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
**/
package cmd

import (
	"encoding/json"
	"log"
	"sasectl/utils"
	"strings"

	"github.com/akraino-edge-stack/icn-sdwan/central-controller/src/scc/pkg/module"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current cluster role",
	Run: func(cmd *cobra.Command, args []string) {
		// fmt.Println("status called")
		var statusOutput strings.Builder
		if sasectlConf.ICNSdewanRole != "" {
			statusOutput.WriteString("Current cluster is initialized as ")
			statusOutput.WriteString(sasectlConf.ICNSdewanRole)
		} else {
			statusOutput.WriteString("Cluster has not been initialized.")
		}
		log.Println(statusOutput.String())
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func queryControllerObjects(baseUrl string) (string, error) {
	res, err := utils.CallRest("GET", baseUrl, "")
	if err != nil {
		log.Println(err.Error())
	}

	return res, nil
}

func queryConnections(serverIP string, overlay string, hubName string) ([]module.HubDeviceObject, error) {
	var conObjs []module.HubDeviceObject

	conUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.HubCollection + "/" + hubName + "/" + utils.ConnectionCollection

	res, err := queryControllerObjects(conUrl)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	err = json.Unmarshal([]byte(res), &conObjs)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	return conObjs, nil
}

func queryHubs(serverIP string, overlay string) ([]module.HubObject, error) {
	var hubObjs []module.HubObject

	hubUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.HubCollection

	res, err := queryControllerObjects(hubUrl)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	err = json.Unmarshal([]byte(res), &hubObjs)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	return hubObjs, nil
}

func queryDevs(serverIP string, overlay string) ([]module.DeviceObject, error) {
	var devObjs []module.DeviceObject

	devUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.DeviceCollection

	res, err := queryControllerObjects(devUrl)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	err = json.Unmarshal([]byte(res), &devObjs)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	return devObjs, nil
}

func queryCerts(serverIP string, overlay string) ([]module.CertificateObject, error) {
	var certObjs []module.CertificateObject

	certUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.CertCollection

	res, err := queryControllerObjects(certUrl)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	err = json.Unmarshal([]byte(res), &certObjs)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	return certObjs, nil
}

func queryIPranges(serverIP string, overlay string) ([]module.IPRangeObject, error) {
	var ipRangeObjs []module.IPRangeObject

	var ipRangeUrl string
	if overlay != "" {
		ipRangeUrl = "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
			"/" + overlay + "/" + utils.IPRangeCollection
	} else {
		ipRangeUrl = "http://" + serverIP + ":9015/scc/v1/provider/" + utils.IPRangeCollection
	}

	res, err := queryControllerObjects(ipRangeUrl)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	err = json.Unmarshal([]byte(res), &ipRangeObjs)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	return ipRangeObjs, nil
}

func queryProposals(serverIP string, overlay string) ([]module.ProposalObject, error) {
	var proposalObjs []module.ProposalObject

	proposalUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.ProposalCollection

	res, err := queryControllerObjects(proposalUrl)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	err = json.Unmarshal([]byte(res), &proposalObjs)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	return proposalObjs, nil
}

func queryOverlays(serverIP string) ([]module.OverlayObject, error) {

	var overlayObjs []module.OverlayObject

	overlayUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection

	res, err := queryControllerObjects(overlayUrl)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	err = json.Unmarshal([]byte(res), &overlayObjs)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	return overlayObjs, err
}
