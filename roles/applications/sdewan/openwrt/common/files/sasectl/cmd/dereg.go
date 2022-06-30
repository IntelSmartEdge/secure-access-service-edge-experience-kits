/**
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
**/
package cmd

import (
	"log"
	"sasectl/utils"

	"github.com/spf13/cobra"
)

var deregCmd = &cobra.Command{
	Use:   "dereg",
	Short: "Deregister operation for Smart-Edge Open SASE EK",
	// Run: func(cmd *cobra.Command, args []string) {
	// },
}

var deregEdgeCmd = &cobra.Command{
	Use:   "edge",
	Short: "Deregister operation for edge cluster.",
	// Run: func(cmd *cobra.Command, args []string) {
	// },
}

var edgeDeregToControllerCmd = &cobra.Command{
	Use:   "toOverlay",
	Short: "Deregister connection between edge and overlay controller.",
	Run: func(cmd *cobra.Command, args []string) {
		deregEdgeToOverlay()
	},
}

var deregOverlayCmd = &cobra.Command{
	Use:   "overlay",
	Short: "Deregister operation for overlay controller cluster.",
	// Run: func(cmd *cobra.Command, args []string) {
	// },
}

var overlayDepreRegCmd = &cobra.Command{
	Use:   "depreReg",
	Short: "Deregister device from overlay controller.",
	// Run: func(cmd *cobra.Command, args []string) {
	// 	deregOverlayDePreReg()
	// },
}

var overlayDeregDev = &cobra.Command{
	Use:   "deregDev",
	Short: "Deregister device from overlay controller.",
	// Run: func(cmd *cobra.Command, args []string) {
	// 	deregOverlayDeregDev()
	// },
}

var overlayDeregCon = &cobra.Command{
	Use:   "deregCon",
	Short: "Deregister connection from overlay controller.",
	// Run: func(cmd *cobra.Command, args []string) {
	// 	deregOverlayDeregCon()
	// },
}

// func init() {
// 	deregEdgeCmd.AddCommand(edgeDeregToControllerCmd)
// 	deregCmd.AddCommand(deregEdgeCmd)

// 	deregOverlayCmd.AddCommand(overlayDepreRegCmd)
// 	deregOverlayCmd.AddCommand(overlayDeregDev)
// 	deregOverlayCmd.AddCommand(overlayDeregCon)
// 	deregCmd.AddCommand(deregOverlayCmd)
// 	rootCmd.AddCommand(deregCmd)
// }

func deleteControllerObjects(baseUrl string) error {
	resp, err := utils.CallRest("DELETE", baseUrl, "")
	if err != nil {
		log.Print(resp)
		return err
	}
	return nil
}

func deregEdgeToOverlay() {
	edgeCleanIpsecCRsApiServer()
}

func deregOverlayDePreReg() {
	serverIP := utils.CheckPodIP("scc")
	// 3 Nodes PreReg Con
	// TODO: Provide more general way.
	overlay := "overlay1"
	overlayProposal1 := "proposal1"
	overlayProposal2 := "proposal2"

	providerIPrangeName := "provideripr"
	dataIPRangeName := "dataipr"

	resetOverlayIPRule("40")
	err := deregIPRange(serverIP, overlay, dataIPRangeName)
	if err != nil {
		log.Print(err.Error())
	}
	err = deregIPRange(serverIP, "", providerIPrangeName)
	if err != nil {
		log.Print(err.Error())
	}
	err = deregProposal(serverIP, overlay, overlayProposal2)
	if err != nil {
		log.Print(err.Error())
	}
	err = deregProposal(serverIP, overlay, overlayProposal1)
	if err != nil {
		log.Print(err.Error())
	}
	err = deregOverlay(serverIP, overlay)
	if err != nil {
		log.Print(err.Error())
	}
}

func deregOverlayDeregDev(devType string, deviceName string) error {
	serverIP := utils.CheckPodIP("scc")
	var err error
	if devType == "edge" {
		err = deregDevice(serverIP, "overlay1", deviceName)
		if err != nil {
			log.Print(err.Error())
			return err
		}
		err = deregCert(serverIP, "overlay1", deviceName)
		if err != nil {
			log.Print(err.Error())
			return err
		}
	} else if devType == "pop" || devType == "popoverlay" {
		err = deregHub(serverIP, "overlay1", deviceName)
		if err != nil {
			log.Print(err.Error())
			return err
		}
	} else {
		log.Fatal("Illegal device type")
	}
	return nil
}

func deregOverlayDeregCon(overlay string, deviceName string, hubName string) error {
	serverIP := utils.CheckPodIP("scc")

	deregConUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.HubCollection + "/" + hubName + "/" + utils.DeviceCollection +
		"/" + deviceName

	err := deleteControllerObjects(deregConUrl)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func deregOverlayCon(overlay string, deviceName string, hubName string) error {
	serverIP := utils.CheckPodIP("scc")
	regConUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.HubCollection + "/" + hubName + "/" + utils.DeviceCollection

	// conName := hubName + strings.Replace(deviceName, "-", "", -1) + "conn"

	// regConReq := module.HubDeviceObject{
	// 	Metadata: module.ObjectMetaData{conName, "", "", ""},
	// 	Specification: module.HubDeviceObjectSpec{
	// 		Device:        deviceName,
	// 		IsDelegateHub: true,
	// 	},
	// }
	err := deleteControllerObjects(regConUrl)
	if err != nil {
		log.Print(err.Error())
		return err
	}
	return nil
}

func deregOverlay(serverIP string, overlay string) error {
	overlayUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay

	_, err := utils.CallRest("DELETE", overlayUrl, "")
	if err != nil {
		log.Println(err.Error())
		return err
	}

	return nil
}

func deregProposal(serverIP string, overlay string, proposal string) error {
	proposalUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.ProposalCollection + "/" + proposal

	_, err := utils.CallRest("DELETE", proposalUrl, "")
	if err != nil {
		log.Println(err.Error())
		return err
	}

	return nil
}

func deregDevice(serverIP string, overlay string, deviceName string) error {
	DeviceUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.DeviceCollection + "/" + deviceName
	_, err := utils.CallRest("DELETE", DeviceUrl, "")
	if err != nil {
		log.Println(err.Error())
		return err
	}
	return nil
}

func deregHub(serverIP string, overlay string, hubName string) error {
	HubUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.HubCollection + "/" + hubName

	_, err := utils.CallRest("DELETE", HubUrl, "")
	if err != nil {
		log.Println(err.Error())
		return err
	}

	return nil
}

func deregIPRange(serverIP string, overlay string, ipRangeName string) error {
	var ipRangeUrl string
	if overlay != "" {
		ipRangeUrl = "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
			"/" + overlay + "/" + utils.IPRangeCollection + "/" + ipRangeName
	} else {
		ipRangeUrl = "http://" + serverIP + ":9015/scc/v1/provider/" + utils.IPRangeCollection +
			"/" + ipRangeName
	}

	_, err := utils.CallRest("DELETE", ipRangeUrl, "")
	if err != nil {
		log.Println(err.Error())
		return err
	}

	return nil
}

func deregCert(serverIP string, overlay string, deviceName string) error {
	CertUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.CertCollection

	err := deleteControllerObjects(CertUrl)
	if err != nil {
		log.Print("Failed to delete Certs")
		return err
	}
	return nil
}
