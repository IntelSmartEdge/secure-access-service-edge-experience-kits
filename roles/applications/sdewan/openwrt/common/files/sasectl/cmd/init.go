/**
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
**/
package cmd

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sasectl/utils"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize cluster role of SASE-EK",
	// PersistentPreRun: func(cmd *cobra.Command, args []string) {
	// 	if sasectlConf.ICNSdewanRole != "" {
	// 		log.Fatal("Cluster has already been initialized")
	// 	}
	// },
	// Run: func(cmd *cobra.Command, args []string) {
	// 	fmt.Println("init called")
	// },
}

var initEdgeCmd = &cobra.Command{
	Use:   "edge",
	Short: "Initialize cluster as edge cluster",
	Run: func(cmd *cobra.Command, args []string) {
		if sasectlConf.ICNSdewanRole != "" {
			log.Fatal("Cluster has already been initialized")
		}
		// edgePubIP := "192.168.0.1"
		providerIP, err := cmd.Flags().GetString("providerIP")
		if err != nil {
			log.Fatal("Failed to get provider ip for edge CNF.")
		}

		publicIP, err := cmd.Flags().GetString("publicIP")
		if err != nil {
			log.Fatal("Failed to get public ip for edge CNF.")
		}

		initEdgeCluster(publicIP, providerIP)
	},
}

var initPopCmd = &cobra.Command{
	Use:   "pop",
	Short: "Initialize cluster as pop cluster",
	Run: func(cmd *cobra.Command, args []string) {
		if sasectlConf.ICNSdewanRole != "" {
			log.Fatal("Cluster has already been initialized")
		}
		popPubIP := "10.10.70.39"
		initPopCluster(popPubIP)
	},
}

var initOverlayCmd = &cobra.Command{
	Use:   "overlay",
	Short: "Initialize cluster as overlay controller cluster",
	Run: func(cmd *cobra.Command, args []string) {
		if sasectlConf.ICNSdewanRole != "" {
			log.Fatal("Cluster has already been initialized")
			return
		}
		log.Println("Initialize cluster as Overlay")
		overlayProviderNfn := []*utils.ICNNfnConfig{
			{
				DefaultGateway: false,
				Interface:      "net2",
				IPAddress:      "10.10.70.49",
				Name:           "pnetwork",
				Separate:       ",",
				Namespace:      "sdewan-system",
			},
			{
				DefaultGateway: false,
				Interface:      "net0",
				IPAddress:      "172.16.70.49",
				Name:           "ovn-network",
				Separate:       "",
				Namespace:      "sdewan-system",
			},
		}
		initOverlayCluster(overlayProviderNfn, false)
	},
}

var initPopOverlayCmd = &cobra.Command{
	Use:   "popoverlay",
	Short: "Initialize cluster as pop & overlay controller cluster",
	Run: func(cmd *cobra.Command, args []string) {
		if sasectlConf.ICNSdewanRole != "" {
			log.Fatal("Cluster has already been initialized")
			return
		}
		log.Println("Initialize cluster as pop & overlay")
		overlayProviderNfn := []*utils.ICNNfnConfig{
			{
				DefaultGateway: false,
				Interface:      "net2",
				IPAddress:      "10.10.70.49",
				Name:           "pnetwork",
				Separate:       ",",
				Namespace:      "sdewan-system",
			},
			{
				DefaultGateway: false,
				Interface:      "net3",
				IPAddress:      "10.10.70.39",
				Name:           "pnetwork",
				Separate:       ",",
				Namespace:      "sdewan-system",
			},
			{
				DefaultGateway: false,
				Interface:      "net0",
				IPAddress:      "172.16.70.49",
				Name:           "ovn-network",
				Separate:       "",
				Namespace:      "sdewan-system",
			},
		}
		initOverlayCluster(overlayProviderNfn, true)
	},
}

func init() {

	initEdgeCmd.Flags().String("providerIP", "", "IP address for edge CNF provider network.")
	initEdgeCmd.MarkFlagRequired("providerIP")
	initEdgeCmd.Flags().String("publicIP", "", "Public ip address for edge CNF.")
	initEdgeCmd.MarkFlagRequired("publicIP")
	initCmd.AddCommand(initEdgeCmd)

	initCmd.AddCommand(initPopCmd)
	initCmd.AddCommand(initOverlayCmd)
	initCmd.AddCommand(initPopOverlayCmd)
	rootCmd.AddCommand(initCmd)
}

func initEdgeCluster(publicIP string, providerIP string) {
	log.Println("Initialize cluster as Edge")

	ovnNetIP := parseOVNIP(providerIP)

	edgeProviderNfn := []*utils.ICNNfnConfig{
		{
			DefaultGateway: false,
			Interface:      "net2",
			IPAddress:      providerIP,
			Name:           "pnetwork",
			Separate:       ",",
			Namespace:      "sdewan-system",
		},
		{
			DefaultGateway: false,
			Interface:      "net0",
			IPAddress:      ovnNetIP,
			Name:           "ovn-network",
			Separate:       "",
			Namespace:      "sdewan-system",
		},
	}
	initDataplane(edgeProviderNfn, publicIP, "edge")
	utils.SetClusterRole(configFP, "edge", sasectlConf)
	exportKubeConfig()
	log.Println("Successfully set cluster role as Edge")
}

func initPopCluster(publicIP string) {
	log.Println("Initialize cluster as pop")
	popProviderNfn := []*utils.ICNNfnConfig{
		{
			DefaultGateway: false,
			Interface:      "net2",
			IPAddress:      "10.10.70.39",
			Name:           "pnetwork",
			Separate:       ",",
			Namespace:      "sdewan-system",
		},
		{
			DefaultGateway: false,
			Interface:      "net0",
			IPAddress:      "172.16.70.39",
			Name:           "ovn-network",
			Separate:       "",
			Namespace:      "sdewan-system",
		},
	}
	initDataplane(popProviderNfn, publicIP, "pop")
	utils.SetClusterRole(configFP, "pop", sasectlConf)
	exportKubeConfig()
	log.Println("Successfully set cluster role as pop")
}

func initOverlayCluster(dataPlaneNfn []*utils.ICNNfnConfig, combined bool) {
	overlayWorkingDir := filepath.Join(sasectlConf.ICNSdewanFilePath, "central-controller/deployments/kubernetes")

	log.Println("Setting up data plane")

	var clusterRole string
	if combined {
		clusterRole = "popoverlay"
	} else {
		clusterRole = "overlay"
	}

	initDataplane(dataPlaneNfn, "10.10.70.49", clusterRole)
	log.Println("Setting up control plane")
	overlayCmdList := []utils.CmdInfo{
		{CmdName: "kubectl", CmdArgs: []string{"apply", "-f", "scc_mongo.yaml", "-n", "sdewan-system"}, CmdDir: overlayWorkingDir},
		{CmdName: "kubectl", CmdArgs: []string{"apply", "-f", "scc_etcd.yaml", "-n", "sdewan-system"}, CmdDir: overlayWorkingDir},
		{CmdName: "kubectl", CmdArgs: []string{"apply", "-f", "scc_rsync.yaml", "-n", "sdewan-system"}, CmdDir: overlayWorkingDir},
		{CmdName: "kubectl", CmdArgs: []string{"apply", "-f", "scc_secret.yaml", "-n", "sdewan-system"}, CmdDir: overlayWorkingDir},
		{CmdName: "kubectl", CmdArgs: []string{"apply", "-f", "scc.yaml", "-n", "sdewan-system"}, CmdDir: overlayWorkingDir},
	}
	for _, item := range overlayCmdList {
		cmd := exec.Command(item.CmdName, item.CmdArgs...)
		cmd.Dir = item.CmdDir
		output, err := cmd.CombinedOutput()
		log.Println(string(output))
		if err != nil {
			log.Fatal(err)
			return
		}
	}
	utils.SetClusterRole(configFP, clusterRole, sasectlConf)

	if combined {
		exportKubeConfig()
	}

	log.Println("Successfully set cluster role as " + clusterRole + ".")
}

func initDataplane(nfnSettings []*utils.ICNNfnConfig, publicIP string, clusterRole string) {
	helmWorkingDir := filepath.Join(sasectlConf.ICNSdewanFilePath, "platform/deployment/helm")
	certWorkingDir := filepath.Join(helmWorkingDir, "cert")
	cnfValueFp := filepath.Join(helmWorkingDir, "sdewan_cnf/values.yaml")
	cmFp := filepath.Join(helmWorkingDir, "sdewan_cnf/templates/cm.yaml")

	cnfValue := utils.LoadCNFValueFile(cnfValueFp)
	cnfValue["nfn"] = nfnSettings
	cnfValue["publicIpAddress"] = publicIP
	utils.UpdateCNFValueFile(cnfValueFp, cnfValue)

	utils.GenerateCMYaml(cmFp, clusterRole)

	cmdList := []utils.CmdInfo{
		// General Steps
		{CmdName: "kubectl", CmdArgs: []string{"apply", "-f", "namespace.yaml"}, CmdDir: sasectlConf.ICNSdewanFilePath},
		{CmdName: "kubectl", CmdArgs: []string{"apply", "-f", "multus-cr.yaml"}, CmdDir: sasectlConf.ICNSdewanFilePath},
		{CmdName: "kubectl", CmdArgs: []string{"apply", "-f", "default-networks.yaml"}, CmdDir: sasectlConf.ICNSdewanFilePath},
		{CmdName: "kubectl", CmdArgs: []string{"apply", "-f", "cnf_cert.yaml"}, CmdDir: certWorkingDir},
		{CmdName: "helm", CmdArgs: []string{"package", "sdewan_cnf"}, CmdDir: helmWorkingDir},
		{CmdName: "helm", CmdArgs: []string{"install", sasectlConf.ICNSdewanCNFChartName, "./cnf-0.1.0.tgz"}, CmdDir: helmWorkingDir},
		{CmdName: "helm", CmdArgs: []string{"install", sasectlConf.ICNSdewanCtrlChartName, "./controllers-0.1.0.tgz"}, CmdDir: helmWorkingDir},
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

func exportKubeConfig() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Failed to export kube config file.")
		log.Fatal(err)
	}
	kubeConfigFp := filepath.Join(homeDir, "/.kube/config")

	var outFile strings.Builder
	apiServerCmd := exec.Command("kubectl", "config", "view", "-o", "jsonpath='{range .clusters[*]}{.cluster.server}{\"\\n\"}{end}'")
	apiServerb, err := apiServerCmd.CombinedOutput()
	if err != nil {
		log.Fatal("Failed to export kube config file.")
		log.Fatal(err)
	}
	apiServer := string(apiServerb)[9:]
	apiServer = apiServer[:len(apiServer)-7]
	outFile.WriteString(apiServer)
	outFile.WriteString("-")
	outFile.WriteString(sasectlConf.ICNSdewanRole)

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to export kube config file.")
		log.Fatal(err)
	}
	outFp := filepath.Join(cwd, outFile.String())

	src, err := os.Open(kubeConfigFp)
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()
	dst, err := os.Create(outFp)
	if err != nil {
		log.Fatal(err)
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)

	if err != nil {
		log.Fatal(err)
	}
}

func parseOVNIP(providerIP string) string {
	ovnSubnet := "172.16.70.0/24"
	ipField := strings.Split(providerIP, ".")
	if len(ipField) != 4 {
		log.Panic("Invalid provider IP")
	}
	ipSuffix := ipField[len(ipField)-1]
	ovnPrefix := strings.Split(ovnSubnet, ".")
	ovnIP := strings.Join(ovnPrefix[:3], ".")
	ovnIP = ovnIP + "." + ipSuffix
	return ovnIP

}
