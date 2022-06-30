/**
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
**/
package cmd

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sasectl/utils"
	"strings"

	"github.com/akraino-edge-stack/icn-sdwan/central-controller/src/scc/pkg/module"
	"github.com/akraino-edge-stack/icn-sdwan/central-controller/src/scc/pkg/resource"
	"github.com/spf13/cobra"
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register operation for SASE-EK cluster",
	// Run: func(cmd *cobra.Command, args []string) {
	// 	fmt.Println("register called")
	// },
}

var registerEdgeCmd = &cobra.Command{
	Use:   "edge",
	Short: "Register operation for SASE-EK Edge cluster",
	// Run: func(cmd *cobra.Command, args []string) {
	// },
}

var edgeRegToControllerCmd = &cobra.Command{
	Use:   "toController",
	Short: "Register edge cluster to Overlay controller",
	Run: func(cmd *cobra.Command, args []string) {
		configFp, err := cmd.Flags().GetString("file")
		if err != nil {
			log.Fatal(err)
		}
		certFp, err := cmd.Flags().GetString("ca")
		if err != nil {
			log.Fatal(err)
		}

		regEdgeToOverlay(configFp, certFp)
	},
}

var registerOverlayCmd = &cobra.Command{
	Use:   "overlay",
	Short: "Register operation for overlay cluster",
	// Run: func(cmd *cobra.Command, args []string) {
	// },
}

var overlayPreRegCmd = &cobra.Command{
	Use:   "preReg",
	Short: "Pre-register overlay and devices",
	Run: func(cmd *cobra.Command, args []string) {
		providerIPrange, err := cmd.Flags().GetString("providerIPrange")
		if err != nil {
			log.Fatal(err)
		}
		dataIPrange, err := cmd.Flags().GetString("dataIPrange")
		if err != nil {
			log.Fatal(err)
		}

		regOverlayPreReg(providerIPrange, dataIPrange)
	},
}

var overlayRegDevCmd = &cobra.Command{
	Use:   "regDev",
	Short: "Register device on overlay",
	Run: func(cmd *cobra.Command, args []string) {
		devName, err := cmd.Flags().GetString("name")
		if err != nil {
			log.Fatal(err)
		}

		configFp, err := cmd.Flags().GetString("file")
		if err != nil {
			log.Fatal(err)
		}
		regOverlayRegDev(configFp, "overlay1", devName)
	},
}

var overlayRegConCmd = &cobra.Command{
	Use:   "regCon",
	Short: "Register connections between edges",
	Run: func(cmd *cobra.Command, args []string) {
		overlay, err := cmd.Flags().GetString("overlay")
		if err != nil {
			log.Fatal(err)
		}

		deviceName, err := cmd.Flags().GetString("device")
		if err != nil {
			log.Fatal(err)
		}

		popName, err := cmd.Flags().GetString("pop")
		if err != nil {
			log.Fatal(err)
		}

		regOverlayCon(overlay, deviceName, popName)
	},
}

func init() {
	registerEdgeCmd.AddCommand(edgeRegToControllerCmd)

	edgeRegToControllerCmd.Flags().StringP("file", "f", "", "Register info file getting from overlay controller")
	edgeRegToControllerCmd.MarkFlagRequired("file")
	edgeRegToControllerCmd.MarkFlagFilename("file")
	edgeRegToControllerCmd.Flags().StringP("ca", "c", "", "CA file getting from overlay controller")
	edgeRegToControllerCmd.MarkFlagRequired("ca")
	edgeRegToControllerCmd.MarkFlagFilename("ca")

	registerOverlayCmd.AddCommand(overlayPreRegCmd)
	registerOverlayCmd.AddCommand(overlayRegDevCmd)
	registerOverlayCmd.AddCommand(overlayRegConCmd)

	// Add flags to preReg cmd
	overlayPreRegCmd.Flags().StringP("providerIPrange", "p", "192.168.0.0", "providerIPrange")
	overlayPreRegCmd.Flags().StringP("dataIPrange", "d", "192.169.0.0", "dataIPrange")

	// Add flags to regDev cmd
	overlayRegDevCmd.Flags().StringP("file", "f", "", "Register info file export from sasectl init")
	overlayRegDevCmd.MarkFlagRequired("file")
	overlayRegDevCmd.MarkFlagFilename("file")
	overlayRegDevCmd.Flags().StringP("name", "n", "", "Device name to register in overlay controller")
	overlayRegDevCmd.MarkFlagRequired("name")

	// Add flags to regCon cmd
	overlayRegConCmd.Flags().StringP("overlay", "o", "", "Overlay network to setup connection.")
	overlayRegConCmd.MarkFlagRequired("name")
	overlayRegConCmd.Flags().StringP("device", "d", "", "Edge node to setup connection")
	overlayRegConCmd.MarkFlagRequired("device")
	overlayRegConCmd.Flags().StringP("pop", "p", "", "Pop node to setup connection")
	overlayRegConCmd.MarkFlagRequired("pop")

	registerCmd.AddCommand(registerEdgeCmd)
	registerCmd.AddCommand(registerOverlayCmd)
	rootCmd.AddCommand(registerCmd)
}

func regEdgeToOverlay(configFp string, certFp string) {
	safePodName := utils.CheckPodFullname("safe")
	caPem, err := ioutil.ReadFile(certFp)
	if err != nil {
		log.Println("Failed to read cert file.")
		log.Fatal(err)
	}
	block, _ := pem.Decode(caPem)
	if block == nil {
		log.Fatal("Invalid input PEM file.")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Fatal("Invalid input PEM file.")
	}

	if cert.Issuer.CommonName != "sdewan-controller" {
		log.Fatal("Invalid input PEM file.")
		return
	}

	copyCmd := "kubectl exec -n sdewan-system " + safePodName + " -- bash -c \"echo \\\"" + string(caPem) + "\\\" | sudo tee /etc/ipsec.d/cacerts/ca.pem\""

	output, err := exec.Command("bash", "-c", copyCmd).CombinedOutput()
	if err != nil {
		log.Println("Failed to copy cert file to CNF pods")
		log.Println(string(output))
		log.Fatal(err)
	}
	log.Print(string(output))
	regCmdInfo := utils.CmdInfo{
		CmdName: "kubectl",
		CmdArgs: []string{"apply", "-f", configFp},
	}
	regCmd := exec.Command(regCmdInfo.CmdName, regCmdInfo.CmdArgs...)
	output, err = regCmd.CombinedOutput()
	if err != nil {
		log.Println(string(output))
		log.Fatal(err)
	}
	log.Print(string(output))
}

func regOverlayPreReg(providerIPrange string, dataIPrange string) {
	serverIP := utils.CheckPodIP("scc")
	// 3 Nodes PreReg Con
	// TODO: Provide more general way.
	overlay := "overlay1"
	overlayProposal1 := "proposal1"
	overlayProposal2 := "proposal2"

	providerIPrangeName := "provideripr"

	dataIPRangeName := "dataipr"

	if sasectlConf.ICNSdewanRole == "popoverlay" {
		regCustomizeCombinedIptables()
	}

	regOverlay(serverIP, overlay)
	regProposal(serverIP, overlay, overlayProposal1)
	regProposal(serverIP, overlay, overlayProposal2)
	regIPRange(serverIP, "", providerIPrange, providerIPrangeName)
	regIPRange(serverIP, overlay, dataIPrange, dataIPRangeName)
	regConfigSCCDB()
	regCallRegCluster()
	regSetIPRule(providerIPrange, "40")

	// DEBUG Check pre reg result.
	overlayUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection
	overlayData, err := utils.CallRest("GET", overlayUrl, "")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(overlayData)

	proposalUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.ProposalCollection
	proposalData, err := utils.CallRest("GET", proposalUrl, "")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(proposalData)

	providerIpRangeUrl := "http://" + serverIP + ":9015/scc/v1/" + "provider" +
		"/" + utils.IPRangeCollection
	providerIPData, err := utils.CallRest("GET", providerIpRangeUrl, "")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(providerIPData)

	overlayIpRangeUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.IPRangeCollection
	overlayIPData, err := utils.CallRest("GET", overlayIpRangeUrl, "")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(overlayIPData)
}

func regOverlayRegDev(configFP string, overlay string, deviceName string) {
	serverIP := utils.CheckPodIP("scc")
	if serverIP == "" {
		log.Fatal("SCC Pod not found.")
		return
	}
	_, confFileName := filepath.Split(configFP)
	confInfo := strings.Split(confFileName, "-")
	// devApiServer := confInfo[0]
	if len(confInfo) <= 1 {
		log.Fatal("Illegal cluster config file found.")
		return
	}
	devType := confInfo[1]
	if devType == "edge" {
		regCert(serverIP, overlay, deviceName)
		regDevice(serverIP, overlay, deviceName, configFP)
		// TODO Add flag or something else to replace hard code
		exportEdgeIpsecInfo(serverIP, overlay, "10.10.70.49", deviceName)
	} else if devType == "pop" || devType == "popoverlay" {
		var publicIP []string
		// TODO Add flag or something else to replace hard code
		publicIP = append(publicIP, "10.10.70.39")
		regHub(serverIP, overlay, deviceName, configFP, publicIP)
	} else {
		log.Fatal("Illegal device type")
	}
	regExportCapem(deviceName)
}

func regOverlayCon(overlay string, deviceName string, hubName string) {
	serverIP := utils.CheckPodIP("scc")
	regConUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.HubCollection + "/" + hubName + "/" + utils.DeviceCollection

	conName := hubName + strings.Replace(deviceName, "-", "", -1) + "conn"

	regConReq := module.HubDeviceObject{
		Metadata: module.ObjectMetaData{conName, "", "", ""},
		Specification: module.HubDeviceObjectSpec{
			Device:        deviceName,
			IsDelegateHub: true,
		},
	}
	_, err := createControllerObject(regConUrl, &regConReq, &module.HubDeviceObject{})
	if err != nil {
		log.Print("Failed to create controller object")
	}
}

func createControllerObject(baseUrl string, obj module.ControllerObject, retObj module.ControllerObject) (module.ControllerObject, error) {
	url := baseUrl
	obj_str, _ := json.Marshal(obj)

	res, err := utils.CallRest("POST", url, string(obj_str))
	if err != nil {
		log.Println(err.Error())
		return retObj, err
	}

	err = json.Unmarshal([]byte(res), retObj)
	if err != nil {
		return retObj, err
	}

	return retObj, nil
}

func regOverlay(serverIP string, overlay string) {
	OverlayUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection
	overlayObj := module.OverlayObject{
		Metadata:      module.ObjectMetaData{overlay, "", "", ""},
		Specification: module.OverlayObjectSpec{}}

	_, err := createControllerObject(OverlayUrl, &overlayObj, &module.OverlayObject{})
	if err != nil {
		log.Print("Failed to create controller object")
	}
}

func regProposal(serverIP string, overlay string, proposal string) {
	ProposalUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.ProposalCollection
	proposalObj := module.ProposalObject{
		Metadata:      module.ObjectMetaData{proposal, "", "", ""},
		Specification: module.ProposalObjectSpec{"aes128", "sha256", "modp3072"}}

	_, err := createControllerObject(ProposalUrl, &proposalObj, &module.ProposalObject{})
	if err != nil {
		log.Print("Failed to create controller object")
	}
}

func regDevice(serverIP string, overlay string, deviceName string, deviceConfigFp string) {
	deviceConfig, err := ioutil.ReadFile(deviceConfigFp)
	if err != nil {
		log.Fatal("Failed to open device config file.")
		log.Fatal(err)
	}

	DeviceUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.DeviceCollection
	encodedDevConf := base64.StdEncoding.EncodeToString([]byte(deviceConfig))
	certName := "device-" + deviceName + "-cert"
	deviceObj := module.DeviceObject{
		Metadata:      module.ObjectMetaData{deviceName, "", "", ""},
		Specification: module.DeviceObjectSpec{[]string{}, true, "", 65536, true, false, certName, encodedDevConf}}

	_, err = createControllerObject(DeviceUrl, &deviceObj, &module.DeviceObject{})
	if err != nil {
		log.Print("Failed to create controller object")
	}
}

func regHub(serverIP string, overlay string, hubName string, hubConfigFp string, hubPublicIp []string) {
	hubConfig, err := ioutil.ReadFile(hubConfigFp)
	if err != nil {
		log.Fatal("Failed to open hub config file.")
		log.Fatal(err)
	}

	HubUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.HubCollection
	encodedHubConf := base64.StdEncoding.EncodeToString([]byte(hubConfig))
	hubCertId := "CN=hub-" + hubName + "-cert"
	hubObj := module.HubObject{
		Metadata:      module.ObjectMetaData{hubName, "", "", ""},
		Specification: module.HubObjectSpec{hubPublicIp, hubCertId, encodedHubConf}}

	_, err = createControllerObject(HubUrl, &hubObj, &module.HubObject{})
	if err != nil {
		log.Print("Failed to create controller object")
	}
}

func regIPRange(serverIP string, overlay string, ipRange string, ipRangeName string) {
	var ipRangeUrl string
	if overlay != "" {
		ipRangeUrl = "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
			"/" + overlay + "/" + utils.IPRangeCollection
	} else {
		ipRangeUrl = "http://" + serverIP + ":9015/scc/v1/provider/" + utils.IPRangeCollection
	}

	iprangeObj := module.IPRangeObject{
		Metadata:      module.ObjectMetaData{ipRangeName, "", "", ""},
		Specification: module.IPRangeObjectSpec{ipRange, 1, 25}}

	_, err := createControllerObject(ipRangeUrl, &iprangeObj, &module.IPRangeObject{})
	if err != nil {
		log.Print("Failed to create controller object")
	}
}

func regCert(serverIP string, overlay string, deviceName string) {
	CertUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.CertCollection
	certObj := module.CertificateObject{
		Metadata: module.ObjectMetaData{deviceName, "", "", ""}}

	_, err := createControllerObject(CertUrl, &certObj, &module.CertificateObject{})
	if err != nil {
		log.Print("Failed to create controller object")
	}
}

func exportEdgeIpsecInfo(serverIP string, overlay string, overlayIP string, deviceName string) {
	var proposalObjs []utils.ICNSdewanProposalObject
	var proposalResource resource.ProposalResource
	var proposals []string

	var certs module.CertificateObject

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get current path.")
		log.Fatal(err)
	}
	outFileName := filepath.Join(cwd, deviceName+".yaml")

	f, err := os.Create(outFileName)
	if err != nil {
		log.Fatal(err)
	}
	f.WriteString("---\n")
	proposalUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.ProposalCollection
	checkProposal, err := utils.CallRest("GET", proposalUrl, "")
	if err != nil {
		log.Fatal(err)
	}
	json.Unmarshal([]byte(checkProposal), &proposalObjs)

	certUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.CertCollection + "/" + deviceName
	checkCerts, err := utils.CallRest("GET", certUrl, "")
	if err != nil {
		log.Fatal(err)
	}
	json.Unmarshal([]byte(checkCerts), &certs)

	deviceUrl := "http://" + serverIP + ":9015/scc/v1/" + utils.OverlayCollection +
		"/" + overlay + "/" + utils.DeviceCollection

	deviceData, err := utils.CallRest("GET", deviceUrl, "")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(deviceData)

	for _, item := range proposalObjs {
		proposals = append(proposals, item.Metadata.Name)
		proposalResource = resource.ProposalResource{
			Name:       item.Metadata.Name,
			Encryption: item.Specification.Encryption,
			Hash:       item.Specification.Hash,
			DhGroup:    item.Specification.DhGroup,
		}
		f.WriteString(proposalResource.ToYaml(deviceName))
		f.WriteString("\n\n---\n\n")
	}

	combinedRootCA, err := base64.StdEncoding.DecodeString(certs.Data.RootCA)
	if err != nil {
		log.Print("Failed to decode RootCA.")
		log.Fatal(err)
	}
	sCombinedRootCA := strings.Split(string(combinedRootCA), "\n")
	// Hard code to get last 19 line of certs.
	targetRootCA := strings.Join(sCombinedRootCA[len(sCombinedRootCA)-20:len(sCombinedRootCA)-1], "\n")
	targetEncodedRootCA := base64.StdEncoding.EncodeToString([]byte(targetRootCA))

	ipsecConName := "Conn" + strings.Replace(deviceName, "-", "", -1)
	ipsecResName := "localto" + strings.Replace(deviceName, "-", "", -1)
	ipsecCon := resource.Connection{
		ConnectionType: "tunnel",
		CryptoProposal: proposals,
		LocalUpDown:    "/usr/lib/ipsec/_updown iptables",
		Mode:           "start",
		Name:           ipsecConName,
		LocalSourceIp:  "%config",
	}
	ipsecRes := resource.IpsecResource{
		Name:                 ipsecResName,
		AuthenticationMethod: "pubkey",
		Connections:          ipsecCon,
		CryptoProposal:       proposals,
		ForceCryptoProposal:  "0",
		LocalIdentifier:      "CN=device-" + deviceName + "-cert",
		PrivateCert:          certs.Data.Key,
		PublicCert:           certs.Data.Ca,
		Remote:               overlayIP,
		RemoteIdentifier:     "CN=sdewan-controller-base",
		SharedCA:             targetEncodedRootCA,
	}

	f.WriteString(ipsecRes.ToYaml(deviceName))

	f.Sync()
}

func regConfigSCCDB() {
	configFP := filepath.Join(sasectlConf.ICNSdewanFilePath, "central-controller/src/reg_cluster/config.json")
	etcdIP := utils.CheckPodIP("etcd")
	mongoIP := utils.CheckPodIP("mongo")
	sccConf := utils.ICNSccDbConfig{
		EtcdIP: etcdIP,
		DBIP:   mongoIP,
	}
	sccConfD, err := json.Marshal(sccConf)
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile(configFP, sccConfD, 0664)
	if err != nil {
		log.Fatal("Failed to prepare database info for scc")
		log.Fatal(err)
	}
}

func regCallRegCluster() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Println("Failed to get user home dir.")
	}
	kubeConfigFp := filepath.Join(homeDir, ".kube/config")
	regClusterWD := filepath.Join(sasectlConf.ICNSdewanFilePath, "central-controller/src/reg_cluster")

	regClusterCmd := exec.Command("./reg_cluster", "-kubeconfigPath", kubeConfigFp)
	regClusterCmd.Dir = regClusterWD

	output, err := regClusterCmd.CombinedOutput()
	if err != nil {
		log.Println(string(output))
		log.Fatal(err)
	}
	log.Println(string(output))
}

func regSetIPRule(providerIPrange string, tableID string) {
	cnfIP := utils.CheckPodIP("safe")
	providerCIDR := providerIPrange + "/24"
	cnfIfName := utils.GetIPIfName(cnfIP)
	cmdList := []utils.CmdInfo{
		{CmdName: "sudo", CmdArgs: []string{"ip", "rule", "add", "to", providerCIDR, "lookup", tableID}},
		{CmdName: "sudo", CmdArgs: []string{"ip", "rule", "add", "to", "10.10.70.39/32", "lookup", tableID}},
		{CmdName: "sudo", CmdArgs: []string{"ip", "rule", "add", "to", "10.10.70.49/32", "lookup", tableID}},
		{CmdName: "sudo", CmdArgs: []string{"ip", "route", "add", "default", "via", cnfIP, "dev", cnfIfName, "table", tableID}},
	}
	for _, item := range cmdList {
		cmd := exec.Command(item.CmdName, item.CmdArgs...)
		output, err := cmd.CombinedOutput()
		// log.Println(output)
		if err != nil {
			log.Println(string(output))
			log.Fatal(err)
		}
	}
}

func regExportCapem(deviceName string) {
	cmd := exec.Command("kubectl", "get", "secrets", "-n", "sdewan-system", "sdewan-controller-cert-secret", "-o=jsonpath=\"{['data']['ca\\.crt']}\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	soutput := string(output)
	soutput = strings.Trim(soutput, "\"")
	data, err := base64.StdEncoding.DecodeString(soutput)
	if err != nil {
		log.Fatal("Failed to decode ca.crt.")
		log.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed to get current path.")
		log.Fatal(err)
	}
	outputFp := filepath.Join(cwd, deviceName+"ca.pem")

	err = ioutil.WriteFile(outputFp, []byte(data), 0644)
	if err != nil {
		log.Fatal("Failed to export ca.pem for device.")
		log.Fatal(err)
	}
}

func regCustomizeCombinedIptables() {
	popProviderIP := "10.10.70.39"
	safePodName := utils.CheckPodFullname("safe")
	iptableCmd := "sudo iptables -I PREROUTING -d " + popProviderIP + "/32 -p tcp -m tcp --dport 6443 -j DNAT --to-destination 10.96.0.1:443 -t nat "
	kubeIptableCmd := "kubectl exec -n sdewan-system " + safePodName + " -- " + iptableCmd

	output, err := exec.Command("bash", "-c", kubeIptableCmd).CombinedOutput()
	if err != nil {
		log.Println(string(output))
		log.Fatal(err)
	}
	log.Println(string(output))
}
