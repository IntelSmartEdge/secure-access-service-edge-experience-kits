/**
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
**/
package cmd

import (
	"log"
	"os/exec"
	"path/filepath"
	"sasectl/utils"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	IpsecHosts     = "ipsechosts.batch.sdewan.akraino.org"
	IpsecProposals = "ipsecproposals.batch.sdewan.akraino.org"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Clean cluster role of SASE-EK",
	Run: func(cmd *cobra.Command, args []string) {
		switch sasectlConf.ICNSdewanRole {
		case "edge":
			resetEdgeCluster()
		case "pop":
			resetPopCluster()
		case "overlay", "popoverlay":
			resetOverlayCluster()
		default:
			log.Fatal("Cluster didn't initialized as any role!")
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}

func resetEdgeCluster() {
	log.Println("Reset cluster role")
	log.Println("Delete custom resources of edge cluster.")
	edgeCleanIpsecCRsApiServer()
	log.Println("Reset data plane of edge cluster.")
	resetDataplane()
	log.Println("Successfully reset cluster role")
}

func resetPopCluster() {
	log.Println("Reset cluster role")
	// resetIpsecCRs()
	resetDataplane()
	log.Println("Successfully reset cluster role")
}

func resetOverlayCluster() {
	log.Println("Reset cluster role")
	// overlayWorkingDir := filepath.Join(sasectlConf.ICNSdewanFilePath, "central-controller/deployments/kubernetes")
	log.Println("Delete custom resources of overlay cluster.")
	overlayCleanIpsecCRsRest()
	log.Println("Delete ip rule and route for overlay.")
	resetOverlayIPRule("40")
	resetOverlayController()
	time.Sleep(1000)
	resetDataplane()
	log.Println("Successfully reset cluster role")
}

func resetOverlayController() {
	overlayWorkingDir := filepath.Join(sasectlConf.ICNSdewanFilePath, "central-controller/deployments/kubernetes")

	cmdList := []utils.CmdInfo{
		{CmdName: "kubectl", CmdArgs: []string{"delete", "-f", "scc.yaml", "-n", "sdewan-system"}, CmdDir: overlayWorkingDir},
		{CmdName: "kubectl", CmdArgs: []string{"delete", "-f", "scc_secret.yaml", "-n", "sdewan-system"}, CmdDir: overlayWorkingDir},
		{CmdName: "kubectl", CmdArgs: []string{"delete", "-f", "scc_rsync.yaml", "-n", "sdewan-system"}, CmdDir: overlayWorkingDir},
		{CmdName: "kubectl", CmdArgs: []string{"delete", "-f", "scc_etcd.yaml", "-n", "sdewan-system"}, CmdDir: overlayWorkingDir},
		{CmdName: "kubectl", CmdArgs: []string{"delete", "-f", "scc_mongo.yaml", "-n", "sdewan-system"}, CmdDir: overlayWorkingDir},
	}
	for _, item := range cmdList {
		cmd := exec.Command(item.CmdName, item.CmdArgs...)
		cmd.Dir = item.CmdDir
		output, err := cmd.CombinedOutput()
		log.Println(string(output))
		if err != nil {
			log.Fatal(err)
			return
		}
	}
}

func resetDataplane() {

	helmWorkingDir := filepath.Join(sasectlConf.ICNSdewanFilePath, "platform/deployment/helm")
	cnfValueFp := filepath.Join(helmWorkingDir, "sdewan_cnf/values.yaml")
	cmFp := filepath.Join(helmWorkingDir, "sdewan_cnf/templates/cm.yaml")

	cmdList := []utils.CmdInfo{
		{CmdName: "helm", CmdArgs: []string{"uninstall", sasectlConf.ICNSdewanCNFChartName}},
		{CmdName: "helm", CmdArgs: []string{"uninstall", sasectlConf.ICNSdewanCtrlChartName}},
		{CmdName: "kubectl", CmdArgs: []string{"delete", "-f", "cnf_cert.yaml"}, CmdDir: filepath.Join(sasectlConf.ICNSdewanFilePath, "platform/deployment/helm/cert")},
		{CmdName: "kubectl", CmdArgs: []string{"delete", "-f", "default-networks.yaml"}, CmdDir: sasectlConf.ICNSdewanFilePath},
		{CmdName: "kubectl", CmdArgs: []string{"delete", "-f", "multus-cr.yaml"}, CmdDir: sasectlConf.ICNSdewanFilePath},
		{CmdName: "kubectl", CmdArgs: []string{"delete", "-f", "namespace.yaml"}, CmdDir: sasectlConf.ICNSdewanFilePath},
	}
	for _, item := range cmdList {
		cmd := exec.Command(item.CmdName, item.CmdArgs...)
		cmd.Dir = item.CmdDir
		output, err := cmd.CombinedOutput()
		log.Println(string(output))
		if err != nil {
			log.Fatal(err)
			return
		}
	}
	// Reset values.yaml for sdewan_cnf
	cnfValue := utils.LoadCNFValueFile(cnfValueFp)
	utils.ResetCNFValueNFN(cnfValue)
	utils.UpdateCNFValueFile(cnfValueFp, cnfValue)
	// Reset cm.yaml for sdewan_cnf
	utils.GenerateCMYaml(cmFp, "reset")

	utils.SetClusterRole(configFP, "", sasectlConf)
}

func resetOverlayIPRule(tableID string) {
	listTableIDCmd := `ip route show table all | grep "table" | sed 's/.*\(table.*\)/\1/g' | awk '{print $2}' | sort | uniq`
	ipRuleCmd := exec.Command("ip", "rule")
	ipRouteShowCmd := exec.Command("bash", "-c", listTableIDCmd)
	ipRouteFlushCmd := exec.Command("sudo", "ip", "route", "flush", "table", tableID)

	ruleOutput, err := ipRuleCmd.CombinedOutput()
	if err != nil {
		log.Print(string(ruleOutput))
		log.Fatal(err)
	}

	ruleList := strings.Split(string(ruleOutput), "\n")
	var prioList []string

	// Find all lookup tableID rules.
	for _, v := range ruleList {
		items := strings.Fields(v)
		if len(items) > 1 && items[len(items)-2] == "lookup" && items[len(items)-1] == tableID {
			prio := strings.Trim(items[0], ":")
			prioList = append(prioList, prio)
		}
	}

	// Clean all lookup tableID rules if existed.
	if len(prioList) > 0 {
		for _, prio := range prioList {
			delRuleCmd := exec.Command("sudo", "ip", "rule", "del", "prio", prio)
			delRuleOut, err := delRuleCmd.CombinedOutput()
			if err != nil {
				log.Print(string(delRuleOut))
				log.Fatal(err)
			}
		}
	}

	ipRouteTableList, err := ipRouteShowCmd.CombinedOutput()
	if err != nil {
		log.Print(string(ipRouteTableList))
		log.Fatal(err)
	}
	tableList := strings.Split(string(ipRouteTableList), "\n")

	if len(tableList) > 0 {
		for _, tid := range tableList {
			if tid == tableID {
				ipFlushOut, err := ipRouteFlushCmd.CombinedOutput()
				if err != nil {
					log.Println(string(ipFlushOut))
					log.Fatal(err)
				}
			}
		}
	}
}

func edgeCleanIpsecCRsApiServer() {
	// Clean ipsechost info.
	checkIpsecHostCmd := exec.Command("kubectl", "get", IpsecHosts, "--no-headers", "-A", "-o", "custom-columns=Name:.metadata.name")
	checkIpsecProposalCmd := exec.Command("kubectl", "get", IpsecProposals, "--no-headers", "-A", "-o", "custom-columns=Name:.metadata.name")

	ipsecHostOutput, err := checkIpsecHostCmd.CombinedOutput()
	if err != nil {
		log.Print(string(ipsecHostOutput))
		log.Print("Failed to get custom resources of type " + IpsecHosts)
		log.Fatal(err)
	}

	if len(ipsecHostOutput) > 0 {
		ipsecHostList := strings.Split(string(ipsecHostOutput), "\n")
		for _, ipsecHost := range ipsecHostList {
			if len(ipsecHost) > 0 {
				delIpsecHostCmd := exec.Command("kubectl", "delete", IpsecHosts, ipsecHost)
				delIpsecHostOutput, err := delIpsecHostCmd.CombinedOutput()
				if err != nil {
					log.Print(string(delIpsecHostOutput))
					log.Print("Failed to delete custom resource of type " + IpsecHosts)
					log.Fatal(err)
				}
				log.Println(string(delIpsecHostOutput))
			}
		}
	}

	ipsecProposalOutput, err := checkIpsecProposalCmd.CombinedOutput()
	if err != nil {
		log.Print(string(ipsecProposalOutput))
		log.Print("Failed to get custom resources of type " + IpsecHosts)
		log.Fatal(err)
	}

	if len(ipsecProposalOutput) > 0 {
		ipsecProposalList := strings.Split(string(ipsecProposalOutput), "\n")
		for _, ipsecProposal := range ipsecProposalList {
			if len(ipsecProposal) > 0 {
				delIpsecProposalCmd := exec.Command("kubectl", "delete", IpsecProposals, ipsecProposal)
				delIpsecProposalOutput, err := delIpsecProposalCmd.CombinedOutput()
				if err != nil {
					log.Print(string(delIpsecProposalOutput))
					log.Print("Failed to delete custom resources of type " + IpsecHosts)
					log.Fatal(err)
				}
				log.Println(string(delIpsecProposalOutput))
			}
		}
	}
}

func overlayCleanIpsecCRsRest() {
	//Clean Connections
	serverIP := utils.CheckPodIP("scc")

	// overlays := []string{"overlay1"}

	overlays, err := queryOverlays(serverIP)

	if err != nil {
		log.Println("Failed to query overlay info.")
	}

	for _, overlay := range overlays {
		// Delete registed connections and hubs.
		o := overlay.GetMetadata().Name
		hubs, err := queryHubs(serverIP, o)
		if err != nil {
			log.Printf("Failed to query pops info of %s from overlay controller.", o)
			log.Println(err)
		}
		for _, v := range hubs {
			hubName := v.GetMetadata().Name
			cons, err := queryConnections(serverIP, o, hubName)
			if err != nil {
				log.Printf("Failed to query connections from pop %s.", hubName)
				log.Println(err)
			}
			for _, c := range cons {
				conName := c.GetMetadata().Name
				err = deregOverlayCon(o, conName, hubName)
				if err != nil {
					log.Printf("Failed to delete connections %s from pop %s overlay %s.", conName, hubName, o)
				}
			}
			err = deregHub(serverIP, o, hubName)
			if err != nil {
				log.Printf("Failed to delete pop %s from overlay %s.", hubName, o)
			}
		}
		// Delete registed devices.
		devs, err := queryDevs(serverIP, o)
		if err != nil {
			log.Printf("Fatiled to query devices info of %s from overlay controller.", o)
			log.Println(err)
		}
		for _, v := range devs {
			deviceName := v.GetMetadata().Name
			err = deregDevice(serverIP, o, deviceName)
			if err != nil {
				log.Printf("Failed to delete edge device %s of overlay %s.", deviceName, o)
			}
		}

		// Delete IPRanges
		ipranges, err := queryIPranges(serverIP, o)
		if err != nil {
			log.Printf("Fatiled to query IPRange info of %s from overlay controller.", o)
			log.Println(err)
		}
		for _, v := range ipranges {
			iprangeName := v.GetMetadata().Name
			err = deregIPRange(serverIP, o, iprangeName)
			if err != nil {
				log.Printf("Failed to delete IPRange %s of overlay %s.", iprangeName, o)
			}
		}

		// Delete proposals
		proposals, err := queryProposals(serverIP, o)
		if err != nil {
			log.Printf("Fatiled to query Proposal info of %s from overlay controller.", o)
			log.Println(err)
		}
		for _, v := range proposals {
			proposalName := v.GetMetadata().Name
			err = deregProposal(serverIP, o, proposalName)
			if err != nil {
				log.Printf("Failed to delete proposal %s of overlay %s.", proposalName, o)
			}
		}

		//Delete Overlay
		err = deregOverlay(serverIP, o)
		if err != nil {
			log.Printf("Failed to delete Overlay %s.", o)
		}
	}

	providerIPranges, err := queryIPranges(serverIP, "")
	if err != nil {
		log.Println("Fatiled to query IPRange info from overlay controller.")
		log.Println(err)
	}
	for _, proIpr := range providerIPranges {
		iprName := proIpr.GetMetadata().Name
		err = deregIPRange(serverIP, "", iprName)
		if err != nil {
			log.Printf("Failed to delete provider iprange %s.", iprName)
		}
	}
}
