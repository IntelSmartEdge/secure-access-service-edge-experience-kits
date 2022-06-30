/**
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation

 * This application will be executed from an Ansible role during the provisioning process
 * in deployment of experiance kit.
 * The user configures in a configuration YAML file:
   1) How many proper NIC's he needs for SR-IOV.
   2) What is the priority of each NIC: According to the order of the pf's in the YAML file.
   3) In each NIC - which capabilities does he need.
 * The goal is to find enough proper NIC's in the Linux for configuring and using SR-IOV.
 *
 *  @author Eyal Belkin
 *  @version 1.2 May/13/2022
 *
 * Input: 2 Optional parameters: Debug mode and configuration file name.
 *
 * Output: A print of string with NIC's interfaces of the proper devices for SR-IOV.
 *
 * A brief at:
 * https://github.com/smart-edge-open/
 * docs/tree/main/components/networking/sriov-detection-app.md
*/

package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/safchain/ethtool"    //Package for Linux only
	"github.com/vishvananda/netlink" //Package for Linux only
	"gopkg.in/yaml.v2"
)

/*****************************************************************************************
 *                   Global Types and IOTA
 ****************************************************************************************/
type LinkStateType uint8

const (
	LINK_STATE_DOWN          LinkStateType = iota // 0
	LINK_STATE_UP                                 // 1
	LINK_STATE_INVALID_VALUE                      // 2
)

type CardAffinityPfCoupleType uint8

const (
	CARD_AFFINITY_PF_COUPLE_TYPE_EMPTY_MARKED_TRUE               CardAffinityPfCoupleType = iota // 0
	CARD_AFFINITY_PF_COUPLE_TYPE_EMPTY_MARKED_FALSE                                              // 1
	CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_SMALLER_THAN_COUPLED                                 // 2
	CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_BIGGER_THAN_COUPLED                                  // 3
)

type DeviceCapabilityType uint8

const (
	DEVICE_CAPABILITY_TYPE_REGULAR        DeviceCapabilityType = iota // 0
	DEVICE_CAPABILITY_TYPE_PTP_SUPPORT                                // 1
	DEVICE_CAPABILITY_TYPE_RRU_CONNECTION                             // 2
)

/*****************************************************************************************
 *                   Global Variables and Constants
 ****************************************************************************************/
const INVALID_VALUE_FOR_NUMA_NODE = -1
const INVALID_VALUE_FOR_SPECIFIC_PORT = -1
const INVALID_VALUE_FOR_CAPABILITY_FROM_LINUX_FS = -1 //Used by the function DetectCapabilityFromLinuxFS
const BYTE_BUFFER_SIZE_FOR_PTP_MESSAGES = 1024        //Design to contain from 1 ptp4l stdout message to at least 3
const INTEL_VENDOR_NAME = "Intel Corporation"         //Using to filter NIC's that has Intel vendor ONLY

var CONFIGURATION_FILE string = "sriov_detection_configuration.yml"
var DEBUG_MODE bool = false
var NUMBER_OF_DEVICES_REQUIRED uint8 = 4    //Default value of 4 devices
var TIMEOUT_FOR_FIND_PTP_MASTER int = 21000 //In milli-seconds
var USE_OF_CARD_AFFINITY_CAPABILITY bool = false
var USE_OF_PTP_SUPPORT_CAPABILITY bool = false
var USE_OF_RRU_CONNECTION_CAPABILITY bool = false

/*****************************************************************************************
 *                    Structs
 ****************************************************************************************/

type SupDevice struct {
	InterfaceName          string
	PciBus                 string
	DeviceId               string
	Driver                 string
	LinkSpeed              uint64 //Units: Mb/s
	NumVFSupp              uint32
	LinkState              LinkStateType
	NUMALocation           int8 //Store the value of the NUMA Node of the device in the Linux
	ConnectorType          string
	DDPSupport             bool
	PTPSupport             bool
	RRUConnection          bool
	SpecificPortNumber     int8
	CardAffinity           string
	CheckedAgainstCriteria bool //Helper field for function CheckDevicesAgainstCriteria
	IsCardAffinityCouple   bool //Helper field for function CheckDevicesAgainstCriteria
}

type ConfigurationsStruct struct {
	TimeoutForFindPTPMaster int `yaml:"timeoutForFindPTPMaster"`
}

type CriteriaList struct {
	//Fields that contain values from the YAML file
	Functionality        string        `yaml:"functionality"`
	DeviceId             []string      `yaml:"deviceId"`
	Driver               []string      `yaml:"driver"`
	LinkSpeed            uint64        `yaml:"linkSpeed"`
	NumVFSupp            uint32        `yaml:"numVFSupp"`
	LinkState            LinkStateType `yaml:"linkState"`
	NUMALocation         int8          `yaml:"numaLocation"`
	ConnectorType        []string      `yaml:"connectorType"`
	DDPSupport           bool          `yaml:"ddpSupport"`
	PTPSupport           bool          `yaml:"ptpSupport"`
	RRUConnection        bool          `yaml:"rruConnection"`
	SpecificPortNumber   int8          `yaml:"specificPortNumber"`
	CardAffinityPfCouple string        `yaml:"cardAffinityPfCouple"`
	//Helper field for the checking against the criteria stage.
	//Contains the card affinity value. It will be used when there will be
	//a requirement for card affinity couples.
	CardAffinity string
	//Helper field using card affinity with RRU and PTP capabilities
	SelectedDeviceInterface string
}

type YamlConfigurationsData struct {
	CriteriaListsMap map[string]*CriteriaList `yaml:"criteriaLists"`
	Configurations   ConfigurationsStruct     `yaml:"configurations"`
}

type CardAffinityDevicesInfo struct {
	DevicesIndexesSlice           []uint8
	ChosenDeviceHasNoCardAffinity bool
}

//Helper maps for the checking against the criteria stage. It will be used when there will be
//a requirement for card affinity couples.
//It will be created in the devices detection stage.
var CardAffinityMap map[string]*CardAffinityDevicesInfo = make(map[string]*CardAffinityDevicesInfo)
var PtpCardAffinityMap map[string]*CardAffinityDevicesInfo = make(map[string]*CardAffinityDevicesInfo)
var RruCardAffinityMap map[string]*CardAffinityDevicesInfo = make(map[string]*CardAffinityDevicesInfo)

/*****************************************************************************************
 *                        Functions
 ****************************************************************************************/
/****************************************************************************************************************
    @brief Helper function for print to the stdout in debug mode

    @param stringToPrint - string to print to stdout
    @param args - values of variables to print as part of the string

******************************************************************************************************************/
func PrintStdOut(stringToPrint string, args ...interface{}) {
	if DEBUG_MODE == true {
		log.Printf(stringToPrint, args...)
	}
}

/****************************************************************************************************************
    @brief Unmarshaler interface: may be implemented by types to customize their behavior when being
           unmarshaled from a YAML document.
           Using the above API of package 'yaml' ONLY for CriteriaList objects to Initialize fields
           in each criteria list BEFORE the unmarshalling action.

    @param func(interface{}) error : function that may be called
	       to unmarshal the SRIOV detection configuration YAML values into CriteriaList struct.
    @return error in case of failure
	        nil in case of success

******************************************************************************************************************/
func (criteriaListPtr *CriteriaList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	//Before the decoding of the YAML file:
	//Initializing some integers to specific defaults in each criteria list
	criteriaListPtr.LinkState = LINK_STATE_INVALID_VALUE
	criteriaListPtr.NUMALocation = INVALID_VALUE_FOR_NUMA_NODE
	criteriaListPtr.SpecificPortNumber = INVALID_VALUE_FOR_SPECIFIC_PORT
	//Casting the receiver pointer to a pointer to the alias type and then
	//Unmarshaling in to the new pointer. This trick with pointers prevents
	//infinite recursion call to this function.
	type criteriaListYaml CriteriaList
	err := unmarshal((*criteriaListYaml)(criteriaListPtr))
	if err != nil {
		return err
	}
	return nil
}

/****************************************************************************************************************
    @brief This function validates the parsed criteria lists from the map of the
           configuration YAML file. The function will validate specifically the part
           of card affinity couples configurations.

    Assumptions: 1) criteriaListsMap and pf are not empty.
                 2) criteriaListPtr is not nil.
                 3) criteriaListPtr.CardAffinityPfCouple exist and have non empty string.
    @param MAP: string --> CriteriaList : The map of the parsed YAML file:
                 1) The key is the PF number.
                 2) The data is pointer to criteria list required from this PF.
    @param Pointer to CriteriaList
    @param String : The curretn PF.
    @return Error: In case of failure after checking the users configurations.
	        nil: In case of success.

******************************************************************************************************************/
func ValidateCardAffinityCouplesInYamlFile(criteriaListsMap map[string]*CriteriaList,
	criteriaListPtr *CriteriaList, pf string) error {
	//Checking that the configured card affinity PF couple is different from the current PF
	if criteriaListPtr.CardAffinityPfCouple == pf {
		PrintStdOut("ValidateCardAffinityCouplesInYamlFile: The Card affinity pf couple in criteria list %s is IDENTICAL to the current PF! It has to be with different PF!",
			pf)
		return errors.New("INVALID card affinity pf couple in criteria list: The card affinity configured MUST be different from the current PF")
	}
	//Checking if the configured card affinity is valid: A PF in the criteria lists map
	if _, foundCardAffinityInMap := criteriaListsMap[criteriaListPtr.CardAffinityPfCouple]; foundCardAffinityInMap {
		//In case that the PF is valid: Checking if the card affinity Pf couple is valid
		if len(criteriaListsMap[criteriaListPtr.CardAffinityPfCouple].CardAffinityPfCouple) > 0 {
			//Currently (1.2 version) the validation supports only 2 devices in 1 card affinity)
			if criteriaListsMap[criteriaListPtr.CardAffinityPfCouple].CardAffinityPfCouple != pf {
				PrintStdOut("ValidateCardAffinityCouplesInYamlFile: NO Card affinity pf couple MATCH in criteria lists %s and %s !",
					pf, criteriaListPtr.CardAffinityPfCouple)
				return errors.New("NO Card affinity pf couple MATCH in 2 criteria lists in the YAML file")
			}
		} else { //In this scenario the user configured the card affinity Pf couple in 1 PF only.
			//We will add in the map the card affinity Pf couple in the coupled PF
			criteriaListsMap[criteriaListPtr.CardAffinityPfCouple].CardAffinityPfCouple = pf
		}
	} else { //Invalid card affinity configuration by the user: The coupled PF does not exist in the YAML file/
		PrintStdOut("ValidateCardAffinityCouplesInYamlFile: The Card affinity pf couple in criteria list %s : %s is not a PF in the criteriaLists section in YAML file!",
			pf, criteriaListPtr.CardAffinityPfCouple)
		return errors.New("INVALID card affinity pf couple in criteria list: It is not a PF in the criteriaLists section in the YAML file")
	}
	USE_OF_CARD_AFFINITY_CAPABILITY = true //Modifying global variable
	return nil
}

/****************************************************************************************************************
    @brief This function validates the parsed map of the configuration YAML file.
           Currently (1.2 version) it validates ONLY the criteria lists section and
           it check 4 users configurations:
		   1) Device Id in each PF.
		   2) The PTP support in each PF
		   3) The RRU connection in each PF
		   4) Card Affinity Pf Couple.
           This function will update global variables for special capabilities:
           PTP support, RRU connection and Card affinity couples.

    Assumption: criteriaListsMap is not empty.
    @param MAP: string --> CriteriaList : The map of the parsed YAML file:
                 1) The key is the PF number.
                 2) The data is pointer to criteria list required from this PF.
    @return Error: In case of failure after checking the users configurations.
	        nil: In case of success.

******************************************************************************************************************/
func YamlFileValidation(criteriaListsMap map[string]*CriteriaList) error {
	if len(criteriaListsMap) == 0 {
		PrintStdOut("YamlFileValidation: The criteria lists map from the YAML file is EMPTY!")
		return errors.New("The criteria lists map from the YAML file is EMPTY!")
	}
	//Validation on the criteria lists in the YAML file
	for pf, criteriaListPtr := range criteriaListsMap {
		//Verifying that the user put value in the Device Id field. Device Id is a MUST in each
		//criteria list for ALL experience kits.
		if criteriaListPtr.DeviceId == nil || len(criteriaListPtr.DeviceId) == 0 {
			PrintStdOut("YamlFileValidation: Invalid criteria list for PF: %s Criteria list MUST be with minimum of 1 Device Id!",
				pf)
			return errors.New("there is no device id in criteria list")
		}
		//Verifying that the user did not configure both PTP support and RRU connection on the same PF
		if criteriaListPtr.PTPSupport == true && criteriaListPtr.RRUConnection == true {
			PrintStdOut("YamlFileValidation: %s MUST NOT support both PTP and RRU connection! Those 2 capabilities are mutually exclusive!",
				pf)
			return errors.New("A PF MUST NOT support both PTP and RRU connection! Invalid configurations in PF")
		}
		//Checking the PTP support configuration field
		if criteriaListPtr.PTPSupport == true {
			USE_OF_PTP_SUPPORT_CAPABILITY = true
		}
		//Checking the RRU connection configuration field
		if criteriaListPtr.RRUConnection == true {
			USE_OF_RRU_CONNECTION_CAPABILITY = true
		}
		//Validation on the card affinity PF couple field
		if len(criteriaListPtr.CardAffinityPfCouple) > 0 {
			err := ValidateCardAffinityCouplesInYamlFile(criteriaListsMap, criteriaListPtr, pf)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

/****************************************************************************************************************
    @brief This function uses the yaml package to parse the YAML configuration file. It uses UnmarshalStrict
           API in order to validate the user input: Any fields that are found in the data that do not have
           corresponding struct members, or mapping keys that are duplicates, will result in an error.
           The function will update 2 issues for the application:
           1) Criteria lists: It will output a map of the criteria lists. The function will validate
              existence of device id field AFTER the unmarshalling action in each criteria list in the map.
           2) It updates global variables with configurations values from the YAML file.

    @return MAP: string --> CriteriaList :
                 1) The key is the PF number. When there will be N PF's
                    'pf1' will be with the highest priority while 'pfN' will be with the lowest priority.
                 2) The data is pointer to the criteria list required from this PF.
            Error : In case of failure to unmarshal the content of the YAML file
                    (either configurations or criteria lists)

******************************************************************************************************************/
func ParseConfigurationFile() (map[string]*CriteriaList, error) {
	//Read the configuration YAML file
	yamlFile, err := ioutil.ReadFile(CONFIGURATION_FILE)
	if err != nil {
		PrintStdOut("ParseConfigurationFile: Error in ReadFile function: %v ", err)
		return nil, err
	}
	//Parse the criteria lists and the configurations from the yaml file
	yamlConfigurationsData := YamlConfigurationsData{}
	err = yaml.UnmarshalStrict(yamlFile, &yamlConfigurationsData)
	if err != nil || len(yamlConfigurationsData.CriteriaListsMap) == 0 {
		PrintStdOut("ParseConfigurationFile: UnmarshalStrict function for Criteria Lists failed: %v", err)
		return nil, err
	}
	//Validate the YAML file parsing result
	err = YamlFileValidation(yamlConfigurationsData.CriteriaListsMap)
	if err != nil {
		return nil, err
	}
	//Update global variables
	NUMBER_OF_DEVICES_REQUIRED = uint8(len(yamlConfigurationsData.CriteriaListsMap))
	//Updating the configuration values from the YAML file
	TIMEOUT_FOR_FIND_PTP_MASTER = yamlConfigurationsData.Configurations.TimeoutForFindPTPMaster

	return yamlConfigurationsData.CriteriaListsMap, nil
}

/****************************************************************************************************************
    @brief Creates the output string of the application.

    @param [in] InterfaceIndexToAdd - byte : The interface index to add to the output string
    @param [in] deviceInterfaceToAdd - string : The device interface value to add to the output string
    @param [out] outputStringPtr - pointer to string : pointer to the output string of the application.
                                   A pointer to an empty string in case of invalid interface index.

******************************************************************************************************************/
func UpdateOutputString(interfaceIndexToAdd uint8, deviceInterfaceToAdd string, outputStringPtr *string) {
	if interfaceIndexToAdd > 0 && interfaceIndexToAdd <= NUMBER_OF_DEVICES_REQUIRED {
		*outputStringPtr += deviceInterfaceToAdd + "\n"
		PrintStdOut("Added NIC: %s as priority: pf%d to the final output result of the application",
			deviceInterfaceToAdd, interfaceIndexToAdd)
	} else {
		*outputStringPtr = ""
	}
}

/****************************************************************************************************************
    @brief Helper function: Searches for device capability in capabilities criteria list.

    @param Slice of strings: Capabilities Criteria List
    @param String : Device Capability to find in the list
    @param String : The Device Interface - for printings
    @param String : The Capability Type to find - for printings
    @return true in case of finding the capability in the criteria list and false otherwise.

******************************************************************************************************************/
func SearchCapabilityInCriteriaList(capabilityCriteriaList []string, deviceCapability string, deviceInterface string, capabilityType string) bool {
	capabilityFound := false
	//Checking if the device capability is one of the capabilities in the criteria list
	for _, capabilityInCriteriaList := range capabilityCriteriaList {
		if capabilityInCriteriaList == deviceCapability {
			capabilityFound = true
			break
		}
	}
	if capabilityFound == false {
		PrintStdOut("The NIC with interface: %s has %s : %s . So it is not fit for specific PF in a criteria list",
			deviceInterface, capabilityType, deviceCapability)
	}
	return capabilityFound
}

/****************************************************************************************************************
    @brief This function pass through on all the available Intels NICs with device id,
	       bus info and support for SR-IOV. For each one of them the function compares the NIC
           capabilities against the capabilities in a specific criteria list.
    Assumption: criteriaListsMap and devicesInfoSlice are not empty.
    @param Criteria Lists Map: keys: The PF's (string) from the configurations YAML file.
	                           data: Criteria List of capabilities
    @param A slice of devices structs. Each struct contain the capabilities of the device.
    @param Integer : The PF index of specific criteria list, from the criteria lists map.
    @return Integer : In case of full match the function will return the index of the device in the devices slice.
                      -1 - Invalid slice index in case of failure.

******************************************************************************************************************/
func CheckDevicesAgainstSpecificCriteriaList(criteriaListPtr *CriteriaList, devicesInfoSlice []SupDevice) int {
	//Checking each detected device against the criteria list in criteriaListPtr
	for deviceIndex, device := range devicesInfoSlice {
		//If this detected NIC already has chosen to another use case functionality(FH/PTP/RRU etc...) or
		//we need this device as couple of card affinity to another device --> we will skip it
		if device.CheckedAgainstCriteria == true || device.IsCardAffinityCouple == true {
			continue
		}
		//Device Id check
		if len(criteriaListPtr.DeviceId) > 0 && len(device.DeviceId) > 0 &&
			SearchCapabilityInCriteriaList(criteriaListPtr.DeviceId, device.DeviceId, device.InterfaceName, "Device ID") == false {
			continue
		}
		//Driver check
		if len(criteriaListPtr.Driver) > 0 && len(device.Driver) > 0 &&
			SearchCapabilityInCriteriaList(criteriaListPtr.Driver, device.Driver, device.InterfaceName, "Driver") == false {
			continue
		}
		//Link Speed check
		if criteriaListPtr.LinkSpeed > 0 && device.LinkSpeed < criteriaListPtr.LinkSpeed {
			PrintStdOut("The NIC with interface: %s support maximum link speed of: %d Mb/s, while the requirment is: %d Mb/s, So it is not fit for the input EK to use for SR-IOV",
				device.InterfaceName, device.LinkSpeed, criteriaListPtr.LinkSpeed)
			continue
		}
		//Number of VFs support check
		if criteriaListPtr.NumVFSupp > 0 && device.NumVFSupp < criteriaListPtr.NumVFSupp {
			PrintStdOut("The NIC with interface: %s supports maximum number of VF's: %d, while the requirment is: %d So it is not fit for the input EK to use for SR-IOV",
				device.InterfaceName, device.NumVFSupp, criteriaListPtr.NumVFSupp)
			continue
		}
		//Link State check
		if criteriaListPtr.LinkState != LINK_STATE_INVALID_VALUE && device.LinkState != criteriaListPtr.LinkState {
			PrintStdOut("The NIC with interface: %s supports the link state %d, while the requirment is: %d So it is not fit for the input EK to use for SR-IOV",
				device.InterfaceName, device.LinkState, criteriaListPtr.LinkState)
			continue
		}
		//NUMA Location check
		if criteriaListPtr.NUMALocation > INVALID_VALUE_FOR_NUMA_NODE && device.NUMALocation != criteriaListPtr.NUMALocation {
			PrintStdOut("The NIC with interface: %s supports NUMA location of: %d, while the requirment is: %d So it is not fit for the input EK to use for SR-IOV",
				device.InterfaceName, device.NUMALocation, criteriaListPtr.NUMALocation)
			continue
		}
		//Connector Type check
		if len(criteriaListPtr.ConnectorType) > 0 && len(device.ConnectorType) > 0 &&
			SearchCapabilityInCriteriaList(criteriaListPtr.ConnectorType, device.ConnectorType, device.InterfaceName, "Connector Type") == false {
			continue
		}
		//DDP Support check
		if criteriaListPtr.DDPSupport && device.DDPSupport == false {
			PrintStdOut("The NIC with interface: %s does not support DDP! while the requirment does! So it is not fitting for the input EK to use for SR-IOV",
				device.InterfaceName)
			continue
		}
		//Specific Port Number check
		if criteriaListPtr.SpecificPortNumber > 0 && device.SpecificPortNumber != criteriaListPtr.SpecificPortNumber {
			PrintStdOut("The NIC with interface: %s supports a specific port number: %d, while the requirment is: %d So it is not fit for the input EK to use for SR-IOV",
				device.InterfaceName, device.SpecificPortNumber, criteriaListPtr.SpecificPortNumber)
			continue
		}
		//Full match between 1 device to a criteria list. Returnning the index of the device in the devices slice
		return deviceIndex
	}
	//Failure: From all the detected devices there is no device that met the criteria list of capabilities.
	//Returnning invalid value of index in slice.
	return -1
}

/****************************************************************************************************************
    @brief Helper function for removing an index of device from the devices indexes slice in the
           card affinity map for some card affinity.
           The function will delete the entire card affinity from the map in case of empty slice after the removal.

    @param integer - Device index in the slice (In the card affinity map)
    @param string - Card affinity as the key in the map
	@param MAP: string --> Pointer to Card Affinity Devices Info : The relevant map to remove the device index:
                 1) The key is the card affinity of the devices.
                 2) The value is pointer to the Card Affinity Devices Info struct

******************************************************************************************************************/
func RemoveDeviceIndexFromSlice(deviceIndex int, currentCardAffinity string,
	cardAffinityMap map[string]*CardAffinityDevicesInfo) {
	// Removing the element at deviceIndex from the devices indexes slice for a currentCardAffinity in the map
	//with 2 phases:
	// 1) Copy last element to index deviceIndex.
	cardAffinityMap[currentCardAffinity].DevicesIndexesSlice[deviceIndex] =
		cardAffinityMap[currentCardAffinity].DevicesIndexesSlice[len(cardAffinityMap[currentCardAffinity].DevicesIndexesSlice)-1]
	// 2) Truncate slice.
	cardAffinityMap[currentCardAffinity].DevicesIndexesSlice =
		cardAffinityMap[currentCardAffinity].DevicesIndexesSlice[:len(cardAffinityMap[currentCardAffinity].DevicesIndexesSlice)-1]
	//Deleting the entire card affinity from the map in case of empty slice after the removal
	if len(cardAffinityMap[currentCardAffinity].DevicesIndexesSlice) == 0 {
		delete(cardAffinityMap, currentCardAffinity)
	}
}

/****************************************************************************************************************
    @brief Helper function for checking and selecting devices against configured ctireria lists with card
           affinity capabilities. This function receives a slice of devices indexes. All devices have the
           same card affinity. The function checks each of them until finding a proper NIC for SR-IOV
           against some criteria list.
           In addition the function treat the slice and the criteria lists map according to the input
           card affinity type.

    Assumptions:
	1) criteriaListsMap and devicesInfoSlice are not empty.
	2) currentPF and currentCardAffinity are not empty strings.

    @param CardAffinityPfCoupleType type: The type of card affinity: not exist, exist and with which priority.
    @param Card affintiy Map: keys: The card affinity (string) .
                              data: Pointer to Card Affinity Devices Info.
    @param string: Current Card Affinity: The card affinity that has to all devices in the slice.
    @param Criteria Lists Map: keys: The PF's (string) from the configurations YAML file.
                               data: Criteria List of capabilities
    @param string: Current PF - To select a device for it.
    @param slice of devices
    @return integer: Index of device in the device info slice, in case of success.
                     -1 in case of failure.

******************************************************************************************************************/
func SelectingDeviceFromCardAffinityIndexesSlice(cardAffinityPfCoupleType CardAffinityPfCoupleType,
	cardAffinityMap map[string]*CardAffinityDevicesInfo,
	currentCardAffinity string,
	criteriaListsMap map[string]*CriteriaList,
	currentPF string,
	devicesInfoSlice []SupDevice) int {
	//Passing through on each device in the slice
	for indexInDevicesIndexesSlice, deviceIndex := range cardAffinityMap[currentCardAffinity].DevicesIndexesSlice {
		if cardAffinityPfCoupleType == CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_BIGGER_THAN_COUPLED {
			//In scenario that the coupled PF is smaller than the current PF we will disable this flag
			//This is the stage that we don't need it
			devicesInfoSlice[deviceIndex].IsCardAffinityCouple = false
		}
		coupledDeviceSlice := []SupDevice{devicesInfoSlice[deviceIndex]}
		checkingSpecificDeviceResult := CheckDevicesAgainstSpecificCriteriaList(criteriaListsMap[currentPF], coupledDeviceSlice)
		if checkingSpecificDeviceResult == 0 { //We sent to the function above a slice with only 1 device. result = 0 is success
			switch cardAffinityPfCoupleType {
			case CARD_AFFINITY_PF_COUPLE_TYPE_EMPTY_MARKED_FALSE:
				cardAffinityMap[currentCardAffinity].ChosenDeviceHasNoCardAffinity = true
			case CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_SMALLER_THAN_COUPLED:
				{ //In scenario that the coupled PF is bigger than the current PF we will save the card affinity value in the
					//criteria list map of the coupled and mark a flag in each device in the slice (exept the chosen one)
					//Saving the card affinity VALUE in the criteria list of the coupled PF
					criteriaListsMap[criteriaListsMap[currentPF].CardAffinityPfCouple].CardAffinity =
						devicesInfoSlice[deviceIndex].CardAffinity
					//Marking all the devices with the same card affinity as the chosen above to be saved as couple for other PF
					for _, coupledDeviceIndex := range cardAffinityMap[currentCardAffinity].DevicesIndexesSlice {
						devicesInfoSlice[coupledDeviceIndex].IsCardAffinityCouple = true
					}
					RemoveDeviceIndexFromSlice(indexInDevicesIndexesSlice, currentCardAffinity, cardAffinityMap)
					return int(deviceIndex)
				}
			case CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_BIGGER_THAN_COUPLED:
				devicesInfoSlice[deviceIndex].IsCardAffinityCouple = true
			default: //EMPTY
			}

			RemoveDeviceIndexFromSlice(indexInDevicesIndexesSlice, currentCardAffinity, cardAffinityMap)
			return int(deviceIndex)
		}
		if cardAffinityPfCoupleType == CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_BIGGER_THAN_COUPLED {
			//In scenario that the coupled PF is smaller than the current PF we will enable this flag: The device is not
			//proper for SR-IOV against this specific criteria: But this device is stil need to be part of a group with
			//the same card affinity
			devicesInfoSlice[deviceIndex].IsCardAffinityCouple = true
		}
	}
	return -1
}

/****************************************************************************************************************
    @brief This function will check devices against criteria list with card affinity.
           It will analyse the position and requirements from the PF configurations:
           1) No need for card affinity couple for the current PF.
           2) The coupled PF is bigger than the current PF.
           3) The coupled PF is smaller than the current PF.
           The function will treat each scenario differently in order to maximize the chance to find proper
           device with the requirement of card affinity couples.

    Assumptions:
	1) criteriaListsMap and devicesInfoSlice are not empty.
	2) currentPF is not an empty string

    @param Criteria Lists Map: keys: The PF's (string) from the configurations YAML file.
                               data: Criteria List of capabilities
    @param string: The current PF to find a device for.
    @param A slice of devices structs. Each struct contain the capabilities of the device.
    @param Card affintiy Map: keys: The card affinity (string) .
                              data: Pointer to Card Affinity Devices Info.
    @return integer: Index of device in the device info slice, in case of success.
                     -1 in case of failure.

******************************************************************************************************************/
func CheckDevicesAgainstCriteriaListWithCardAffinity(criteriaListsMap map[string]*CriteriaList,
	currentPF string,
	devicesInfoSlice []SupDevice,
	cardAffinityMap map[string]*CardAffinityDevicesInfo) int {
	if criteriaListsMap[currentPF].CardAffinityPfCouple == "" {
		//Passing through on each key in the card affinity map to search for card affinty that marked before.
		for cardAffinity, cardAffinityDevicesInfo := range cardAffinityMap {
			if cardAffinityDevicesInfo.ChosenDeviceHasNoCardAffinity == true {
				deviceIndex := SelectingDeviceFromCardAffinityIndexesSlice(CARD_AFFINITY_PF_COUPLE_TYPE_EMPTY_MARKED_TRUE,
					cardAffinityMap, cardAffinity, criteriaListsMap, currentPF, devicesInfoSlice)
				if deviceIndex >= 0 {
					return deviceIndex
				}
			}
		}
		//In this scenario there is no need for card affinity. But the last scan of the card affinity map did not reveal any
		//proper device with card affinity that marked as couple. So we will look for a proper device in the map:
		//with enough devices with the same card affinity for the future couple.
		for cardAffinity, cardAffinityDevicesInfo := range cardAffinityMap {
			if len(cardAffinityDevicesInfo.DevicesIndexesSlice) > 1 {
				deviceIndex := SelectingDeviceFromCardAffinityIndexesSlice(CARD_AFFINITY_PF_COUPLE_TYPE_EMPTY_MARKED_FALSE,
					cardAffinityMap, cardAffinity, criteriaListsMap, currentPF, devicesInfoSlice)
				if deviceIndex >= 0 {
					return deviceIndex
				}
			}
		}
	} else if criteriaListsMap[currentPF].CardAffinityPfCouple > currentPF {
		//Scenario that the coupled PF is bigger than the current PF.
		for cardAffinity, cardAffinityDevicesInfo := range cardAffinityMap {
			//Passing through on each key in the card affinity map and checking 2 things:
			//1) The flag of the "no card affinity" is disabled.
			//2) We have enough devices with the same card affinity for the future couple.
			if cardAffinityDevicesInfo.ChosenDeviceHasNoCardAffinity == false && len(cardAffinityDevicesInfo.DevicesIndexesSlice) > 1 {
				deviceIndex := SelectingDeviceFromCardAffinityIndexesSlice(CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_SMALLER_THAN_COUPLED,
					cardAffinityMap, cardAffinity, criteriaListsMap, currentPF, devicesInfoSlice)
				if deviceIndex >= 0 {
					return deviceIndex
				}
			}
		}
		PrintStdOut("Could not find proper device for PF: %s with PF couple: %s",
			currentPF, criteriaListsMap[currentPF].CardAffinityPfCouple)
	} else { //Scenario of: criteriaListsMap[currentPF].CardAffinityPfCouple < currentPF
		//The coupled PF is smaller than the current PF.
		if currentPF == "" || criteriaListsMap[currentPF].CardAffinity == "" {
			PrintStdOut("MISUSE of function CheckDevicesAgainstCriteriaListWithCardAffinity! Empty PF or EMPTY card affinity in criteria lists map!")
			return -1
		}
		deviceIndex := SelectingDeviceFromCardAffinityIndexesSlice(CARD_AFFINITY_PF_COUPLE_TYPE_CURRENT_PF_BIGGER_THAN_COUPLED,
			cardAffinityMap, criteriaListsMap[currentPF].CardAffinity, criteriaListsMap, currentPF, devicesInfoSlice)
		if deviceIndex >= 0 {
			return deviceIndex
		}
		PrintStdOut("Could not find proper device for card affinity couple to PF: %s with card affinity: %s",
			currentPF, criteriaListsMap[currentPF].CardAffinity)
	}
	return -1
}

/****************************************************************************************************************
    @brief This function will check devices against a criteria list with the assumption that there is
           no requirement in the criteria list for card affinity couples.

    Another Assumption:
    criteriaListsMap and devicesInfoSlice are not empty.

    @param Criteria Lists Map: keys: The PF's (string) from the configurations YAML file.
                               data: Criteria List of capabilities
    @param Pointer to unsigned 8 bytes integer: Pointer number of successful checked devices.
    @param Pointer to string: Pointer to the output string of the entire application.
    @param A slice of regular devices structs. Each struct contain the capabilities of the device.
    @param A slice of PTP devices structs. Each struct contain the capabilities of the device that supports PTP.
    @param A slice of RRU devices structs. Each struct contain the capabilities of the device that has RRU connection.
    @return bool : True in case of success. In this case the 2 pointers will be used to update the output of the app.
                   False in case of failure.

******************************************************************************************************************/
func CheckDevicesAgainstCriteriaListWithNoCardAffinity(criteriaListPtr *CriteriaList, outputStringPtr *string,
	successfulCheckedDevicesPtr *uint8,
	regualrDevicesSlice []SupDevice,
	ptpDevicesSlice []SupDevice,
	rruDevicesSlice []SupDevice) bool {
	deviceIndex := -1
	deviceCapabilityType := DEVICE_CAPABILITY_TYPE_REGULAR
	if criteriaListPtr.PTPSupport == true {
		if ptpDevicesSlice == nil || len(ptpDevicesSlice) == 0 {
			PrintStdOut("%s REQUIRED PTP support capability, but there are not enough devices with PTP support in the Linux!",
				criteriaListPtr.Functionality)
			return false
		}
		deviceCapabilityType = DEVICE_CAPABILITY_TYPE_PTP_SUPPORT
		deviceIndex = CheckDevicesAgainstSpecificCriteriaList(criteriaListPtr, ptpDevicesSlice)
	} else if criteriaListPtr.RRUConnection == true {
		if rruDevicesSlice == nil || len(rruDevicesSlice) == 0 {
			PrintStdOut("%s REQUIRED RRU connection capability, but there are not enough devices with RRU connection in the Linux!",
				criteriaListPtr.Functionality)
			return false
		}
		deviceCapabilityType = DEVICE_CAPABILITY_TYPE_RRU_CONNECTION
		deviceIndex = CheckDevicesAgainstSpecificCriteriaList(criteriaListPtr, rruDevicesSlice)
	} else {
		deviceIndex = CheckDevicesAgainstSpecificCriteriaList(criteriaListPtr, regualrDevicesSlice)
	}
	if deviceIndex >= 0 {
		//After finising all the checks against the criteria we will keep this suitable device for SR-IOV from the correlated devices slice
		*successfulCheckedDevicesPtr++
		switch deviceCapabilityType {
		case DEVICE_CAPABILITY_TYPE_REGULAR:
			{ //Choosing this NIC to this functionality
				regualrDevicesSlice[deviceIndex].CheckedAgainstCriteria = true //Verifying not to choose this NIC again
				UpdateOutputString(*successfulCheckedDevicesPtr, regualrDevicesSlice[deviceIndex].InterfaceName, outputStringPtr)
			}
		case DEVICE_CAPABILITY_TYPE_PTP_SUPPORT:
			{ //Choosing this NIC with PTP support to this functionality
				ptpDevicesSlice[deviceIndex].CheckedAgainstCriteria = true //Verifying not to choose this NIC with PTP again
				UpdateOutputString(*successfulCheckedDevicesPtr, ptpDevicesSlice[deviceIndex].InterfaceName, outputStringPtr)
			}
		case DEVICE_CAPABILITY_TYPE_RRU_CONNECTION:
			{ //Choosing this NIC with RRU connection to this functionality
				rruDevicesSlice[deviceIndex].CheckedAgainstCriteria = true //Verifying not to choose this NIC with RRU again
				UpdateOutputString(*successfulCheckedDevicesPtr, rruDevicesSlice[deviceIndex].InterfaceName, outputStringPtr)
			}
		}
		return true
	}
	return false
}

/****************************************************************************************************************
    @brief This function search for proper devices for a PF couple. It searches proper device in specific card
           affinity map against specific criteria list (of the current PF).
           In case of success the function will search for proper device for the coupled PF in different card
           affinity map against different criteria list (of the coupled PF).

    Assumption: 1) devicesSlice, coupledDevicesSlice, criteriaListsMap are not empty.
                2) currentPf is not ""
                3) cardAffinityMapOfCurrentPF and cardAffinityMapOfCoupledPF ARE DIFFERENT
				4) EACH CARD AFFINITY HAS COUPLE ONLY AND NO MORE THAN 2 DEVICES!

    @param Card Affinity Map Of the CURRENT PF: string --> Pointer to Card Affinity Devices Info :
           The relevant map to search the device index:
                 1) The key is the card affinity of the devices.
                 2) The value is pointer to the Card Affinity Devices Info struct
    @param Card Affinity Map Of the COUPLED PF: string --> Pointer to Card Affinity Devices Info :
           The relevant map to search the device index:
                 1) The key is the card affinity of the devices.
                 2) The value is pointer to the Card Affinity Devices Info struct
    @param A slice of devices. In this slice there is all the information on each device for the current PF.
    @param A slice of devices. In this slice there is all the information on each device for the coupled PF
    @param Criteria Lists Map: keys: The PF's (string) from the configurations YAML file.
                               data: Criteria List of capabilities
    @param String: The current PF in the criteria lists map.
    @param Boolean: Required to output a Couple Of Devices - Or ONLY one.
    @return Boolean : In case of success: true
                      false in case of failure.

******************************************************************************************************************/
func FindProperCoupleDevicesFromDifferentCardAffinityMaps(
	cardAffinityMapOfCurrentPF map[string]*CardAffinityDevicesInfo,
	cardAffinityMapOfCoupledPF map[string]*CardAffinityDevicesInfo,
	devicesSlice []SupDevice,
	coupledDevicesSlice []SupDevice,
	criteriaListsMap map[string]*CriteriaList,
	currentPf string,
	requiredCoupleOfDevices bool) bool {
	var deviceIndex int = -1
	var coupledDeviceIndex int = -1
	//First stage: Searching 1 device in the cardAffinityMapOfCurrentPF that has card affinity in the
	//cardAffinityMapOfCoupledPF
	for cardAffinity := range cardAffinityMapOfCurrentPF {
		if len(cardAffinityMapOfCurrentPF[cardAffinity].DevicesIndexesSlice) == 1 {
			//Checking if the card affinity exists in the cardAffinityMapOfCoupledPF
			if _, foundCardAffinityInMap := cardAffinityMapOfCoupledPF[cardAffinity]; foundCardAffinityInMap {
				//Found 2 potential devices with the same card affinity: Checking if the 2 of them are met
				//With the user configurations in the YAML file
				oneDeviceInSlice := []SupDevice{devicesSlice[cardAffinityMapOfCurrentPF[cardAffinity].DevicesIndexesSlice[0]]}
				deviceIndex = CheckDevicesAgainstSpecificCriteriaList(criteriaListsMap[currentPf], oneDeviceInSlice)
				if deviceIndex == 0 {
					if requiredCoupleOfDevices == true { //The Requirement is 2 devices: For current PF and its coupled PF.
						oneDeviceInSlice =
							[]SupDevice{coupledDevicesSlice[cardAffinityMapOfCoupledPF[cardAffinity].DevicesIndexesSlice[0]]}
						coupledDeviceIndex = CheckDevicesAgainstSpecificCriteriaList(
							criteriaListsMap[criteriaListsMap[currentPf].CardAffinityPfCouple], oneDeviceInSlice)
						if coupledDeviceIndex == 0 {
							criteriaListsMap[currentPf].SelectedDeviceInterface =
								devicesSlice[cardAffinityMapOfCurrentPF[cardAffinity].DevicesIndexesSlice[0]].InterfaceName
							//After finding the device: It will be removed from the card affinity map and marked in the relevant slice.
							devicesSlice[cardAffinityMapOfCurrentPF[cardAffinity].DevicesIndexesSlice[0]].CheckedAgainstCriteria = true
							RemoveDeviceIndexFromSlice(0, cardAffinity, cardAffinityMapOfCurrentPF) //The length of the slice is 1
							//After finding the coupled device: It will be removed from the coupled card affinity map and marked in the relevant slice.
							coupledDevicesSlice[cardAffinityMapOfCoupledPF[cardAffinity].DevicesIndexesSlice[0]].CheckedAgainstCriteria = true
							criteriaListsMap[criteriaListsMap[currentPf].CardAffinityPfCouple].SelectedDeviceInterface =
								coupledDevicesSlice[cardAffinityMapOfCoupledPF[cardAffinity].DevicesIndexesSlice[0]].InterfaceName
							RemoveDeviceIndexFromSlice(0, cardAffinity, cardAffinityMapOfCoupledPF) //The length of the slice is 1
							return true
						}
					} else { //The Requirement is only for 1 device. We will mark the couple in the coupled caed affinity map.
						criteriaListsMap[currentPf].SelectedDeviceInterface =
							devicesSlice[cardAffinityMapOfCurrentPF[cardAffinity].DevicesIndexesSlice[0]].InterfaceName
						//After finding the device: It will be removed from the card affinity map and marked in the relevant slice.
						devicesSlice[cardAffinityMapOfCurrentPF[cardAffinity].DevicesIndexesSlice[0]].CheckedAgainstCriteria = true
						RemoveDeviceIndexFromSlice(0, cardAffinity, cardAffinityMapOfCurrentPF)
						//Marking the other device in the other card affinity map
						cardAffinityMapOfCoupledPF[cardAffinity].ChosenDeviceHasNoCardAffinity = true
						return true
					}
				}
			}
		}
	}
	return false
}

/****************************************************************************************************************
    @brief This function searches for a proper devices for a PF card affinity couple in the SAME DEVICES SLICE.
           The function searches against criteria lists with PF card affinity couple and will use
           the relevant card affintiy map.

    Assumption: criteriaListsMap, devicesSlice are not empty and the currentPf is not "" .

    @param MAP: string --> Pointer to Card Affinity Devices Info : The relevant map to search the device index:
                 1) The key is the card affinity of the devices.
                 2) The value is pointer to the Card Affinity Devices Info struct
    @param String: The current PF in the criteria lists map.
    @param A slice of relevant devices (with the relevant special capabilities).
           In this slice there is all the information on each device.
    @param Criteria Lists Map: keys: The PF's (string) from the configurations YAML file.
                               data: Criteria List of capabilitie
    @return Boolean : In case of success: Finding couple of proper devices to the card affinity couple:
                      return true
                      false in case of failure.

******************************************************************************************************************/
func FindDevicesForPfCoupleWithSpecialCapability(cardAffinityMap map[string]*CardAffinityDevicesInfo, currentPf string,
	devicesSlice []SupDevice, criteriaListsMap map[string]*CriteriaList) bool {
	//First stage: Analyze if the PF, with special capability requirement, is greater from its couple or smaller
	coupledPf := criteriaListsMap[currentPf].CardAffinityPfCouple
	if currentPf > criteriaListsMap[currentPf].CardAffinityPfCouple {
		//In this scenario we Need to "switch" between them in order to
		// use correctly of the CheckDevicesAgainstCriteriaListWithCardAffinity API
		coupledPf = currentPf
		currentPf = criteriaListsMap[currentPf].CardAffinityPfCouple //currentPf is a copy of the string
	}
	deviceIndex := CheckDevicesAgainstCriteriaListWithCardAffinity(criteriaListsMap, currentPf,
		devicesSlice, cardAffinityMap)
	if deviceIndex >= 0 {
		coupledDeviceIndex := CheckDevicesAgainstCriteriaListWithCardAffinity(criteriaListsMap, coupledPf, devicesSlice,
			cardAffinityMap)
		if coupledDeviceIndex >= 0 {
			criteriaListsMap[currentPf].SelectedDeviceInterface =
				devicesSlice[deviceIndex].InterfaceName
			criteriaListsMap[coupledPf].SelectedDeviceInterface =
				devicesSlice[coupledDeviceIndex].InterfaceName
			return true
		}
		PrintStdOut("Could not find a proper device to a PF couple %s, With special capability",
			coupledPf)
	} else {
		PrintStdOut("Could not find a proper device to a PF %s, With special capability", currentPf)
	}
	return false
}

/****************************************************************************************************************
    @brief This function will find a proper device for a PF card affinity couple with
           PTP support and / or RRU connection according to the follow scenarios:
           1) The current PF requires PTP support or RRU connection, But there is NO requirement
              for card affinity couple. Since the entire list of PF's requires card affinity couples
              we will mark the couple device
              (if there is one and if it  is the only device in the slice of the card affinity map).
           2) The current PF requires RRU connection and the PF couple requires RRU connection too.
           3) The current PF requires RRU connection and the PF couple requires PTP support.
           4) The current PF requires PTP support and the PF couple requires PTP support.
           5) The current PF requires PTP support or RRU connection and there is a requirement for
              card affinity PF couple, But the PF couple does NOT require neither PTP support nor
              RRU connection.
           When the function need to find couples proper devices for both PF's it will:
           1) Use the card affinity maps for PTP, RRU and regular devices.
           2) Try to search a couple of devices: 1 with special capability and the other regular.
              Afterwards 1 with special capabilities and the other with the other special capability.
              Only if those 2 attemps will fail the function will search 2 proper devices with
              the same special capability.

    Assumptions:
    1) specificCapabilityDevicesSlice, criteriaListsMap, sortedCriteriaListsKeys and the
       targetCapabilityCardAffinityMap are not empty.
    2) There is at list 1 card affinity couple in the criteriaListsMap.
    3) deviceCapabilityType is not DEVICE_CAPABILITY_TYPE_REGULAR
    4) If deviceCapabilityType will be DEVICE_CAPABILITY_TYPE_RRU_CONNECTION there will be at least 1 PF that
	   require RRU Connection.
    5) If deviceCapabilityType will be DEVICE_CAPABILITY_TYPE_PTP_SUPPORT there will be at least 1 PF that
	   require PTP Support.
    6) A device that has RRU connection will never support PTP and vise versa.
    7) In case of both requirements of RRU connection and PTP support the code of the RRU checking will be FIRST.


    @param A slice of strings: Slice of sorted PF's from the configured criteria lists.
    @param Criteria Lists Map: keys: The PF's (string) from the configurations YAML file.
                               data: Criteria List of capabilitie
    @param Target Capability CardA ffinity Map: keys: The card affinity (string).
                                                data: pointer to the card affinity devices info
    @param Other Capability Card Affinity Map: keys: The card affinity (string).
                                               data: pointer to the card affinity devices info
    @param A slice of regualar devices: Slice of devices with no special capabilities.
    @param A slice of devices with a special capability - The target capability
    @param A slice of devices with a special capability - The other capability
    @param Device Capability Type - IOTA of the possiblities of special device capabilities.
    @return Boolean : True in case of finding the proper devices.
                      False in case of failure.

******************************************************************************************************************/
func TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys []string,
	criteriaListsMap map[string]*CriteriaList,
	targetCapabilityCardAffinityMap map[string]*CardAffinityDevicesInfo,
	otherCapabilityCardAffinityMap map[string]*CardAffinityDevicesInfo,
	regularDevicesSlice []SupDevice,
	targetCapabilityDevicesSlice []SupDevice,
	otherCapabilityDevicesSlice []SupDevice,
	deviceCapabilityType DeviceCapabilityType) bool {
	//Initializing a string for debug printings
	specialCapabilityStr := ""
	if deviceCapabilityType == DEVICE_CAPABILITY_TYPE_RRU_CONNECTION {
		specialCapabilityStr = "RRU Connection"
	} else {
		specialCapabilityStr = "PTP support"
	}
	for _, pfIndex := range sortedCriteriaListsKeys {
		if criteriaListsMap[pfIndex].SelectedDeviceInterface != "" {
			continue
		}
		//Treating ONLY the PF's with requirement for RRU connection / PTP support
		if (deviceCapabilityType == DEVICE_CAPABILITY_TYPE_RRU_CONNECTION && criteriaListsMap[pfIndex].RRUConnection == true) ||
			(deviceCapabilityType == DEVICE_CAPABILITY_TYPE_PTP_SUPPORT && criteriaListsMap[pfIndex].PTPSupport == true) {
			if criteriaListsMap[pfIndex].CardAffinityPfCouple == "" {
				//The current PF has NO requirement for card affinity couple
				if FindProperCoupleDevicesFromDifferentCardAffinityMaps(targetCapabilityCardAffinityMap, CardAffinityMap,
					targetCapabilityDevicesSlice, regularDevicesSlice, criteriaListsMap, pfIndex, false) == false &&
					FindProperCoupleDevicesFromDifferentCardAffinityMaps(targetCapabilityCardAffinityMap, otherCapabilityCardAffinityMap,
						targetCapabilityDevicesSlice, otherCapabilityDevicesSlice, criteriaListsMap, pfIndex, false) == false {
					PrintStdOut("Could NOT find single (1 in key in card affinity map) proper device WITH %s for %s !\n",
						specialCapabilityStr, pfIndex)
					deviceIndex := CheckDevicesAgainstSpecificCriteriaList(criteriaListsMap[pfIndex],
						targetCapabilityDevicesSlice)
					if deviceIndex >= 0 {
						//After finding the device: It will be removed from the card affinity map and marked in the relevant slice.
						//Passing through on each device in the slice inorder to find the index to remove
						for indexInDevicesIndexesSlice, deviceIndexInSlice := range targetCapabilityCardAffinityMap[targetCapabilityDevicesSlice[deviceIndex].CardAffinity].DevicesIndexesSlice {
							if int(deviceIndexInSlice) == deviceIndex {
								criteriaListsMap[pfIndex].SelectedDeviceInterface =
									targetCapabilityDevicesSlice[deviceIndex].InterfaceName
								RemoveDeviceIndexFromSlice(indexInDevicesIndexesSlice,
									targetCapabilityDevicesSlice[deviceIndex].CardAffinity, targetCapabilityCardAffinityMap)
								targetCapabilityDevicesSlice[deviceIndex].CheckedAgainstCriteria = true
								break
							}
						}
						if targetCapabilityDevicesSlice[deviceIndex].CheckedAgainstCriteria == true {
							continue
						}
					}
				} else {
					continue
				}
			} else {
				//Assumption: CardAffinityPfCouple fields already had tested as valid PF in the criteria lists map
				//Treating the couple PF of pfIndex: Checking if this PF couple requires capabilities also
				//The current PF has a requirement for a card affinity couple And special capability (RRU / PTP)
				if (deviceCapabilityType == DEVICE_CAPABILITY_TYPE_RRU_CONNECTION &&
					criteriaListsMap[criteriaListsMap[pfIndex].CardAffinityPfCouple].RRUConnection == true) ||
					(deviceCapabilityType == DEVICE_CAPABILITY_TYPE_PTP_SUPPORT &&
						criteriaListsMap[criteriaListsMap[pfIndex].CardAffinityPfCouple].PTPSupport == true) {
					//The current PF requires RRU connection and the PF couple requires RRU connection too
					if FindDevicesForPfCoupleWithSpecialCapability(targetCapabilityCardAffinityMap,
						pfIndex, targetCapabilityDevicesSlice, criteriaListsMap) == true {
						continue
					}
				} else if deviceCapabilityType == DEVICE_CAPABILITY_TYPE_RRU_CONNECTION &&
					criteriaListsMap[criteriaListsMap[pfIndex].CardAffinityPfCouple].PTPSupport == true {
					//The current PF has a requirement for RRU connection But the couple PF requires PTP support
					if FindProperCoupleDevicesFromDifferentCardAffinityMaps(targetCapabilityCardAffinityMap, otherCapabilityCardAffinityMap,
						targetCapabilityDevicesSlice, otherCapabilityDevicesSlice, criteriaListsMap, pfIndex, true) == true {
						continue
					}
				} else {
					//The PF couple does NOT require neither PTP support nor RRU connection.
					if FindProperCoupleDevicesFromDifferentCardAffinityMaps(targetCapabilityCardAffinityMap, CardAffinityMap,
						targetCapabilityDevicesSlice, regularDevicesSlice, criteriaListsMap, pfIndex, true) == false &&
						FindProperCoupleDevicesFromDifferentCardAffinityMaps(targetCapabilityCardAffinityMap, otherCapabilityCardAffinityMap,
							targetCapabilityDevicesSlice, otherCapabilityDevicesSlice, criteriaListsMap, pfIndex, true) == false {
						PrintStdOut("Could NOT find proper device WITH %s and a couple device WITHOUT %s for %s !\n",
							specialCapabilityStr, specialCapabilityStr, pfIndex)
						if FindDevicesForPfCoupleWithSpecialCapability(targetCapabilityCardAffinityMap,
							pfIndex, targetCapabilityDevicesSlice, criteriaListsMap) == true {
							continue
						}
					} else {
						continue
					}
				}
			}
			return false
		}
	}
	return true
}

/****************************************************************************************************************
    @brief This function will sort the input Criteria Lists Map by the keys: the PF's.
           Afterwards it will pass through on all the available Intels NICs with device id,
           bus info and support for SR-IOV. For each one of them the function compares the NIC
           capabilities against the capabilities in the criteria list.
           - In case of full match the function will save the device interface in the output string.
           - In case of finding enough (According to the number of configured PF's in the configuration YAML file)
             suitable NIC's the function will stop and finish.
           The function divided to 4 stages:
           1) Sorting the PF's in the criteria lists map.
           2) Treatment for scenario with combinations of card affinity couples with PTP support and RRU connection.
           3) Treatment for scenario of card affinity couples only.
           4) Treatment for scenario with NO: card affinity, PTP support, RRU connection.
    Assumptions:
	1) criteriaListsMap are not empty.
	2) The criteria lists Map may be unsorted by the PF's (map keys)
    @param Criteria Lists Map: keys: The PF's (string) from the configurations YAML file.
                               data: Criteria List of capabilities
    @param A slice of regular devices structs. Each struct contain the capabilities of the device.
    @param A slice of PTP devices structs. Each struct contain the capabilities of the device that supports PTP.
    @param A slice of RRU devices structs. Each struct contain the capabilities of the device that has RRU connection.
    @return String : the Output string of the application. Empty string in case of failure.

******************************************************************************************************************/
func CheckDevicesAgainstCriteria(criteriaListsMap map[string]*CriteriaList,
	regularDevicesSlice []SupDevice,
	ptpDevicesSlice []SupDevice,
	rruDevicesSlice []SupDevice) string {
	//Sorting the criteria lists Map by the PF's (map keys)
	sortedCriteriaListsKeys := make([]string, 0, len(criteriaListsMap))
	for pf := range criteriaListsMap {
		sortedCriteriaListsKeys = append(sortedCriteriaListsKeys, pf)
	}
	sort.Strings(sortedCriteriaListsKeys)
	//In case of requirement of card affinity couples: There will be a check of requirements for combinations
	//of PTP support and RRU connection with the card affinity couples
	if USE_OF_CARD_AFFINITY_CAPABILITY == true {
		if USE_OF_RRU_CONNECTION_CAPABILITY == true {
			//Checking combinations of RRU connection with card affinity couples + PTP support
			if TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys,
				criteriaListsMap, RruCardAffinityMap, PtpCardAffinityMap, regularDevicesSlice,
				rruDevicesSlice, ptpDevicesSlice, DEVICE_CAPABILITY_TYPE_RRU_CONNECTION) == false {
				PrintStdOut("CheckDevicesAgainstCriteria: Could NOT find device with RRU Connection AND card affinity couple in the file: %s.",
					CONFIGURATION_FILE)
				return ""
			}
		}
		if USE_OF_PTP_SUPPORT_CAPABILITY == true {
			//In this stage RRU connection combinations have already checked.
			//Checking combinations of PTP support with card affinity couples.
			if TreatmentOFCardAffinityWithCapabilities(sortedCriteriaListsKeys,
				criteriaListsMap, PtpCardAffinityMap, RruCardAffinityMap, regularDevicesSlice,
				ptpDevicesSlice, rruDevicesSlice, DEVICE_CAPABILITY_TYPE_PTP_SUPPORT) == false {
				PrintStdOut("CheckDevicesAgainstCriteria: Could NOT find device with PTP Support AND card affinity couple in the file: %s.",
					CONFIGURATION_FILE)
				return ""
			}
		}
	}
	var successfulCheckedDevices uint8 = 0
	outputString := ""
	properDeviceFound := false //Helper flag
	for _, pfIndex := range sortedCriteriaListsKeys {
		//Assumption: Before this stage all the PF's required RRU / PTP with card affinity already have checked and selected
		if USE_OF_CARD_AFFINITY_CAPABILITY == true {
			if criteriaListsMap[pfIndex].SelectedDeviceInterface == "" {
				deviceIndex := CheckDevicesAgainstCriteriaListWithCardAffinity(criteriaListsMap, pfIndex,
					regularDevicesSlice, CardAffinityMap)
				if deviceIndex >= 0 {
					properDeviceFound = true
					successfulCheckedDevices++
					UpdateOutputString(successfulCheckedDevices, regularDevicesSlice[deviceIndex].InterfaceName, &outputString)
				}
			} else {
				properDeviceFound = true
				successfulCheckedDevices++
				UpdateOutputString(successfulCheckedDevices, criteriaListsMap[pfIndex].SelectedDeviceInterface, &outputString)
			}
		} else {
			properDeviceFound = CheckDevicesAgainstCriteriaListWithNoCardAffinity(criteriaListsMap[pfIndex], &outputString,
				&successfulCheckedDevices, regularDevicesSlice, ptpDevicesSlice, rruDevicesSlice)
		}
		if properDeviceFound == true {
			if successfulCheckedDevices == NUMBER_OF_DEVICES_REQUIRED {
				return outputString
			}
			properDeviceFound = false //For finding the next device for the next PF
			continue
		}
		//In this scenario the application could not find proper NIC for a specific criteria list of capabilities
		PrintStdOut("CheckDevicesAgainstCriteria: Could NOT find proper device for %s in the file: %s. There are insufficient NIC's that fit for the input EK to use for SR-IOV!",
			pfIndex, CONFIGURATION_FILE)
		return ""
	}
	//Should never reach this scenario: input of EMPTY map of criteria lists
	PrintStdOut("CheckDevicesAgainstCriteria: INVALID YAML file configurations: There is no criteria list of capabilities to check for this EK!")
	return ""
}

/****************************************************************************************************************
    @brief Find if network device is UP and have IPV4 address.

    @param Interface struct from net package for a network device in the Linux.
    @return true in case of :
            1) Finding that the device is UP and has valid IPV4 address.
            2) The device is UP but the API has a problem with the IP address.

            false otherwise

******************************************************************************************************************/
func CheckIfDeviceCurrentlyUsed(networkDevice net.Interface) bool {

	if networkDevice.Flags&net.FlagUp > 0 {
		addrs, err := networkDevice.Addrs()
		if err == nil && addrs != nil {
			for _, ipAddres := range addrs {
				PrintStdOut("Device with interface %s has IP address: %v\n", networkDevice.Name, ipAddres)
				ipNet, ok := ipAddres.(*net.IPNet)
				if ok && ipNet.IP.To4() != nil {
					PrintStdOut("Device with interface: %s is UP and have correct IPV4 address. We will not use it for SRIOV\n",
						networkDevice.Name)
					return true
				}
			}
		}
		if err != nil {
			PrintStdOut("Device with interface: %s has error in finding IP address: %v. We will not use it for SRIOV\n", networkDevice.Name, err)
			//The device is UP but the API failed and there is an error - We can't conclude that the device is not currently used.
			return true
		}
	}
	return false
}

/****************************************************************************************************************
    @brief Finds:
	       1) The Device Id of a network device in the Linux
		   2) The vendor name of a network device in the Linux

    @param A pointer to PCIInfo in GHW package
    @param A string : The bus information of the device.
    @return true and the device id in case of finding:
            1) Valid device id
            2) The vendor of the device is Intel only.

            false and empty string otherwise.

******************************************************************************************************************/
func CheckVendorNameAndProductId(ptrPci *ghw.PCIInfo, busInfo string) (bool, string) {
	deviceInfo := ptrPci.GetDevice(busInfo)
	if deviceInfo == nil {
		PrintStdOut("CheckVendorNameAndProductId: could not retrieve PCI device information for bus info: %s\n", busInfo)
		return false, ""
	}
	//PCI products are often referred to by their "device ID".
	//We use the term "product ID" in ghw because it more accurately reflects what the identifier is for
	//a specific product line produced by the vendor.
	product := deviceInfo.Product
	if len(product.ID) == 0 {
		PrintStdOut("CheckVendorNameAndProductId: Device with bus info: %s has not Product Id! We will not use it for SR-IOV!\n",
			busInfo)
		return false, ""
	}
	vendor := deviceInfo.Vendor
	if vendor.Name != INTEL_VENDOR_NAME {
		PrintStdOut("CheckVendorNameAndProductId: Device with bus info: %s has vendor: %s which is not Intel! We will not use it for SR-IOV!\n",
			busInfo, vendor.Name)
		return false, ""
	}
	return true, product.ID
}

/****************************************************************************************************************
    @brief Finds the capability that a device can / cant not support.
           It uses the cat command on the file in Linux:
           /sys/class/net/[Device Interface]/device/[File with integer value for capability].
           In case that this file is empty or has invalid value we conclude that this device does not support
           the capability.
    THIS FUNCTION CAN RUN ONLY IN LINUX OS
    @param String: The Device Interface
	@param String: The Capability file to check in the Linux FS.
    @return Integer : The number in the file: the result of the Linux cat command.
                      -1 in case of failure.

******************************************************************************************************************/
func DetectCapabilityFromLinuxFS(deviceInterface string, fileToExtractCapability string) int {
	catCmd := "cat /sys/class/net/" + deviceInterface + "/device/" + fileToExtractCapability
	capabilityValueByte, err := exec.Command("bash", "-c", catCmd).CombinedOutput()
	fileFullPath := "/sys/class/net/" + deviceInterface + "/device/" + fileToExtractCapability
	if err != nil {
		PrintStdOut("DetectCapabilityFromLinuxFS: Device with interface name %s has cat command error : %s on file: %s\n",
			deviceInterface, err, fileFullPath)
		return INVALID_VALUE_FOR_CAPABILITY_FROM_LINUX_FS
	}
	PrintStdOut("The cat command result for device interface: %s, on file: %s, is : %s\n",
		deviceInterface, fileFullPath, capabilityValueByte)
	//Slicing the '\n' from the result of the Linux cat command with strings library
	capabilityStr := strings.TrimSpace(string(capabilityValueByte))
	capabilityInt, err := strconv.Atoi(capabilityStr)
	if err != nil {
		PrintStdOut("DetectCapabilityFromLinuxFS: Device with interface name %s has strconv.Atoi command error : %s for capability of: %s\n",
			deviceInterface, err, fileToExtractCapability)
		return INVALID_VALUE_FOR_CAPABILITY_FROM_LINUX_FS
	}
	return capabilityInt
}

/****************************************************************************************************************
    @brief Finds the driver name of NIC in Linux

    @param A pointer to handler of Ethtool package
    @param string : Network Device Interface
    @return The driver name as: slice of strings - with only 1 string in it
	        (in order to use it in the detect devices function)
			slice with 1 empty string in case of failure.

******************************************************************************************************************/
func GetDeviceDriver(ptrEthtoolHandle *ethtool.Ethtool, networkDeviceInterface string) string {
	driver, err := ptrEthtoolHandle.DriverName(networkDeviceInterface)
	if err != nil {
		PrintStdOut("GetDeviceDriver: Ethtool package could not find the driver of device with Interface: %s . Error: %v\n",
			networkDeviceInterface, err)
		driver = ""
	}
	return driver
}

/****************************************************************************************************************
    @brief Finds link speed of NIC in Linux

    @param A pointer to handler of Ethtool package
    @param string : Network Device Interface
    @return The link speed as long long integer
	        0 in case of failure.

******************************************************************************************************************/
func GetDeviceLinkSpeed(ptrEthtoolHandle *ethtool.Ethtool, networkDeviceInterface string) uint64 {
	ethToolMap, err := ptrEthtoolHandle.CmdGetMapped(networkDeviceInterface) // ethToolMap is: map[string]uint64
	var linkSpeed uint64 = 0
	if err != nil || ethToolMap == nil {
		PrintStdOut("GetDeviceLinkSpeed: Ethtool package could not find the link speed of device with Interface: %s . Error: %v\n",
			networkDeviceInterface, err)
	} else {
		linkSpeed = ethToolMap["Speed"]
	}
	return linkSpeed
}

/****************************************************************************************************************
    @brief Finds link state of a NIC in Linux

    @param A pointer to handler of Ethtool package
    @param string : Network Device Interface
    @return The link state as iota from the type: LinkStateType with the options of: UP / DOWN / INVALID
	        The function will return: UP (1) or DOWN (0) in case of success and INVALID (2) in case of failure.

******************************************************************************************************************/
func GetDeviceLinkState(ptrEthtoolHandle *ethtool.Ethtool, networkDeviceInterface string) LinkStateType {
	linkState, err := ptrEthtoolHandle.LinkState(networkDeviceInterface)
	if err != nil {
		PrintStdOut("GetDeviceLinkState: Ethtool package could not find the link state of device with Interface: %s . Error: %v\n",
			networkDeviceInterface, err)
		return LINK_STATE_INVALID_VALUE
	}
	switch linkState {
	case 0:
		return LINK_STATE_DOWN
	case 1:
		return LINK_STATE_UP
	default:
		return LINK_STATE_INVALID_VALUE
	}
}

/****************************************************************************************************************
    @brief Finds the Card Affinity of a NIC in Linux
	       The function uses the bus info of the NIC.
		   Usually bus info will be in the convention of: "aaaa:bb:cc.d"
		   From that convention we can conclude that all the NIC's
		   with bus info aaaa:bb:cc.x are from the same physical network
		   card.
		   So, after checking that the bus info is valid we will
		   return the bus info with character "x" instead of the number
		   after the "."
    @param string : The Bus Info of the device
    @param string : Network Device Interface
    @return In case of success: return the bus info with character "x" instead of the number after the "."
	        In case of failure: return empty string

******************************************************************************************************************/
func GetDeviceCardAffinity(deviceBusInfo string, networkDeviceInterface string) string {
	//Checking the input device bus info of the function: The shortest string has to be of the format: "a:b.x"
	// So we check a length of minimum 5 characters and check if there is the character '.'
	if len(deviceBusInfo) < 5 || strings.Contains(deviceBusInfo, ".") == false {
		PrintStdOut("GetDeviceCardAffinity failed to get card affinity to device interface %s with bus info: %s\n",
			networkDeviceInterface, deviceBusInfo)
		return ""
	}
	lastDotIndexInBusInfo := strings.LastIndex(deviceBusInfo, ".")
	if len(deviceBusInfo) != (lastDotIndexInBusInfo + 2) {
		PrintStdOut("GetDeviceCardAffinity failed to get card affinity to device interface %s with bus info: %s ! This Bus Info does not finish with '.[number]' !\n",
			networkDeviceInterface, deviceBusInfo)
		return ""
	}
	return (deviceBusInfo[0:(lastDotIndexInBusInfo+1)] + "x")
}

/****************************************************************************************************************
    @brief This function changes the link state of a NIC in Linux.

    THIS FUNCTION REQUIRES SUDO PERMISSIONS IN THE LINUX!

    @param bool - true if the function needs to change the link state of the NIC to be UP.
                  false if the function needs to change the link state of the NIC to be DOWN.
    @param string : Network Device Interface
    @return true in case of success
	        false in case of failure to execute 1 of the API's.

******************************************************************************************************************/
func ChangeLinkState(setLinkStateUp bool, networkDeviceInterface string) bool {
	//Finding a link by name and getting the pointer to the netlink object.
	networkInterfaceLink, err := netlink.LinkByName(networkDeviceInterface)
	if err != nil {
		PrintStdOut("ChangeLinkState function FAILED for NIC %s !!! Error in API LinkByName: %v",
			networkDeviceInterface, err)
		return false
	}
	if setLinkStateUp == true {
		err = netlink.LinkSetUp(networkInterfaceLink)
		if err != nil {
			PrintStdOut("ChangeLinkState function FAILED for NIC %s !!! Error in API LinkSetUp: %v",
				networkDeviceInterface, err)
			return false
		}
	} else {
		err = netlink.LinkSetDown(networkInterfaceLink)
		if err != nil {
			PrintStdOut("ChangeLinkState function FAILED for NIC %s !!! Error in API LinkSetDown: %v",
				networkDeviceInterface, err)
			return false
		}
	}
	return true
}

/****************************************************************************************************************
    @brief Detects if a network device has physical connection to a PTP server.
           1) In case that the NIC is DOWN: the function will UP it temporary.
           2) The function uses the package linuxptp on Linux: It execute the ptp4l
              process with the network device interface input:
              "sudo ptp4l -i [networkDeviceInterface] -2 -s -m" with parameters:
              i - Which network interface to use.
              2 - Network transport: IEEE 802.3
              s - Specify slave ONLY mode.
              m - Prints the log of the command to the stdout.
           3) It checks the ptp4l process stdout.
           4) In case of existing physical connection of the NIC to PTP master there
              will be 1 unique line in the process stdout: "new foreign master".
           5) Since there is a use of I/O, external infinite process and a use of network
              we will use:
              a) Timeout. Will be implement by GoLang Context.
              b) Concurrence:
                 ** The external infinite ptp4l will execute and be managed from a
                    new GoLang goroutine.
                 **	In order to sync between the 2 goroutines we will use channel and select.
                 ** PREVENTING LIVELOCK / DEADLOCK / GOROUTINE LEAK BY: closing the cmd.StdoutPipe()
                    by the main goroutine which will cause the other goroutine to stop running.
                 ** PREVENTING LIVELOCK / GOROUTINE LEAK BY: forcing the main goroutine to wait
                    for result from the other goroutine (Except the case of timeout) and assure
                    that after the send the result through channel the other goroutine will exit.

    Assumption: networkDeviceInterface is a non empty string with a valid NIC name in the Linux.
    @param string : Network Device Interface
    @param unsigned integer : The link state (UP/DOWN/INVALID) of the relevant device
    @return In case of success: true
	        In case of failure or timeout: false

******************************************************************************************************************/
func GetDevicePtpSupport(networkDeviceInterface string, linkState LinkStateType) bool {
	//In case that the NIC in link state Down: we will temporary Up it
	if linkState == LINK_STATE_DOWN && ChangeLinkState(true, networkDeviceInterface) == false {
		return false
	}
	//Initialization of: ptp4l command to execute, channel and context (For using timer)
	ctx := context.Background()
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Duration(TIMEOUT_FOR_FIND_PTP_MASTER)*time.Millisecond)
	defer cancel()
	cmdStr := "sudo ptp4l -i " + networkDeviceInterface + " -2 -s -m"
	cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
	stdout, _ := cmd.StdoutPipe()
	ptpMasterFoundChannel := make(chan bool)
	defer close(ptpMasterFoundChannel) //Closing the channel ONLY in the end of the function
	byteBuffer := make([]byte, BYTE_BUFFER_SIZE_FOR_PTP_MESSAGES)
	//Openning new goroutine for the long run of the ptp4l process execution
	go func() {
		PrintStdOut("Output of the command: %s is:\n", cmdStr)
		err := cmd.Start()
		if err != nil {
			ptpMasterFoundChannel <- false
			PrintStdOut("Error for function Start: %s . GoRoutine in function GetDevicePtpSupport returned!\n",
				err.Error())
			return
		}
		for {
			_, err = stdout.Read(byteBuffer)
			if err != nil { //Failure to read the ptp4l command std out: usually from the main goroutine
				ptpMasterFoundChannel <- false
				PrintStdOut("Error for function Read: %s . GoRoutine in function GetDevicePtpSupport returned!\n",
					err.Error())
				return
			}
			PrintStdOut(string(byteBuffer))
			if strings.Contains(string(byteBuffer), "new foreign master") == true {
				//Success to find a NIC with connection to PTP master: this NIC supports PTP capability
				ptpMasterFoundChannel <- true
				PrintStdOut("The ptp4l command for device interface: %s found PTP Master! It SUPPORTS PTP\n",
					networkDeviceInterface)
				return
			}
		}
	}()
	//Using select between 2 options (timeout and result) in order to sync between the 2 goroutines
	select {
	case result := <-ptpMasterFoundChannel: //Goroutine returned with some result BEFORE tiemout
		cmd.Process.Kill() //We have result, So We can kill the ptp4l process.
		stdout.Close()
		if linkState == LINK_STATE_DOWN && ChangeLinkState(false, networkDeviceInterface) == false {
			return false
		}
		return result
	case <-ctx.Done(): //Encoutering timeout: We will stop The goroutin and return result from the channel
		//The provided context is used to kill the process (by calling os.Process.Kill)
		PrintStdOut("Getting device %s ptp master connection had Time out! It WON'T support ptp!\n",
			networkDeviceInterface)
		stdout.Close()                    //Will cause the Read function in the goroutine to exit with error
		result := <-ptpMasterFoundChannel //Using the channel to sync between the 2 goroutines
		if linkState == LINK_STATE_DOWN && ChangeLinkState(false, networkDeviceInterface) == false {
			return false
		}
		return result
	}
}

/****************************************************************************************************************
    @brief This function will insert new device index in a card affinity map:
           1) If the entire card affinity (key in the map) is NOT exist the function will create it.
           2) If the card affinity (key in the map) is exist the function will add the input device index to the
              relevant slice in the Card Affinity Devices Info

    @param String - The card affintiy of the device index that needs to insert.
    @param Slice of SupDevice - Devices slice of the device index that needs to insert.
    @param MAP: string --> Pointer to Card Affinity Devices Info : The relevant map to insert the device index:
                 1) The key is the card affinity of the devices.
                 2) The value is pointer to the Card Affinity Devices Info struct

******************************************************************************************************************/
func UpdatingCardAffinityMap(cardAffinity string, devicesSlice []SupDevice,
	cardAffinityMap map[string]*CardAffinityDevicesInfo) {

	if _, foundCardAffinityInMap := cardAffinityMap[cardAffinity]; foundCardAffinityInMap {
		cardAffinityMap[cardAffinity].DevicesIndexesSlice = append(cardAffinityMap[cardAffinity].DevicesIndexesSlice,
			uint8(len(devicesSlice)))
	} else {
		cardAffinityDevicesInfo := CardAffinityDevicesInfo{}
		cardAffinityDevicesInfo.DevicesIndexesSlice = append(cardAffinityDevicesInfo.DevicesIndexesSlice,
			uint8(len(devicesSlice)))
		cardAffinityMap[cardAffinity] = &cardAffinityDevicesInfo
	}
}

/****************************************************************************************************************
    @brief The function filters a network device in the Linux with basic capabilities for SR-IOV:
           1) Device id.
           2) Bus info.
           3) Supports for SR-IOV: Checking the maximum number of virtual functions.
           4) Intel's vendor.
           5) Does not have IPV4 address.

    Assumptions:
    1) The pointer to EthtoolHandle had initialized.
    2) The pointer to PCI had initialized.
    3) The struct of Interface in net package had initialized

    @param A pointer to handler of Ethtool package
    @param A pointer to PCIInfo in GHW package
    @param Interface in net package
    @return Boolean: The result of the filtering for 1 network device:
	                 True in case that the device passed the filters. Also:
            1) String: Bus Info of the device.
            2) Integer: The maximum number of virtual functions that the device can configured.
            3) String: Device Id of the device.
	        In case of failure: The boolean will be False, the integer will be 0 and the strings will be empty.

******************************************************************************************************************/
func FilteringDeviceForBasicCapabilities(ptrEthtoolHandle *ethtool.Ethtool, ptrPci *ghw.PCIInfo,
	networkDevice net.Interface) (result bool, deviceBusInfo string, maxVfs int, deviceId string) {
	//First check: Bus info
	deviceBusInfo, err := ptrEthtoolHandle.BusInfo(networkDevice.Name)
	if err != nil || len(deviceBusInfo) == 0 || deviceBusInfo == "N/A" {
		PrintStdOut("FilteringDeviceForBasicCapabilities: Device with interface %s has no valid bus information. Error : %v\n",
			networkDevice.Name, err)
		return false, "", 0, "" //Returnning failure: result = false , empty deviceBusInfo, 0 maxVfs and empty deviceId
	}
	//Second check: If the device currently used - Up with IPV4 address
	if CheckIfDeviceCurrentlyUsed(networkDevice) == true {
		return false, "", 0, "" //Returnning failure: result = false , empty deviceBusInfo, 0 maxVfs and empty deviceId
	}
	//Third check: SR-IOV support: Check the sriov_totalvfs from the Linux file system :
	// In case of 0 VF's: We will conclude that this device does not support SR-IOV.
	// Otherwise we will save the number of the max VF's in the devices slice.
	maxVfs = DetectCapabilityFromLinuxFS(networkDevice.Name, "sriov_totalvfs")
	if maxVfs <= 0 {
		PrintStdOut("FilteringDeviceForBasicCapabilities: Device with interface: %s has 0 sriov_totalvfs! This device does not support SR-IOV!\n",
			networkDevice.Name)
		return false, "", 0, "" //Returnning failure: result = false , empty deviceBusInfo, 0 maxVfs and empty deviceId
	}
	//Fourth check: Detect Vendor and Device Id
	result, deviceId = CheckVendorNameAndProductId(ptrPci, deviceBusInfo)
	if result == false {
		return false, "", 0, "" //Returnning failure: result = false , empty deviceBusInfo, 0 maxVfs and empty deviceId
	}
	return true, deviceBusInfo, maxVfs, deviceId //Returnning success: result = true
}

/****************************************************************************************************************
    @brief Detects available Intels NICs with device id, bus info and suppoert for SR-IOV.
	       Find additional capabilities for each NIC that passed the filters above.
		   In case of requirement of card affinity coupples the function will create the 3 card affinity maps

    Assumption: The input pointers: ptrEthtoolHandle, ptrPci and networkInterfaces were initialized
                and are not nil.
    @param A pointer to handler of Ethtool package
    @param A pointer to PCIInfo in GHW package
    @param A Slice of Interface in net package
    @return 3 slices of NIC's after detection and filtering with additional capabilities.
            1) Slice of regular devices.
            2) Slice of devices that supports PTP.
            3) Slice of devices that has RRU connection.
	        3 Empty slices in case of failure.

******************************************************************************************************************/
func DetectDevices(ptrEthtoolHandle *ethtool.Ethtool, ptrPci *ghw.PCIInfo,
	networkInterfaces []net.Interface) (regularDevicesSlice []SupDevice, ptpDevicesSlice []SupDevice,
	rruDevicesSlice []SupDevice) {
	for _, networkDevice := range networkInterfaces {
		//First stage: Filtering the devices with basic capabilities for SR-IOV
		filteringResult, deviceBusInfo, maxVfs, deviceId := FilteringDeviceForBasicCapabilities(ptrEthtoolHandle,
			ptrPci, networkDevice)
		if filteringResult == false {
			continue
		}
		//Detect additional capabilities of the network device
		driver := GetDeviceDriver(ptrEthtoolHandle, networkDevice.Name)
		linkSpeed := GetDeviceLinkSpeed(ptrEthtoolHandle, networkDevice.Name)
		linkState := GetDeviceLinkState(ptrEthtoolHandle, networkDevice.Name)
		numaNode := DetectCapabilityFromLinuxFS(networkDevice.Name, "numa_node")
		if numaNode == INVALID_VALUE_FOR_NUMA_NODE {
			PrintStdOut("Probelm with finding the NUMA Node for Device with interface: %s\n", networkDevice.Name)
		}
		cardAffinity := GetDeviceCardAffinity(deviceBusInfo, networkDevice.Name)
		var specificPort int8 = INVALID_VALUE_FOR_SPECIFIC_PORT
		ptpSupport := GetDevicePtpSupport(networkDevice.Name, linkState)
		rruConnection := false //Currently RRU Support detection capability was not supported
		if cardAffinity != "" {
			//The specific port number differentiate between several NIC's from the same physical network card.
			//So this is the number after the "." of the bus info of the device.
			//GetDeviceCardAffinity checks the bus info string of the device. If the bus info is valid we proceed.
			specificPort = int8(deviceBusInfo[len(deviceBusInfo)-1] - '0')
			//Checking the card affinity global variable flag: If it enabled we will create the card affinity map
			if USE_OF_CARD_AFFINITY_CAPABILITY == true {
				if USE_OF_RRU_CONNECTION_CAPABILITY == true && rruConnection == true {
					UpdatingCardAffinityMap(cardAffinity, rruDevicesSlice, RruCardAffinityMap)
				} else if USE_OF_PTP_SUPPORT_CAPABILITY == true && ptpSupport == true {
					UpdatingCardAffinityMap(cardAffinity, ptpDevicesSlice, PtpCardAffinityMap)
				} else { //Regular device - with no PTP support or RRU connection
					UpdatingCardAffinityMap(cardAffinity, regularDevicesSlice, CardAffinityMap)
				}
			}
		}
		supDevice := SupDevice{networkDevice.Name, deviceBusInfo, deviceId, driver, linkSpeed, uint32(maxVfs),
			linkState, int8(numaNode), "", false, ptpSupport, rruConnection, specificPort, cardAffinity, false, false}
		//First checking if the device support RRU connection. If it does and the user required RRU connection:
		//Put this device in another different slice
		if USE_OF_RRU_CONNECTION_CAPABILITY == true && rruConnection == true {
			rruDevicesSlice = append(rruDevicesSlice, supDevice)
			continue
		}
		//Afterwards checking if the device support PTP. If it does and the user required PTP support:
		//Put this device in another different slice
		if USE_OF_PTP_SUPPORT_CAPABILITY == true && ptpSupport == true {
			ptpDevicesSlice = append(ptpDevicesSlice, supDevice)
			continue
		}
		regularDevicesSlice = append(regularDevicesSlice, supDevice)
	}
	return regularDevicesSlice, ptpDevicesSlice, rruDevicesSlice
}

/****************************************************************************************************************
@brief Helper function: Collect initialization of pointers to:
       - Handler of Ethtool package
	   - PCIInfo in GHW package
	   - Slice of Interface in net package

       ** The memory for the Handler of Ethtool package WILL BE FREE IN THE MAIN FUNCTION
@return 2 pointers and 1 slice
        Exit of the enire application in case of failure.

******************************************************************************************************************/
func InitializePackagesPointers() (*ethtool.Ethtool, *ghw.PCIInfo, []net.Interface) {
	//Initialize pointer to EthTool
	ptrEthToolHandle, err := ethtool.NewEthtool()
	if err != nil || ptrEthToolHandle == nil {
		log.Printf("InitializePackagesPointers: New Ethtool failed!\n")
		panic("New Ethtool failed!")
	}

	//Initialize pointer to PCI in the GHW package
	ptrPci, err := ghw.PCI()
	if err != nil || ptrPci == nil {
		log.Print(fmt.Errorf("InitializePackagesPointers: Error getting PCI info: %v", err.Error()))
		panic("Error getting PCI info!")
	}

	//Extracting a slice of all network interfaces from package net
	networkInterfaces, err := net.Interfaces()
	if err != nil || networkInterfaces == nil {
		log.Print(fmt.Errorf("InitializePackagesPointers: Network Interfaces not found : %v\n", err.Error()))
		panic("Network Interfaces not found!")
	}
	return ptrEthToolHandle, ptrPci, networkInterfaces
}

/****************************************************************************************************************
@brief Helper function: Prints logs in debug mode of the card affinity maps and all devices slices
       before the check against the criteria lists section.

    @param A slice of regular devices structs. Each struct contain the capabilities of the device.
    @param A slice of PTP devices structs. Each struct contain the capabilities of the device that supports PTP.
    @param A slice of RRU devices structs. Each struct contain the capabilities of the device that has RRU connection.
******************************************************************************************************************/
func PrintDataStructures(regularDevicesSlice []SupDevice, ptpDevicesSlice []SupDevice, rruDevicesSlice []SupDevice) {
	if DEBUG_MODE == true {
		log.Printf("\n")
		log.Printf("THE DATA BEFORE CHECKING AGAINST CRITERIA: \n")
		log.Printf("THE CARD AFFINITY MAP: \n")
		for cardAffinity, cardAffinityDevicesData := range CardAffinityMap {
			log.Println(cardAffinity, " : ", *cardAffinityDevicesData)
		}
		log.Printf("\n")
		log.Printf("THE PTP CARD AFFINITY MAP: \n")
		for cardAffinity, cardAffinityDevicesData := range PtpCardAffinityMap {
			log.Println(cardAffinity, " : ", *cardAffinityDevicesData)
		}
		log.Printf("\n")
		log.Printf("THE RRU CARD AFFINITY MAP: \n")
		for cardAffinity, cardAffinityDevicesData := range RruCardAffinityMap {
			log.Println(cardAffinity, " : ", *cardAffinityDevicesData)
		}
		log.Printf("\n")
		log.Printf("THE REGULAR DEVICES SLICE: \n")
		for _, networkDevice := range regularDevicesSlice {
			log.Println(networkDevice)
		}
		log.Printf("\n")
		log.Printf("THE PTP DEVICES SLICE: \n")
		for _, networkDevice := range ptpDevicesSlice {
			log.Println(networkDevice)
		}
		log.Printf("\n")
		log.Printf("THE RRU DEVICES SLICE: \n")
		for _, networkDevice := range rruDevicesSlice {
			log.Println(networkDevice)
		}
		log.Printf("\n")
	}
}

/****************************************************************************************************************
@brief Helper function: It checks the input parameters of the application.
       Input arguments MUST be one of those options ONLY:
       "go run sriov_detection.go"
       "go run sriov_detection.go debug_mode"
       "go run sriov_detection.go" [CONFIG YAML FILE]
       "go run sriov_detection.go [CONFIG YAML FILE] debug_mode"

    Assumptions:
                1) The appilication will try to open the YAML file AFTER this function.
                   Meaning: In case of INVALID YAML file there will be another check and error message.
                2) The YAML file parameter is the file name ONLY - neither full path nor relative path.
    @param A slice strings: The argumets from the OS.
******************************************************************************************************************/
func CheckInputParameters(argumentsSlice []string) {
	switch len(argumentsSlice) {
	case 1: //go run sriov_detection.go --> Do Nothing
	case 2: //"go run sriov_detection.go debug_mode or go run sriov_detection.go [CONFIG YAML FILE]"
		{
			if argumentsSlice[1] != "debug_mode" {
				//In this case the input parameter must be a YAML configurations file
				if strings.Contains(argumentsSlice[1], ".yaml") == false &&
					strings.Contains(argumentsSlice[1], ".yml") == false {
					panic("Input parameter MUST be the string: 'debug_mode' OR a YAML file!")
				}
				CONFIGURATION_FILE = argumentsSlice[1]
			} else {
				DEBUG_MODE = true
			}
		}
	case 3: //"go run sriov_detection.go [CONFIG YAML FILE] debug_mode"
		{
			if argumentsSlice[2] == "debug_mode" {
				//In this case the input parameter must be a YAML configurations file
				if strings.Contains(argumentsSlice[1], ".yaml") == false &&
					strings.Contains(argumentsSlice[1], ".yml") == false {
					panic("FIRST Input parameter MUST be a YAML file!")
				}
				DEBUG_MODE = true
				CONFIGURATION_FILE = argumentsSlice[1]
			} else {
				panic("SECOND Input parameter MUST be the string: 'debug_mode' !")
			}
		}
	default:
		{
			panic("Input parameter MUST be: 0 / 1 / 2 strings only!")
		}
	}
}

/*****************************************************************************************
 *                       Main Function
 ****************************************************************************************/
func main() {
	//Checking input parameter:
	CheckInputParameters(os.Args)
	//Getting the criteria list and the configurations from the YAML configuration files
	criteriaListsMap, err := ParseConfigurationFile()
	if err != nil {
		log.Printf("The parsing of the configuration YAML file failed! Error: %v", err)
		panic("The parsing of the configuration YAML file failed!")
	}
	PrintStdOut("THE CRITERIA LISTS ARE: \n")
	for pf, criteriaListptr := range criteriaListsMap {
		PrintStdOut("%s: %v \n", pf, *criteriaListptr)
	}
	PrintStdOut("\n")
	PrintStdOut("STARTING TO DETECT THE NETWORK DEVICES ON THE LINUX: \n")
	//Initialize Packages Pointers and the list of all network interfaces in this Linux
	ptrEthToolHandle, ptrPci, networkInterfaces := InitializePackagesPointers()
	defer ptrEthToolHandle.Close()
	//Detecting all network devices in this Linux
	regularDevicesSlice, ptpDevicesSlice, rruDevicesSlice := DetectDevices(ptrEthToolHandle, ptrPci, networkInterfaces)
	if (regularDevicesSlice == nil || len(regularDevicesSlice) == 0) &&
		(ptpDevicesSlice == nil || len(ptpDevicesSlice) == 0) &&
		(rruDevicesSlice == nil || len(rruDevicesSlice) == 0) {
		panic("There are NO available network devices for SR-IOV!")
	}
	if uint8(len(regularDevicesSlice)+len(ptpDevicesSlice)+len(rruDevicesSlice)) < NUMBER_OF_DEVICES_REQUIRED {
		log.Printf("The requirement from the configuration file for SR-IOV is %d NIC's. In this OS there are only %d NIC's available!",
			NUMBER_OF_DEVICES_REQUIRED, (len(regularDevicesSlice) + len(ptpDevicesSlice) + len(rruDevicesSlice)))
		panic("Can't find enough Network Devices available for SR-IOV!")
	}
	//Printing logs in debug mode of the card affinity maps and all devices slices
	//before the check against the criteria lists section.
	PrintDataStructures(regularDevicesSlice, ptpDevicesSlice, rruDevicesSlice)
	//Comparing the devices in the slice to the SupDevice criteria
	outputInterfacesString := CheckDevicesAgainstCriteria(criteriaListsMap, regularDevicesSlice, ptpDevicesSlice, rruDevicesSlice)
	if outputInterfacesString == "" {
		panic("There are available Intels NICs with device id, bus info and supported SR-IOV. But some / all of them have not met with the input EK criteria!")
	}
	//Output the result of the device detection
	PrintStdOut("OUTPUT STRING : \n")
	//Printing in release mode: the content of the string will output into a file
	fmt.Print(outputInterfacesString)
}

/*****************************************************************************************
 *                                        EOF
 ****************************************************************************************/
