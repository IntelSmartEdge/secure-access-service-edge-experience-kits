/**
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation

 * Unit tests for the SR-IOV Detection Application using the testing
 * Capabilities of Go Lang.
 * The SR-IOV Detection Application detects the NIC's with specific capabilities on the
 * Linux that it runs on. So in order to test more than 90% of the code functionality
 * we needed to input in some tests hard coded data from a specific Linux: Ubuntu 20.04
 * Since, at each Linux the detection will have different results - some of the tests will fail
 * in Linux machine that is not the machine above.
 * There are 4 sections of unit tests:
 * 1) TestMain Function - initializes pointers to prerequisites packages.
 * 2) Test functions that suppose to pass successfully in every Linux.
 * 3) Test functions that will pass succesfully ONLY in the Linux machine above.
 * 4) Test of the main function of the application.
 *
 *  @author Eyal Belkin
 *  @version 1.2 Apr/07/2022
 *
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/safchain/ethtool"
)

/*****************************************************************************************
 *                   Global Variables
 ****************************************************************************************/
var NETWORK_INTERFACE_PROPER_TO_SRIOV string = "eth2"              //From Ubuntu 20.04
var NETWORK_INTERFACE_WITH_PTP_MASTER_CONNECTION string = "ens5f0" //From Ubuntu 20.04
var NIC_BUS_INFO string = "0000:cc:00.1"                           //Bus info of the NIC eth0 from Ubuntu 20.04
const NETWORK_INTERFACE_WITH_VALID_IPV4 string = "eno8303"         //From Ubuntu 20.04
const NETWORK_INTERFACE_WITH_INVALID_LINK_STATE string = "tunl0"   //From Ubuntu 20.04
const NETWORK_INTERFACE_NOT_SUPPORTING_SRIOV string = "eth1"       //Has VF's = 0 in Ubuntu 20.04
const NETWORK_BUS_INFO_OF_NON_INTEL_NIC string = "e3:00.0"         //From Ubuntu 20.04

var REGULAR_DEVICES_SLICE []SupDevice = make([]SupDevice, 8)
var PTP_DEVICES_SLICE []SupDevice = make([]SupDevice, 3)
var RRU_DEVICES_SLICE []SupDevice = make([]SupDevice, 3)
var CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY map[string]*CriteriaList = make(map[string]*CriteriaList)

var ptrEthToolHandle *ethtool.Ethtool = nil
var ptrPci *ghw.PCIInfo = nil
var networkInterfaces []net.Interface = nil

/*****************************************************************************************
 *                   Global Intitialize Functions
 ****************************************************************************************/
func IntitializeDevicesSlices() {
	//First making sure to clean the existing values but keeping the allocated memory
	REGULAR_DEVICES_SLICE = REGULAR_DEVICES_SLICE[:0]
	PTP_DEVICES_SLICE = PTP_DEVICES_SLICE[:0]
	RRU_DEVICES_SLICE = RRU_DEVICES_SLICE[:0]

	REGULAR_DEVICES_SLICE = []SupDevice{
		{"eth0", "0000:cc:00.0", "0d58", "i40e", 40000, 64, 1, 1, "", false, false, false, 0, "0000:cc:00.x", false, false},
		{"eth3", "0000:b1:00.0", "0d58", "i40e", 40000, 64, 1, 1, "", false, false, false, 0, "0000:b1:00.x", false, false},
		{"eth4", "0000:31:00.0", "159b", "ice", 65535, 128, 0, 0, "", false, false, false, 0, "0000:31:00.x", false, false},
		{"eth6", "0000:b1:00.1", "159b", "ice", 25000, 128, 0, 1, "", false, false, false, 1, "0000:b1:00.x", false, false},
		{"eth10", "0000:ee:00.0", "159b", "ice", 25000, 128, 0, 1, "", false, false, false, 0, "0000:ee:00.x", false, false},
		{"eth11", "0000:ee:00.1", "159b", "ice", 25000, 128, 0, 1, "", false, false, false, 1, "0000:ee:00.x", false, false},
		{"eth12", "0000:ff:00.0", "159b", "ice", 25000, 128, 0, 1, "", false, false, false, 0, "0000:ff:00.x", false, false},
		{"eth13", "0000:ff:00.1", "159b", "ice", 25000, 128, 0, 1, "", false, false, false, 1, "0000:ff:00.x", false, false},
		{"eth14", "0000:gg:00.0", "159b", "ice", 25000, 64, 0, 1, "", false, false, false, 0, "0000:gg:00.x", false, false},
		{"eth15", "0000:gg:00.1", "159b", "ice", 25000, 63, 0, 1, "", false, false, false, 1, "0000:gg:00.x", false, false},
	}

	PTP_DEVICES_SLICE = []SupDevice{
		{"eth1", "0000:cc:00.1", "0d58", "i40e", 40000, 128, 1, 1, "", false, true, false, 1, "0000:cc:00.x", false, false},
		{"eth7", "0000:dd:00.0", "0d58", "i40e", 40000, 64, 1, 1, "", false, true, false, 0, "0000:dd:00.x", false, false},
		{"eth9", "0000:ii:00.0", "0d58", "i40e", 40000, 64, 1, 1, "", false, true, false, 0, "0000:ii:00.x", false, false},
		{"eth16", "0000:ii:00.1", "0d58", "i40e", 40000, 64, 1, 1, "", false, true, false, 1, "0000:ii:00.x", false, false},
	}

	RRU_DEVICES_SLICE = []SupDevice{
		{"eth2", "0000:dd:00.1", "0d58", "i40e", 40000, 64, 1, 1, "", false, false, true, 1, "0000:dd:00.x", false, false},
		{"eth5", "0000:31:00.1", "159b", "ice", 65535, 128, 0, 0, "", false, false, true, 1, "0000:31:00.x", false, false},
		{"eth8", "0000:hh:00.0", "0d58", "i40e", 40000, 64, 1, 1, "", false, false, true, 0, "0000:hh:00.x", false, false},
		{"eth17", "0000:hh:00.1", "0d58", "i40e", 40000, 64, 1, 1, "", false, false, true, 1, "0000:hh:00.x", false, false},
	}
}

/******************************************************************************************************************/
func InitializeCardAffinityMaps() {
	//First making sure to clean the existing values
	for cardAffinity := range CardAffinityMap {
		delete(CardAffinityMap, cardAffinity)
	}
	for cardAffinity := range PtpCardAffinityMap {
		delete(PtpCardAffinityMap, cardAffinity)
	}
	for cardAffinity := range RruCardAffinityMap {
		delete(RruCardAffinityMap, cardAffinity)
	}
	//Initialize the CardAffinityMap
	firstCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{0}, false}
	CardAffinityMap["0000:cc:00.x"] = &firstCardAffinityDevicesInfo
	thirdCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{2}, false}
	CardAffinityMap["0000:31:00.x"] = &thirdCardAffinityDevicesInfo
	fourthCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{1, 3}, false}
	CardAffinityMap["0000:b1:00.x"] = &fourthCardAffinityDevicesInfo
	fifthCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{4, 5}, false}
	CardAffinityMap["0000:ee:00.x"] = &fifthCardAffinityDevicesInfo
	fixthCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{6, 7}, false}
	CardAffinityMap["0000:ff:00.x"] = &fixthCardAffinityDevicesInfo
	seventhCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{8, 9}, false}
	CardAffinityMap["0000:gg:00.x"] = &seventhCardAffinityDevicesInfo

	//Initialize the PtpCardAffinityMap
	firstPtpCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{2, 3}, false}
	PtpCardAffinityMap["0000:ii:00.x"] = &firstPtpCardAffinityDevicesInfo
	SecondPtpCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{0}, false}
	PtpCardAffinityMap["0000:cc:00.x"] = &SecondPtpCardAffinityDevicesInfo
	thirdPtpCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{1}, false}
	PtpCardAffinityMap["0000:dd:00.x"] = &thirdPtpCardAffinityDevicesInfo

	//Initialize the RruCardAffinityMap
	firstRruCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{2, 3}, false}
	RruCardAffinityMap["0000:hh:00.x"] = &firstRruCardAffinityDevicesInfo
	secondRruCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{0}, false}
	RruCardAffinityMap["0000:dd:00.x"] = &secondRruCardAffinityDevicesInfo
	thirdRruCardAffinityDevicesInfo := CardAffinityDevicesInfo{[]uint8{1}, false}
	RruCardAffinityMap["0000:31:00.x"] = &thirdRruCardAffinityDevicesInfo
}

/******************************************************************************************************************/
func InitializeCriteriaListsMap() {
	//First making sure to clean the existing values
	for pf := range CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY {
		delete(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY, pf)
	}

	firstCriteriaList := CriteriaList{"APP", []string{"158a", "0d58", "1593", "159b", "1592", "188a"}, nil, 0, 100,
		2, -1, nil, false, false, false, -1, "", "", ""}
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"] = &firstCriteriaList

	secondCriteriaList := CriteriaList{"APP", []string{"158a", "0d58", "1593", "159b", "1592", "188a"}, nil, 0, 100,
		2, -1, nil, false, false, false, -1, "", "", ""}
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"] = &secondCriteriaList

	thirdCriteriaList := CriteriaList{"APP", []string{"158a", "0d58", "1593", "159b", "1592", "188a"}, nil, 0, 64,
		2, -1, nil, false, false, false, -1, "pf6", "", ""}
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"] = &thirdCriteriaList

	fourthCriteriaList := CriteriaList{"APP", []string{"158a", "0d58", "1593", "159b", "1592", "188a"}, nil, 0, 64,
		2, -1, nil, false, false, false, -1, "", "", ""}
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf4"] = &fourthCriteriaList

	fifthCriteriaList := CriteriaList{"APP", []string{"158a", "0d58", "1593", "159b", "1592", "188a"}, nil, 0, 64,
		2, -1, nil, false, false, false, -1, "", "", ""}
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf5"] = &fifthCriteriaList

	sixthCriteriaList := CriteriaList{"APP", []string{"158a", "0d58", "1593", "159b", "1592", "188a"}, nil, 0, 64,
		2, -1, nil, false, false, false, -1, "pf3", "", ""}
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"] = &sixthCriteriaList

	seventhCriteriaList := CriteriaList{"APP", []string{"158a", "0d58", "1593", "159b", "1592", "188a"}, nil, 0, 64,
		2, -1, nil, false, false, false, -1, "pf8", "", ""}
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"] = &seventhCriteriaList

	eighthCriteriaList := CriteriaList{"APP", []string{"158a", "0d58", "1593", "159b", "1592", "188a"}, nil, 0, 64,
		2, -1, nil, false, false, false, -1, "pf7", "", ""}
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"] = &eighthCriteriaList
}

/*****************************************************************************************
 *                       TestMain  Function
 ****************************************************************************************/
//The TestMain Function will initialize pointers to 3 prerequisites packages:
// 1)net
// 2)Ethtool
// 3)GHW - PCI
// Those pointers are a MUST for the tests. So in case of failure we will not recover from panics
// like the application behaves.
// Also the TestMain will parse the YAML configuration file inorder to give option to the
// user to use each test as standalone with the values from the file.
func TestMain(m *testing.M) {
	fmt.Println("Initialize pointers to packages: EthTool, PCI and net")
	ptrEthToolHandle, ptrPci, networkInterfaces = InitializePackagesPointers()
	defer ptrEthToolHandle.Close()
	//Getting the criteria list and the configurations from the YAML configuration files
	criteriaListsMap, err := ParseConfigurationFile()
	if err != nil || len(criteriaListsMap) == 0 {
		log.Printf("The parsing of the configuration YAML file failed or the criteria lists is empty! Error: %v",
			err)
		panic("The parsing of the configuration YAML file failed or the criteria lists is empty!")
	}
	exitVal := m.Run()
	os.Exit(exitVal)
}

/*****************************************************************************************
 *                        TEST Functions
 ****************************************************************************************/
func Test_UpdateOutputString(t *testing.T) {
	//Saving the last value of the global variable
	temp := NUMBER_OF_DEVICES_REQUIRED
	//For this test: Using a requirement of 4 devices for SR-IOV
	NUMBER_OF_DEVICES_REQUIRED = 4
	//Using table tests pattern
	data := []struct {
		interfaceIndexToAdd  uint8
		deviceInterfaceToAdd string
		outputStringPtr      string
		expected             string
	}{
		{1, "eth0", "", "eth0\n"},
		{2, "eth1", "eth0\n", "eth0\neth1\n"},
		{3, "eth2", "eth0\neth1\n", "eth0\neth1\neth2\n"},
		{4, "eth3", "eth0\neth1\neth2\n", "eth0\neth1\neth2\neth3\n"},
		{5, "eth4", "eth0\neth1\neth2\neth3\n", ""},
	}
	for _, testLine := range data {
		t.Run(string(testLine.interfaceIndexToAdd), func(t *testing.T) {
			UpdateOutputString(testLine.interfaceIndexToAdd,
				testLine.deviceInterfaceToAdd,
				&testLine.outputStringPtr)
			if testLine.outputStringPtr != testLine.expected {
				t.Errorf("Expected %s, got %s", testLine.expected, testLine.outputStringPtr)
			}
		})
	}
	NUMBER_OF_DEVICES_REQUIRED = temp
}

/******************************************************************************************************************/
func Test_CheckDevicesAgainstCriteria(t *testing.T) {
	//Input NIC's slice for the CheckDevicesAgainstCriteria function:
	//In order to test every capability check against the criteria:
	//This slice contains NIC's with capabilities that match / does not match the criteria in a way that force the
	// function to check all the NIC's and all the capabilities.
	devicesInfoSlice := []SupDevice{
		{"eth0", "", "1593", "", 0, 64, LINK_STATE_INVALID_VALUE, INVALID_VALUE_FOR_NUMA_NODE, "", false,
			false, false, INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth1", "", "eyal", "driver2", 0, 64, LINK_STATE_INVALID_VALUE, INVALID_VALUE_FOR_NUMA_NODE,
			"", false, false, false, INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth2", "", "0d58", "driver1", 0, 128, LINK_STATE_INVALID_VALUE, INVALID_VALUE_FOR_NUMA_NODE,
			"", false, false, false, INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth3", "", "158a", "driver2", 40000, 256, LINK_STATE_INVALID_VALUE, INVALID_VALUE_FOR_NUMA_NODE,
			"", false, false, false, INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth4", "", "158a", "driver2", 40000, 60, LINK_STATE_INVALID_VALUE, INVALID_VALUE_FOR_NUMA_NODE,
			"", false, false, false, INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth5", "", "0d58", "driver2", 20000, 64, LINK_STATE_INVALID_VALUE, INVALID_VALUE_FOR_NUMA_NODE,
			"", false, false, false, INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth6", "", "0d58", "driver1", 40000, 64, LINK_STATE_INVALID_VALUE, INVALID_VALUE_FOR_NUMA_NODE,
			"", false, false, false, INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth7", "", "0d58", "driver2", 10000, 64, 1, 7, "", false, false, false,
			INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth8", "", "0d58", "driver2", 40000, 64, 0, 7, "", false, false, false,
			INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth9", "", "0d58", "driver2", 40000, 64, 1, 7, "", false, false, false,
			INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth10", "", "0d58", "driver2", 40000, 64, 1, 0, "Eyal", false, false, false,
			INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth11", "", "0d58", "driver2", 40000, 64, 1, 0, "SATA", false, false, false,
			INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth12", "", "0d58", "driver2", 40000, 64, 1, 0, "SATA", true, false, false,
			INVALID_VALUE_FOR_SPECIFIC_PORT, "", false, false},
		{"eth13", "", "0d58", "driver2", 40000, 64, 1, 0, "SATA", true, false, false,
			3, "0000:cc:00.x", false, false},
	}
	//Testing success of the function
	criteriaListsMap, err := ParseConfigurationFile()
	if err != nil || len(criteriaListsMap) == 0 {
		t.Errorf("Expected successful parsing of the YAML configuration file. got Error / empty output. Error: %v",
			err)
	}
	ptpDevicesSlice := []SupDevice{}
	rruDevicesSlice := []SupDevice{}
	outputResult := CheckDevicesAgainstCriteria(criteriaListsMap, devicesInfoSlice, ptpDevicesSlice, rruDevicesSlice)
	expectedOutputString := "eth0\neth2\neth3\neth5\n"
	if outputResult != expectedOutputString {
		t.Errorf("Expected output string: `%s`, got `%s`", expectedOutputString, outputResult)
	}
	//Testing another success of the function
	//Changing the flag of the proper devices to false in order to retest with the same devices slice
	devicesInfoSlice[0].CheckedAgainstCriteria = false
	devicesInfoSlice[2].CheckedAgainstCriteria = false
	devicesInfoSlice[3].CheckedAgainstCriteria = false
	devicesInfoSlice[5].CheckedAgainstCriteria = false
	pf5 := CriteriaList{"FH", []string{"158a", "0d58", "1593", "159b", "1592", "188a"}, []string{"driver2"},
		40000, 64, 1, 0, []string{"SATA"}, true, false, false, 3, "pf6", "0000:cc:00.x", ""}
	criteriaListsMap["pf5"] = &pf5
	//Adding 1 device to the global variable since we artificially added 1 device to the criteria list
	NUMBER_OF_DEVICES_REQUIRED++
	outputResult = CheckDevicesAgainstCriteria(criteriaListsMap, devicesInfoSlice, ptpDevicesSlice, rruDevicesSlice)
	expectedOutputString = "eth0\neth2\neth3\neth5\neth13\n" //Since the tested function succeeded
	if outputResult != expectedOutputString {
		t.Errorf("Expected output string: `%s`, got `%s`", expectedOutputString, outputResult)
	}
	//Testing edge case for the function: Invalid input
	outputResult = CheckDevicesAgainstCriteria(nil, nil, nil, nil)
	expectedOutputString = "" //Since the tested function failed
	if outputResult != expectedOutputString {
		t.Errorf("Expected failure with EMPTY output string after nil input, got `%s`", outputResult)
	}
	//Testing a scenario that there are more devices required in the criteriaListsMap
	//Than the slice of the detected devices
	//There is no need to Add 1 more device to the global variable for the next test: Thee devices slice was updated:
	//5 devices has CheckedAgainstCriteria = true, Hence there will be only 4 proper devices while the variable is 5
	outputResult = CheckDevicesAgainstCriteria(criteriaListsMap, devicesInfoSlice, ptpDevicesSlice, rruDevicesSlice)
	if outputResult != expectedOutputString {
		t.Errorf("Expected failure with EMPTY output string with: less proper devices then required, got `%s`",
			outputResult)
	}
	//Testing failure of CheckDevicesAgainstCriteria function with card affinity capability requirement
	USE_OF_CARD_AFFINITY_CAPABILITY = true
	pf6 := CriteriaList{"FH", []string{"158a", "0d58", "1593", "159b", "1592", "188a"}, []string{"driver2"},
		40000, 64, 1, 0, []string{"SATA"}, true, false, false, 3, "pf5", "0000:cc:00.x", ""}
	criteriaListsMap["pf6"] = &pf6
	//Adding 1 device to the global variable since we artificially added 1 more device to the criteria list
	NUMBER_OF_DEVICES_REQUIRED++
	outputResult = CheckDevicesAgainstCriteria(criteriaListsMap, devicesInfoSlice, ptpDevicesSlice, rruDevicesSlice)
	expectedOutputString = "" //Since the tested function failed
	if outputResult != expectedOutputString {
		t.Errorf("Expected FAILURE of CheckDevicesAgainstCriteria with card affinity and empty output string, got `%s`", outputResult)
	}
	USE_OF_CARD_AFFINITY_CAPABILITY = false //restore the global variable to false
	//restore the global variable to the valid number: Number of devices configured in the YAML file
	NUMBER_OF_DEVICES_REQUIRED--
	NUMBER_OF_DEVICES_REQUIRED--
}

/******************************************************************************************************************/
func Test_RemoveDeviceIndexFromSlice(t *testing.T) {
	//Adding device index to new card affinity in the map and removing it
	cardAffinity := "0000:cc:00.x"
	cardAffinityDevicesInfo := CardAffinityDevicesInfo{}
	cardAffinityDevicesInfo.DevicesIndexesSlice = append(cardAffinityDevicesInfo.DevicesIndexesSlice, 0)
	CardAffinityMap[cardAffinity] = &cardAffinityDevicesInfo
	RemoveDeviceIndexFromSlice(0, cardAffinity, CardAffinityMap)
}

/******************************************************************************************************************/
func Test_SelectingDeviceFromCardAffinityIndexesSlice(t *testing.T) {
	//Intialize all the data for the tests
	IntitializeDevicesSlices()
	InitializeCardAffinityMaps()
	InitializeCriteriaListsMap()
	//Testing failure of the function SelectingDeviceFromCardAffinityIndexesSlice: There is no true in the flag in
	//CardAffinityMap["0000:cc:00.x"]
	deviceIndexResult := SelectingDeviceFromCardAffinityIndexesSlice(CARD_AFFINITY_PF_COUPLE_TYPE_EMPTY_MARKED_TRUE,
		CardAffinityMap, "0000:cc:00.x", CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf1", REGULAR_DEVICES_SLICE)
	if deviceIndexResult > -1 {
		t.Errorf("Expected output Device Index Result of FAILURE: -1, got SUCCESS: %d", deviceIndexResult)
	}
	//Testing success of the function SelectingDeviceFromCardAffinityIndexesSlice
	deviceIndexResult = SelectingDeviceFromCardAffinityIndexesSlice(CARD_AFFINITY_PF_COUPLE_TYPE_EMPTY_MARKED_FALSE,
		CardAffinityMap, "0000:ee:00.x", CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf1", REGULAR_DEVICES_SLICE)
	if deviceIndexResult != 4 {
		t.Errorf("Expected output Device Index Result of 4 for empty card affinity, got %d", deviceIndexResult)
	}
	//Testing success of the function SelectingDeviceFromCardAffinityIndexesSlice
	deviceIndexResult = SelectingDeviceFromCardAffinityIndexesSlice(CARD_AFFINITY_PF_COUPLE_TYPE_EMPTY_MARKED_TRUE,
		CardAffinityMap, "0000:ee:00.x", CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf1", REGULAR_DEVICES_SLICE)
	if deviceIndexResult != 5 {
		t.Errorf("Expected output Device Index Result of 5 for empty card affinity, got %d", deviceIndexResult)
	}
	//Testing success of the function SelectingDeviceFromCardAffinityIndexesSlice
	deviceIndexResult = SelectingDeviceFromCardAffinityIndexesSlice(CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_SMALLER_THAN_COUPLED,
		CardAffinityMap, "0000:ff:00.x", CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf3", REGULAR_DEVICES_SLICE)
	if deviceIndexResult != 6 {
		t.Errorf("Expected output Device Index Result of 6 for current pf < coupled pf, got %d", deviceIndexResult)
	}
	//Testing success of the function SelectingDeviceFromCardAffinityIndexesSlice
	deviceIndexResult = SelectingDeviceFromCardAffinityIndexesSlice(CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_BIGGER_THAN_COUPLED,
		CardAffinityMap, "0000:ff:00.x", CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf6", REGULAR_DEVICES_SLICE)
	if deviceIndexResult != 7 {
		t.Errorf("Expected output Device Index Result of 7 for current pf > coupled pf, got %d", deviceIndexResult)
	}
	//Testing failure of the function SelectingDeviceFromCardAffinityIndexesSlice
	deviceIndexResult = SelectingDeviceFromCardAffinityIndexesSlice(CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_BIGGER_THAN_COUPLED,
		CardAffinityMap, "0000:cc:00.x", CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf1", REGULAR_DEVICES_SLICE)
	if deviceIndexResult >= 0 {
		t.Errorf("Expected FAILURE output of -1 Device Index Result, got SUCCESS: %d", deviceIndexResult)
	}
}

/******************************************************************************************************************/

func Test_CheckDevicesAgainstCriteriaListWithCardAffinity(t *testing.T) {
	//Intialize all the data for the tests
	IntitializeDevicesSlices()
	InitializeCardAffinityMaps()
	InitializeCriteriaListsMap()
	//Testing success of the function CheckDevicesAgainstCriteriaListWithCardAffinity with empty
	//card affinity and numVfs = 100
	deviceIndexResult := CheckDevicesAgainstCriteriaListWithCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf1", REGULAR_DEVICES_SLICE, CardAffinityMap)
	if deviceIndexResult < 0 {
		t.Errorf("Expected SUCCESS NON NEGATIVE output Device Index Result for empty card affinity and numVfs = 100 , got -1")
	}
	//Testing second success of the function CheckDevicesAgainstCriteriaListWithCardAffinity with empty
	//card affinity and numVfs = 100 --> in order to test another part of the function.
	deviceIndexResult = CheckDevicesAgainstCriteriaListWithCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf2", REGULAR_DEVICES_SLICE, CardAffinityMap)
	if deviceIndexResult < 0 {
		t.Errorf("Expected SUCCESS NON NEGATIVE output Device Index Result for empty card affinity and numVfs = 100, got -1")
	}
	//Testing success of the function CheckDevicesAgainstCriteriaListWithCardAffinity: current pf < coupled pf
	deviceIndexResult = CheckDevicesAgainstCriteriaListWithCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf3", REGULAR_DEVICES_SLICE, CardAffinityMap)
	if deviceIndexResult < 0 {
		t.Errorf("Expected SUCCESS NON NEGATIVE output Device Index Result for current pf < coupled pf, got -1")
	}
	//Testing success of the function CheckDevicesAgainstCriteriaListWithCardAffinity: current pf > coupled pf
	deviceIndexResult = CheckDevicesAgainstCriteriaListWithCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf6", REGULAR_DEVICES_SLICE, CardAffinityMap)
	if deviceIndexResult < 0 {
		t.Errorf("Expected SUCCESS NON NEGATIVE output Device Index Result for current pf > coupled pf, got -1")
	}
	//In order to fail the test we will high the criteria (too much for the devices) for NumVFSupp
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].NumVFSupp = 130
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].NumVFSupp = 130
	//Testing failure of the function CheckDevicesAgainstCriteriaListWithCardAffinity for current pf < coupled pf
	deviceIndexResult = CheckDevicesAgainstCriteriaListWithCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf7", REGULAR_DEVICES_SLICE, CardAffinityMap)
	if deviceIndexResult > -1 {
		t.Errorf("Expected FAILURE output Device Index Result of -1 for current pf < coupled pf, got SUCCESS %d", deviceIndexResult)
	}
	//Testing failure of the function CheckDevicesAgainstCriteriaListWithCardAffinity for current pf > coupled pf
	deviceIndexResult = CheckDevicesAgainstCriteriaListWithCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf8", REGULAR_DEVICES_SLICE, CardAffinityMap)
	if deviceIndexResult > -1 {
		t.Errorf("Expected FAILURE output Device Index Result of -1 for current pf > coupled pf, got SUCCESS %d", deviceIndexResult)
	}
	//Testing another failure of the function CheckDevicesAgainstCriteriaListWithCardAffinity for current pf > coupled pf
	//Before the failure testing we will have successful run of the function for current pf < coupled pf in order to test
	//to have valid card affinity and valid criteriaListsMap[currentPF].CardAffinity
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].NumVFSupp = 64
	//Successful run of CheckDevicesAgainstCriteriaListWithCardAffinity for current pf > coupled pf
	deviceIndexResult = CheckDevicesAgainstCriteriaListWithCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf7", REGULAR_DEVICES_SLICE, CardAffinityMap)
	if deviceIndexResult < 0 {
		t.Errorf("Expected SUCCESS output Device Index Result for current pf < coupled pf, got FAILURE: -1")
	}
	deviceIndexResult = CheckDevicesAgainstCriteriaListWithCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		"pf8", REGULAR_DEVICES_SLICE, CardAffinityMap)
	if deviceIndexResult > -1 {
		t.Errorf("Expected FAILURE output Device Index Result of -1 for current pf > coupled pf, got SUCCESS %d", deviceIndexResult)
	}
}

/******************************************************************************************************************/
func Test_DetectDevices(t *testing.T) {
	//Clear the card affinity maps before using the function
	for cardAffinity := range CardAffinityMap {
		delete(CardAffinityMap, cardAffinity)
	}
	for cardAffinity := range RruCardAffinityMap {
		delete(RruCardAffinityMap, cardAffinity)
	}
	for cardAffinity := range PtpCardAffinityMap {
		delete(PtpCardAffinityMap, cardAffinity)
	}
	//Testing success of DetectDevices function. Using the function with the card affinity requirements
	USE_OF_CARD_AFFINITY_CAPABILITY = true
	USE_OF_PTP_SUPPORT_CAPABILITY = true
	//Changing the value of the PTP timeout in order to get success for PTP detection
	previousValueOfPtpTimeout := TIMEOUT_FOR_FIND_PTP_MASTER
	TIMEOUT_FOR_FIND_PTP_MASTER = 21000
	regularDevicesSlice, ptpDevicesSlice, rruDevicesSlice := DetectDevices(ptrEthToolHandle, ptrPci, networkInterfaces)
	if regularDevicesSlice == nil || len(regularDevicesSlice) == 0 {
		fmt.Println("DetectDevices function failed to find proper interfaces slice in the Unit test!")
	} else {
		fmt.Printf("DetectDevices function found valid interfaces slice: %v\n", regularDevicesSlice)
		//In case of finding proper devices in this Linux we will use the first NIC as input to capabilities
		//detection functions
		NETWORK_INTERFACE_PROPER_TO_SRIOV = regularDevicesSlice[0].InterfaceName
		NIC_BUS_INFO = regularDevicesSlice[0].PciBus
		//After success of the function DetectDevices we will test the function PrintDataStructures
		DEBUG_MODE = true
		PrintDataStructures(regularDevicesSlice, ptpDevicesSlice, rruDevicesSlice)
		DEBUG_MODE = false
	}
	//After the success of DetectDevices function we will undo the card affinity changes of the function
	USE_OF_CARD_AFFINITY_CAPABILITY = false
	USE_OF_PTP_SUPPORT_CAPABILITY = false
	//Clear the card affinity maps before using the next test
	for cardAffinity := range CardAffinityMap {
		delete(CardAffinityMap, cardAffinity)
	}
	for cardAffinity := range PtpCardAffinityMap {
		delete(PtpCardAffinityMap, cardAffinity)
	}
	for cardAffinity := range RruCardAffinityMap {
		delete(RruCardAffinityMap, cardAffinity)
	}
	TIMEOUT_FOR_FIND_PTP_MASTER = previousValueOfPtpTimeout
}

/******************************************************************************************************************/
//Though this function uses hard coded values to test the function GetDeviceCardAffinity, It does not need any
//values from the Linux NIC's. In order to test GetDeviceCardAffinity we will need several bus info strings
//as input parameter. The NIC name is uses only for printing in the tested function.
//Meaning: This Unit test will work successfully in every Linux.
func Test_GetDeviceCardAffinity(t *testing.T) {
	//Testing success of GetDeviceCardAffinity function
	deviceCardAffinity := GetDeviceCardAffinity(NIC_BUS_INFO, NETWORK_INTERFACE_PROPER_TO_SRIOV)
	if deviceCardAffinity == "" {
		t.Errorf("Expected to find valid card affinity in the NIC '%s' with Bus info: %s - But it found empty one!",
			NETWORK_INTERFACE_PROPER_TO_SRIOV, NIC_BUS_INFO)
	}
	//Testing failure of GetDeviceCardAffinity function: Put invalid bus info string as input parameter.
	deviceCardAffinity = GetDeviceCardAffinity("Eyal", NETWORK_INTERFACE_PROPER_TO_SRIOV)
	if len(deviceCardAffinity) > 0 {
		t.Errorf("Expected to find INVALID VALUE FOR CARD AFFINITY ( empty string ) with INVALID bus info - But it found valid one!")
	}
	//Testing failure of GetDeviceCardAffinity function: Put bus info string with characters after the last '.[NUMBER]'
	//part (also invalid bus info).
	deviceCardAffinity = GetDeviceCardAffinity("0000:cc:00.1EYAL", NETWORK_INTERFACE_PROPER_TO_SRIOV)
	if len(deviceCardAffinity) > 0 {
		t.Errorf("Expected to find INVALID VALUE FOR CARD AFFINITY(empty string) with bus info: '0000:cc:00.1EYAL' - But it found valid one!")
	}
}

/******************************************************************************************************************/
func Test_YamlFileValidation(t *testing.T) {
	InitializeCriteriaListsMap()
	//Testing successful run of YamlFileValidation function with valid criteia lists map with card affinity
	err := YamlFileValidation(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY)
	if err != nil {
		t.Errorf("Expected SUCCESSFUL run of YamlFileValidation function with valid criteia lists map with card affinity - But got error: '%v'",
			err)
	}
	//Testing failure run of YamlFileValidation function with no device id in PF1 in the map
	firstCriteriaList := CriteriaList{"APP", nil, nil, 0, 100, 2, -1, nil, false, false, false, -1, "", "pf1", ""}
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"] = &firstCriteriaList
	err = YamlFileValidation(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY)
	if err == nil {
		t.Errorf("Expected FAILURE run of YamlFileValidation function with no device id in PF1 in the map - But got SUCCESS with no error")
	}
	//Testing failure run of YamlFileValidation function with identiacl PF couple to the current PF1 in the map
	firstCriteriaList.DeviceId = append(firstCriteriaList.DeviceId, "1592")
	firstCriteriaList.CardAffinityPfCouple = "pf1"
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"] = &firstCriteriaList
	err = YamlFileValidation(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY)
	if err == nil {
		t.Errorf("Expected FAILURE run of YamlFileValidation function with identiacl PF couple to the current PF1 in the map - But got SUCCESS with no error")
	}
	//Testing failure run of YamlFileValidation function with invalid PF couple in PF1 in the map
	firstCriteriaList.CardAffinityPfCouple = "eyal"
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"] = &firstCriteriaList
	err = YamlFileValidation(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY)
	if err == nil {
		t.Errorf("Expected FAILURE run of YamlFileValidation function with INVALID PF couple in PF1 in the map - But got SUCCESS with no error")
	}
	//Testing failure run of YamlFileValidation function with NO MATCH of PF couple in pf1 in the map
	firstCriteriaList.CardAffinityPfCouple = "pf3" //Has another couple: pf6
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"] = &firstCriteriaList
	err = YamlFileValidation(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY)
	if err == nil {
		t.Errorf("Expected FAILURE run of YamlFileValidation function with NO MATCH of PF couple in pf1 in the map - But got SUCCESS with no error")
	}
	//Testing failure run of YamlFileValidation function with PF that support PTP and RRU connection
	firstCriteriaList.CardAffinityPfCouple = ""
	firstCriteriaList.PTPSupport = true
	firstCriteriaList.RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"] = &firstCriteriaList
	err = YamlFileValidation(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY)
	if err == nil {
		t.Errorf("Expected FAILURE run of YamlFileValidation function with PTP and RRU support in pf1 in the map - But got SUCCESS with no error")
	}
	//Testing success run of YamlFileValidation function with configuring requiremets for PTP and RRU in the map
	//Configuring PTP support requirement
	firstCriteriaList.CardAffinityPfCouple = ""
	firstCriteriaList.PTPSupport = true
	firstCriteriaList.RRUConnection = false
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"] = &firstCriteriaList
	//Configuring RRU connection requirement
	secondCriteriaList := CriteriaList{"APP", nil, nil, 0, 100, 2, -1, nil, false, false, true, -1, "", "pf3", ""}
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"] = &secondCriteriaList
	err = YamlFileValidation(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY)
	if err == nil {
		t.Errorf("Expected SUCCESS run of YamlFileValidation function with VALID requiremets for PTP and RRU in the map - But got FAILURE with error: %s",
			err)
	}
}

/******************************************************************************************************************/
func Test_UpdatingCardAffinityMap(t *testing.T) {
	InitializeCardAffinityMaps()
	//Updating card affinity map in a key that already exists
	UpdatingCardAffinityMap("0000:cc:00.x", PTP_DEVICES_SLICE, PtpCardAffinityMap)
	//Updating card affinity map in a key that is not currently exists
	UpdatingCardAffinityMap("0000:ff:00.x", RRU_DEVICES_SLICE, RruCardAffinityMap)
}

/******************************************************************************************************************/
func Test_CheckDevicesAgainstCriteriaListWithNoCardAffinity(t *testing.T) {
	IntitializeDevicesSlices()
	InitializeCriteriaListsMap()
	outputString := ""
	var successfulCheckedDevices uint8 = 0
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].CardAffinityPfCouple = ""
	//Testing failure of CheckDevicesAgainstCriteriaListWithNoCardAffinity function: nil PTP slices input
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].PTPSupport = true
	result := CheckDevicesAgainstCriteriaListWithNoCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"],
		&outputString, &successfulCheckedDevices, nil, nil, nil)
	if result == true {
		t.Errorf("Expected FAILURE run of the function with input of NIL PTP slice - But got SUCCESS! outputString: %s",
			outputString)
	}
	//Testing failure of CheckDevicesAgainstCriteriaListWithNoCardAffinity function: nil RRU slices input
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].PTPSupport = false
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].RRUConnection = true
	result = CheckDevicesAgainstCriteriaListWithNoCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"],
		&outputString, &successfulCheckedDevices, nil, nil, nil)
	if result == true {
		t.Errorf("Expected FAILURE run of the function with input of NIL RRU slice - But got SUCCESS! outputString: %s",
			outputString)
	}
	//Testing success of CheckDevicesAgainstCriteriaListWithNoCardAffinity function: With RRU connection
	CheckDevicesAgainstCriteriaListWithNoCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"],
		&outputString, &successfulCheckedDevices, REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE, RRU_DEVICES_SLICE)
	if outputString != "eth2\n" {
		t.Errorf("Expected SUCCESS run of the function with input of RRU connection - But got FAILURE! outputString: %s",
			outputString)
	}
	//Testing success of CheckDevicesAgainstCriteriaListWithNoCardAffinity function: With PTP support
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].RRUConnection = false
	CheckDevicesAgainstCriteriaListWithNoCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"],
		&outputString, &successfulCheckedDevices, REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE, RRU_DEVICES_SLICE)
	if outputString != "eth2\neth1\n" {
		t.Errorf("Expected SUCCESS run of the function with input of PTP support - But got FAILURE! outputString: %s",
			outputString)
	}
	//Testing success of CheckDevicesAgainstCriteriaListWithNoCardAffinity function: With device that has neither
	//requirement for PTP support nor RRU connection
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].PTPSupport = false
	CheckDevicesAgainstCriteriaListWithNoCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"],
		&outputString, &successfulCheckedDevices, REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE, RRU_DEVICES_SLICE)
	if outputString != "eth2\neth1\neth0\n" {
		t.Errorf("Expected SUCCESS run of the function with input with NO special capabilities - But got FAILURE! outputString: %s",
			outputString)
	}
	//Testing failure of CheckDevicesAgainstCriteriaListWithNoCardAffinity function: There will be too much high link speed
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].LinkSpeed = 70000
	result = CheckDevicesAgainstCriteriaListWithNoCardAffinity(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"],
		&outputString, &successfulCheckedDevices, REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE, RRU_DEVICES_SLICE)
	if result == true {
		t.Errorf("Expected FAILURE run of the function with too much high link speed - But got SUCCESS! outputString: %s",
			outputString)
	}
}

/*************************************************************************************************************
 *                   Helper Testing Functions for TreatmentOFCardAffinityWithCapabilities
 *************************************************************************************************************/
// For RRU connection capability test:
//   Testing success of function TreatmentOFCardAffinityWithCapabilities:
//   Testing couple of RRU connection capability with card affinity
// For PTP support capability test:
//   Testing couple of PTP support capability with card affinity
func TestingCoupleOfSpecialCapabilitiesWithCardAffinity(t *testing.T, sortedCriteriaListsKeys []string) {
	//Initialize all the data structure before the next test
	InitializeCardAffinityMaps()
	IntitializeDevicesSlices()
	//Intializing the criteria lists map for RRU connection capability test:
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].RRUConnection = true
	// Requirement of RRU and card affinity: The card affinity couple requires RRU too
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].RRUConnection = true
	//Intializing the criteria lists map for PTP support capability test:
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"].PTPSupport = true
	result := TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys,
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY, RruCardAffinityMap,
		PtpCardAffinityMap, REGULAR_DEVICES_SLICE, RRU_DEVICES_SLICE,
		PTP_DEVICES_SLICE, DEVICE_CAPABILITY_TYPE_RRU_CONNECTION)
	if result == false ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].SelectedDeviceInterface != "eth5" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].SelectedDeviceInterface != "eth8" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].SelectedDeviceInterface != "eth17" {
		t.Errorf("Expected SUCCESS run of the function for RRU connection couple - But got FAILURE! PF1: %s, PF7: %s, PF8: %s",
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].SelectedDeviceInterface)
	}
	result = TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys,
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY, PtpCardAffinityMap,
		RruCardAffinityMap, REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE,
		RRU_DEVICES_SLICE, DEVICE_CAPABILITY_TYPE_PTP_SUPPORT)
	if result == false ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].SelectedDeviceInterface != "eth1" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface != "eth9" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"].SelectedDeviceInterface != "eth16" {
		t.Errorf("Expected SUCCESS run of the function for PTP support couple - But got FAILURE! PF2: %s, PF3: %s, PF6: %s",
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"].SelectedDeviceInterface)
	}
}

/******************************************************************************************************************/
// For RRU connection capability test:
//   Testing another success of function TreatmentOFCardAffinityWithCapabilities:
//   Testing couple of RRU connection capability and PTP support with combination of card affinity
// For PTP support capability test:
//   Testing PTP support capability: There will be not enough devices so the application will
//   take device from a PTP support couple.
func TestingCoupleOfDifferentSpecialCapabilitiesWithCardAffinity(t *testing.T, sortedCriteriaListsKeys []string) {
	//Initialize all the data structure before the next test
	InitializeCardAffinityMaps()
	IntitializeDevicesSlices()
	InitializeCriteriaListsMap()
	//Intializing the criteria lists map for RRU connection capability test:
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].RRUConnection = true
	// Requirement of RRU and card affinity: The card affinity couple requires PTP support
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].PTPSupport = true
	//Intializing the criteria lists map for PTP support capability test:
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf4"].PTPSupport = true
	result := TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys,
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY, RruCardAffinityMap,
		PtpCardAffinityMap, REGULAR_DEVICES_SLICE, RRU_DEVICES_SLICE,
		PTP_DEVICES_SLICE, DEVICE_CAPABILITY_TYPE_RRU_CONNECTION)
	if result == false ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface != "eth5" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].SelectedDeviceInterface != "eth2" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].SelectedDeviceInterface != "eth7" {
		t.Errorf("Expected SUCCESS run of the function for RRU connection couple - But got FAILURE! PF3: %s, PF7: %s, PF8: %s",
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].SelectedDeviceInterface)
	}
	result = TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys,
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY, PtpCardAffinityMap,
		RruCardAffinityMap, REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE,
		RRU_DEVICES_SLICE, DEVICE_CAPABILITY_TYPE_PTP_SUPPORT)
	if result == false ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].SelectedDeviceInterface != "eth1" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf4"].SelectedDeviceInterface != "eth9" {
		t.Errorf("Expected SUCCESS run of the function for PTP support with NO card affinity - But got FAILURE! PF2: %s, PF4: %s",
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf4"].SelectedDeviceInterface)
	}
}

/******************************************************************************************************************/
// For RRU connection capability test:
//   Testing another success of function TreatmentOFCardAffinityWithCapabilities:
//   Testing couple of RRU connection capability required with no card affinity
// For PTP support capability test:
//   Testing PTP support capability: There will be not enough devices so the function will fail!
func TestingCoupleOfSpecialCapabilitiesWithNoCardAffinity(t *testing.T, sortedCriteriaListsKeys []string) {
	//Initialize ONLY the criteria lists before testing failures of the functions
	InitializeCriteriaListsMap()
	//Intializing the criteria lists map for RRU connection capability test:
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].NumVFSupp = 64
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].NumVFSupp = 64
	//Intializing the criteria lists map for PTP support capability test:
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].PTPSupport = true
	result := TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys,
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY, RruCardAffinityMap,
		PtpCardAffinityMap, REGULAR_DEVICES_SLICE, RRU_DEVICES_SLICE,
		PTP_DEVICES_SLICE, DEVICE_CAPABILITY_TYPE_RRU_CONNECTION)
	if result == false ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].SelectedDeviceInterface != "eth8" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].SelectedDeviceInterface != "eth17" {
		t.Errorf("Expected SUCCESS run of the function for RRU connection devices with NO card affinity - But got FAILURE! PF1: %s, PF2: %s",
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].SelectedDeviceInterface)
	}
	result = TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys,
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY, PtpCardAffinityMap,
		RruCardAffinityMap, REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE,
		RRU_DEVICES_SLICE, DEVICE_CAPABILITY_TYPE_PTP_SUPPORT)
	if result == true ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface != "" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].SelectedDeviceInterface != "" {
		t.Errorf("Expected Failure run of the function for PTP connection devices with card affinity - But got SUCCESS! PF3: %s, PF7: %s",
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].SelectedDeviceInterface)
	}
}

/******************************************************************************************************************/
// For RRU connection capability test:
//   Testing another failure of function TreatmentOFCardAffinityWithCapabilities:
//   Testing couple of RRU connection capability and no requirement for special capabilites
//   For the couple PF, while the devices left are couple in the RRU devices slice
// For PTP support capability test:
//   Testing another success of function TreatmentOFCardAffinityWithCapabilities:
//   Testing couple of RRU connection capability and no requirement for special capabilites
//   For the couple PF, while the devices left are couple in the RRU devices slice
func TestingFailureForTreatmentOFCardAffinityWithCapabilities(t *testing.T, sortedCriteriaListsKeys []string) {
	//Initialize all the data structure before the next test
	InitializeCardAffinityMaps()
	IntitializeDevicesSlices()
	InitializeCriteriaListsMap()
	//Intializing the criteria lists map for RRU connection capability test:
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].NumVFSupp = 64
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf4"].RRUConnection = true
	result := TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys,
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY, RruCardAffinityMap,
		PtpCardAffinityMap, REGULAR_DEVICES_SLICE, RRU_DEVICES_SLICE,
		PTP_DEVICES_SLICE, DEVICE_CAPABILITY_TYPE_RRU_CONNECTION)
	if result == true ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].SelectedDeviceInterface != "eth5" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].SelectedDeviceInterface != "eth2" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface != "eth8" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"].SelectedDeviceInterface != "eth17" {
		t.Errorf("Expected Failure run of the function for RRU connection devices with card affinity - But got SUCCESS! PF1: %s, PF2: %s, PF3: %s, PF6: %s",
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"].SelectedDeviceInterface)
	}
	//Initialize ONLY the criteria lists before testing
	InitializeCriteriaListsMap()
	//Intializing the criteria lists map for PTP support capability test:
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf4"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].PTPSupport = true
	result = TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys,
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY, PtpCardAffinityMap,
		RruCardAffinityMap, REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE,
		RRU_DEVICES_SLICE, DEVICE_CAPABILITY_TYPE_PTP_SUPPORT)
	if result == false ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface != "eth1" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf4"].SelectedDeviceInterface != "eth7" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"].SelectedDeviceInterface != "eth0" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].SelectedDeviceInterface != "eth9" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].SelectedDeviceInterface != "eth16" {
		t.Errorf("Expected SUCCESS run of the function for PTP support devices with card affinity - But got FAILURE! PF3: %s, PF4: %s, PF6: %s, PF7: %s, PF8: %s",
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf4"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].SelectedDeviceInterface)
	}
}

/******************************************************************************************************************/
//This function will test thouroughly the functions:
// 1) TreatmentOFCardAffinityWithCapabilities
// 2) FindProperCoupleDevicesFromDifferentCardAffinityMaps
// 3) FindDevicesForPfCoupleWithSpecialCapability
func Test_TreatmentOFCardAffinityWithCapabilities(t *testing.T) {
	InitializeCriteriaListsMap()
	//Sorting the criteria lists Map by the PF's (map keys)
	sortedCriteriaListsKeys := make([]string, 0, len(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY))
	for pf := range CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY {
		sortedCriteriaListsKeys = append(sortedCriteriaListsKeys, pf)
	}
	sort.Strings(sortedCriteriaListsKeys)

	TestingCoupleOfSpecialCapabilitiesWithCardAffinity(t, sortedCriteriaListsKeys)

	TestingCoupleOfDifferentSpecialCapabilitiesWithCardAffinity(t, sortedCriteriaListsKeys)

	TestingCoupleOfSpecialCapabilitiesWithNoCardAffinity(t, sortedCriteriaListsKeys)

	TestingFailureForTreatmentOFCardAffinityWithCapabilities(t, sortedCriteriaListsKeys)
}

/******************************************************************************************************************/
//Testing function CheckDevicesAgainstCriteria:
//Testing a scenario that there are requirements for:
// 1) RRU connection in a card affinity couple.
// 2) PTP support in other card affinity couple.
func Test_CheckDevicesAgainstCriteriaWithCardAffinityAndSpecialCapabilities(t *testing.T) {
	//Initialize all the data structure before the next test
	InitializeCardAffinityMaps()
	IntitializeDevicesSlices()
	InitializeCriteriaListsMap()

	//Testing Success for function CheckDevicesAgainstCriteria
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].PTPSupport = true
	numberDevices := NUMBER_OF_DEVICES_REQUIRED
	NUMBER_OF_DEVICES_REQUIRED = 8
	USE_OF_RRU_CONNECTION_CAPABILITY = true
	USE_OF_PTP_SUPPORT_CAPABILITY = true
	USE_OF_CARD_AFFINITY_CAPABILITY = true

	outputResult := CheckDevicesAgainstCriteria(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE, RRU_DEVICES_SLICE)
	//Since the tested function succeeded
	if outputResult == "" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface != "eth5" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"].SelectedDeviceInterface != "eth4" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].SelectedDeviceInterface != "eth1" ||
		CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].SelectedDeviceInterface != "eth0" {
		t.Errorf("Expected SUCCESS run of the function for special capabilities with card affinity - But got FAILURE! PF3: %s, PF6: %s, PF7: %s, PF8: %s",
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf6"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf7"].SelectedDeviceInterface,
			CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf8"].SelectedDeviceInterface)
	}
	//Testing Failure for function CheckDevicesAgainstCriteria with RRU connection requirements
	InitializeCriteriaListsMap()
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf4"].RRUConnection = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf5"].RRUConnection = true

	outputResult = CheckDevicesAgainstCriteria(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE, RRU_DEVICES_SLICE)
	//Since the tested function should fail
	if outputResult != "" {
		t.Errorf("Expected FAILURE run of the function for special capabilities with card affinity and too many RRU devices - But got SUCCESS!")
	}
	//Testing Failure for function CheckDevicesAgainstCriteria with PTP support requirements
	InitializeCriteriaListsMap()
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf1"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf2"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf3"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf4"].PTPSupport = true
	CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY["pf5"].PTPSupport = true

	outputResult = CheckDevicesAgainstCriteria(CRITERIA_LISTS_MAP_WITH_CARD_AFFINITY,
		REGULAR_DEVICES_SLICE, PTP_DEVICES_SLICE, RRU_DEVICES_SLICE)
	//Since the tested function should fail
	if outputResult != "" {
		t.Errorf("Expected FAILURE run of the function for special capabilities with card affinity and too many PTP devices - But got SUCCESS!")
	}
	//Rollback the changes in the global variables for the future tests
	NUMBER_OF_DEVICES_REQUIRED = numberDevices
	USE_OF_RRU_CONNECTION_CAPABILITY = false
	USE_OF_PTP_SUPPORT_CAPABILITY = false
	USE_OF_CARD_AFFINITY_CAPABILITY = false
}

func Test_CheckInputParameters(t *testing.T) {
	//Testing Success of the function CheckInputParameters: No input parameters.
	argumentsSlice := []string{"sriov_detection.go"}
	CheckInputParameters(argumentsSlice)
	//Testing Success of the function CheckInputParameters: 1 "debug_mode" parameter.
	argumentsSlice = []string{"sriov_detection.go", "debug_mode"}
	CheckInputParameters(argumentsSlice)
	//Testing Success of the function CheckInputParameters: 1 parameter:
	//Valid YAML file name.
	argumentsSlice = []string{"sriov_detection.go", "sriov_detection_configuration.yml"}
	CheckInputParameters(argumentsSlice)
	//Testing Success of the function CheckInputParameters: 2 parameters:
	//Valid YAML file name and "debug_mode" parameter.
	argumentsSlice = []string{"sriov_detection.go", "sriov_detection_configuration.yml",
		"debug_mode"}
	CheckInputParameters(argumentsSlice)

	//Testing Failure of the function CheckInputParameters: INVALID YAML file name
	//Turnning off the panic.
	defer func() {
		r := recover()
		if r != nil {
			fmt.Println("RECOVER from panic: ", r)
		}
	}()
	//Since there is no recover function in CheckInputParameters function,
	// the recover of this test will occur ONLY ONCE
	// (character of recover function in GoLang).
	//So currently the failure test contain only 1 example:
	//1 input parameters with Invalid YAML file
	//(There is another failure test in Test_RunMain function)
	argumentsSlice = []string{"sriov_detection.go", "eyal"}
	CheckInputParameters(argumentsSlice)
}

/*****************************************************************************************
 *                       Detect TEST Functions with hard coded NIC's
 * (Tests that will pass successfully ONLY in: Ubuntu 20.04)
 ****************************************************************************************/
func Test_ParseConfigurationFile(t *testing.T) {
	//Testing Failure of the ParseConfigurationFile function: Invalid file name
	CONFIGURATION_FILE = "EYAL"
	_, err := ParseConfigurationFile()
	if err == nil {
		t.Errorf("Expected error from ParseConfigurationFile function after changing the configuration file to invalid value!")
	}
	//Testing Failure of the ParseConfigurationFile function: YAML file unmarshal error
	CONFIGURATION_FILE = "sriov_detection_configuration_unmarshal_failure.yml"
	_, err = ParseConfigurationFile()
	if err == nil {
		t.Errorf("Expected error from ParseConfigurationFile function after changing the configuration file to file with unmarshal error!")
	}
	//Testing Failure of the ParseConfigurationFile function: YAML file with PF with no device ID
	CONFIGURATION_FILE = "sriov_detection_configuration_no_device_id.yml"
	_, err = ParseConfigurationFile()
	if err == nil {
		t.Errorf("Expected error from ParseConfigurationFile function after changing the configuration file to file with lack of device id!")
	}
	//Restoring the global variables the valid value
	CONFIGURATION_FILE = "sriov_detection_configuration.yml"
	USE_OF_CARD_AFFINITY_CAPABILITY = false
}

/******************************************************************************************************************/
func Test_CheckIfDeviceCurrentlyUsed(t *testing.T) {
	nicWithValidIpv4Found := false
	nicProperToSriovFound := false
	//We will need to find the entire struct of the network inetrface (from net package)
	//from the network interfaces slice
	for _, networkDevice := range networkInterfaces {
		if networkDevice.Name == NETWORK_INTERFACE_WITH_VALID_IPV4 && nicWithValidIpv4Found == false {
			nicWithValidIpv4Found = true
			//Here we have the data to test the function
			if CheckIfDeviceCurrentlyUsed(networkDevice) == false {
				t.Errorf("Expected success to find the network interface '%s' as a Device Currently Used - But it failed!",
					NETWORK_INTERFACE_WITH_VALID_IPV4)
			}
		} else if networkDevice.Name == NETWORK_INTERFACE_PROPER_TO_SRIOV && nicProperToSriovFound == false {
			nicProperToSriovFound = true
			//Here we have the data to test the function
			if CheckIfDeviceCurrentlyUsed(networkDevice) == true {
				t.Errorf("Expected failure to find the network interface '%s' as a Device Currently Used - But it succeeded!",
					NETWORK_INTERFACE_PROPER_TO_SRIOV)
			}
		}
		if nicWithValidIpv4Found && nicProperToSriovFound {
			break
		}
	}
}

/******************************************************************************************************************/
func Test_CheckVendorNameAndProductId(t *testing.T) {
	//Testing success of CheckVendorNameAndProductId function
	result, deviceId := CheckVendorNameAndProductId(ptrPci, NIC_BUS_INFO)
	if result == false || deviceId == "" {
		t.Errorf("Expected success to run CheckVendorNameAndProductId function with bus info '%s' - But it failed!",
			NIC_BUS_INFO)
	}
	//Testing failure of CheckVendorNameAndProductId function: with invalid Bus Info as input parameter.
	result, _ = CheckVendorNameAndProductId(ptrPci, "invalid_Bus_Info")
	if result == true {
		t.Errorf("Expected failure to run CheckVendorNameAndProductId function with INVALID bus info - But it succeeded!")
	}
	//Testing failure of CheckVendorNameAndProductId function: with Bus Info of non Intel device as input parameter.
	result, _ = CheckVendorNameAndProductId(ptrPci, NETWORK_BUS_INFO_OF_NON_INTEL_NIC)
	if result == true {
		t.Errorf("Expected failure to run CheckVendorNameAndProductId function with bus info of NON INTEL NIC : %s - But it succeeded!",
			NETWORK_BUS_INFO_OF_NON_INTEL_NIC)
	}
}

/******************************************************************************************************************/
func Test_DetectCapabilityFromLinuxFS(t *testing.T) {
	//Testing success of DetectCapabilityFromLinuxFS function
	numberVfs := DetectCapabilityFromLinuxFS(NETWORK_INTERFACE_PROPER_TO_SRIOV, "sriov_totalvfs")
	if numberVfs == 0 {
		t.Errorf("Expected to find 64 or more VF's in the NIC '%s' - But it found 0 VF's!",
			NETWORK_INTERFACE_PROPER_TO_SRIOV)
	}
	//Testing failure of DetectCapabilityFromLinuxFS function: NIC with no sriov_totalvfs file in the Linux
	numberVfs = DetectCapabilityFromLinuxFS(NETWORK_INTERFACE_NOT_SUPPORTING_SRIOV, "sriov_totalvfs")
	if numberVfs > -1 {
		t.Errorf("Expected to find 0 VF's in the NIC '%s' and failure of DetectCapabilityFromLinuxFS function But it found %d VF's!",
			NETWORK_INTERFACE_NOT_SUPPORTING_SRIOV, numberVfs)
	}
	//Testing failure of DetectCapabilityFromLinuxFS function: NIC with file device that contain characters that fail the
	//strconv.Atoi function
	deviceId := DetectCapabilityFromLinuxFS(NETWORK_INTERFACE_NOT_SUPPORTING_SRIOV, "device")
	if numberVfs > -1 {
		t.Errorf("Expected to find invalid value of -1 while trying to detect the device id for the NIC '%s' and failure of DetectCapabilityFromLinuxFS function But the value was %d !",
			NETWORK_INTERFACE_NOT_SUPPORTING_SRIOV, deviceId)
	}

}

/******************************************************************************************************************/
func Test_GetDeviceDriver(t *testing.T) {
	//Testing success of GetDeviceDriver function
	driver := GetDeviceDriver(ptrEthToolHandle, NETWORK_INTERFACE_PROPER_TO_SRIOV)
	//TODO: Understand why do we need this sleep in order to get successful test
	time.Sleep(1 * time.Millisecond)
	if driver == "" {
		t.Errorf("Expected to find valid driver in the NIC '%s' - But it found empty string!",
			NETWORK_INTERFACE_PROPER_TO_SRIOV)
	}
	//Testing failure of GetDeviceDriver function
	driver = GetDeviceDriver(ptrEthToolHandle, "Eyal")
	if driver != "" {
		t.Errorf("Expected to find empty string for driver with NO VALID network interface - But it found valid driver!")
	}
}

/******************************************************************************************************************/
func Test_GetDeviceLinkSpeed(t *testing.T) {
	//Testing success of GetDeviceLinkSpeed function
	deviceLinkedSpeed := GetDeviceLinkSpeed(ptrEthToolHandle, NETWORK_INTERFACE_PROPER_TO_SRIOV)
	if deviceLinkedSpeed == 0 {
		t.Errorf("Expected to find valid linked speed in the NIC '%s' - But it found 0!",
			NETWORK_INTERFACE_PROPER_TO_SRIOV)
	}
	//Testing failure of GetDeviceLinkSpeed function
	deviceLinkedSpeed = GetDeviceLinkSpeed(ptrEthToolHandle, "Eyal")
	if deviceLinkedSpeed != 0 {
		t.Errorf("Expected to find linked speed = 0 with NO VALID network interface - But it found positive linked speed!")
	}
}

/******************************************************************************************************************/
func Test_GetDeviceLinkState(t *testing.T) {
	//Testing failure of GetDeviceLinkState function: Input invalid network interface
	deviceLinkedState := GetDeviceLinkState(ptrEthToolHandle, "Eyal")
	if deviceLinkedState != LINK_STATE_INVALID_VALUE {
		t.Errorf("Expected to find INVALID VALUE FOR LINK STATE ( %d ) with NO VALID network interface - But it found valid link state!",
			LINK_STATE_INVALID_VALUE)
	}
	//Testing failure of GetDeviceLinkState function: NIC with invalid link state
	deviceLinkedState = GetDeviceLinkState(ptrEthToolHandle, NETWORK_INTERFACE_WITH_INVALID_LINK_STATE)
	if deviceLinkedState != LINK_STATE_INVALID_VALUE {
		t.Errorf("Expected to find INVALID VALUE FOR LINK STATE ( %d ) with network interface: %s - But it found valid link state: %d",
			LINK_STATE_INVALID_VALUE, NETWORK_INTERFACE_WITH_INVALID_LINK_STATE, deviceLinkedState)
	}
	//Making sure that the NIC is in DOWN state - using sudo permissions
	setLinkDownCmd := "sudo ip link set " + NETWORK_INTERFACE_PROPER_TO_SRIOV + " down"
	_, err := exec.Command("bash", "-c", setLinkDownCmd).CombinedOutput()
	if err != nil {
		t.Errorf("Device with interface name %s has 'sudo ip link set' command error : %v \n",
			NETWORK_INTERFACE_PROPER_TO_SRIOV, err)
	}
	//Testing success of GetDeviceLinkState function: NIC that currently is DOWN
	deviceLinkedState = GetDeviceLinkState(ptrEthToolHandle, NETWORK_INTERFACE_PROPER_TO_SRIOV)
	if deviceLinkedState != LINK_STATE_DOWN {
		t.Errorf("Expected to find valid linked state: DOWN in the NIC '%s' - But it found %d!",
			NETWORK_INTERFACE_PROPER_TO_SRIOV, deviceLinkedState)
	}
	//Making sure that the NIC is in UP state - using sudo permissions
	setLinkUpCmd := "sudo ip link set " + NETWORK_INTERFACE_PROPER_TO_SRIOV + " up"
	_, err = exec.Command("bash", "-c", setLinkUpCmd).CombinedOutput()
	if err != nil {
		t.Errorf("Device with interface name %s has 'sudo ip link set' command error : %s \n",
			NETWORK_INTERFACE_PROPER_TO_SRIOV, err)
	}
	//Testing success of GetDeviceLinkState function: NIC that currently is UP
	deviceLinkedState = GetDeviceLinkState(ptrEthToolHandle, NETWORK_INTERFACE_PROPER_TO_SRIOV)
	if deviceLinkedState != LINK_STATE_UP {
		t.Errorf("Expected to find valid linked state: UP in the NIC '%s' - But it found %d!",
			NETWORK_INTERFACE_PROPER_TO_SRIOV, deviceLinkedState)
	}
}

/******************************************************************************************************************/
//ChangeLinkState REQUIRES sudo permissions in the Linux.
//Assumption: This unit test is not running with sudo permissions:
//So We will use it in order to test failures of the function.
func Test_ChangeLinkState(t *testing.T) {
	//Testing Failure of function ChangeLinkState with invalid input
	if ChangeLinkState(true, "eyal") == true {
		t.Errorf("Expected FAILURE from function ChangeLinkState with invalid NIC interface: 'eyal' - But it did not!")
	}
	//Testing Failure of function ChangeLinkState with no sudo permissions - Trying change the NIC to state UP
	if ChangeLinkState(true, NETWORK_INTERFACE_WITH_PTP_MASTER_CONNECTION) == true {
		t.Errorf("Expected FAILURE from function ChangeLinkState (trying to UP %s) with no sudo permissions - But it succeeded!",
			NETWORK_INTERFACE_WITH_PTP_MASTER_CONNECTION)
	}
	//Testing Failure of function ChangeLinkState with no sudo permissions - Trying change the NIC to state DOWN
	if ChangeLinkState(false, NETWORK_INTERFACE_WITH_PTP_MASTER_CONNECTION) == true {
		t.Errorf("Expected FAILURE from function ChangeLinkState (trying to DOWN %s) with no sudo permissions - But it succeeded!",
			NETWORK_INTERFACE_WITH_PTP_MASTER_CONNECTION)
	}
}

/******************************************************************************************************************/
func Test_GetDevicePtpSupport(t *testing.T) {
	//Making sure that the NIC is in UP state - using sudo permissions
	setLinkUpCmd := "sudo ip link set " + NETWORK_INTERFACE_WITH_PTP_MASTER_CONNECTION + " up"
	_, err := exec.Command("bash", "-c", setLinkUpCmd).CombinedOutput()
	if err != nil {
		t.Errorf("Device with interface name %s has 'sudo ip link set' command error : %s \n",
			NETWORK_INTERFACE_WITH_PTP_MASTER_CONNECTION, err)
	}
	//Changing the value of the PTP timeout in order to get success
	previousValueOfPtpTimeout := TIMEOUT_FOR_FIND_PTP_MASTER
	TIMEOUT_FOR_FIND_PTP_MASTER = 21000
	//Testing success of GetDevicePtpSupport function
	if GetDevicePtpSupport(NETWORK_INTERFACE_WITH_PTP_MASTER_CONNECTION, LINK_STATE_UP) == false {
		t.Errorf("Expected to detect PTP master connection for the NIC '%s' - But it did not!",
			NETWORK_INTERFACE_WITH_PTP_MASTER_CONNECTION)
	}
	//Testing failure of GetDevicePtpSupport function: Invalid network device interface input
	if GetDevicePtpSupport("Eyal", LINK_STATE_DOWN) == true {
		t.Errorf("Expected failure for wrong input network device interface: 'Eyal'. But the function GetDevicePtpSupport succeeded")
	}
	TIMEOUT_FOR_FIND_PTP_MASTER = previousValueOfPtpTimeout
}

/*****************************************************************************************
 *                       Testing the main function
 ****************************************************************************************/
// This function tests ONLY the main function response to the input parameters.
func Test_RunMain(t *testing.T) {
	//Clear the card affinity maps before using the function
	for cardAffinity := range CardAffinityMap {
		delete(CardAffinityMap, cardAffinity)
	}
	for cardAffinity := range RruCardAffinityMap {
		delete(RruCardAffinityMap, cardAffinity)
	}
	for cardAffinity := range PtpCardAffinityMap {
		delete(PtpCardAffinityMap, cardAffinity)
	}

	//Turnning off the panic.
	defer func() {
		r := recover()
		if r != nil {
			fmt.Println("RECOVER from panic: ", r)
		}
	}()

	//Using table tests pattern
	testCases := []struct {
		testName string
		args     []string
	}{
		//Testing success input parameters to the application
		{"flags set to empty string", []string{}},             //No parameters
		{"flags set with debug mode", []string{"debug_mode"}}, //1 valid parameter: debug_mode
		{"flags set with no debug mode But with YAML file",
			[]string{"sriov_detection_configuration.yml"}}, //1 valid parameter: YAML configurations file
		{"flags set with YAML file AND with debug mode",
			[]string{"sriov_detection_configuration.yml", "debug_mode"}}, //2 valid parameters
		//Since there is no recover function in main function of the application the recover in the loop
		//of this test will occur ONLY ONCE (character of recover function in GoLang).
		//So currently the failure test contain only 1 example:
		//2 input parameters with Invalid YAML file
		{"flags set with debug mode", []string{"dek", "debug_mode"}},
	}
	for mainTestIndex, tc := range testCases {
		// this call is required because otherwise flags panics, if args are set between flag.Parse calls
		flag.CommandLine = flag.NewFlagSet(tc.testName, flag.ExitOnError)
		// we need a value to set args[0] to, cause flag begins parsing at args[1]
		os.Args = append([]string{tc.testName}, tc.args...)
		main()

		if mainTestIndex == 4 {
			// Never reaches here if 'flags set to empty string' case panics.
			t.Errorf("Testing main function with INVALID YAML file input parameter did not panic like it should!")
		}
	}
}
