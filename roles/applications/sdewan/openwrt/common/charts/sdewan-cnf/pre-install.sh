#! /bin/bash

# INTEL CONFIDENTIAL
#
# Copyright 2021-2021 Intel Corporation.
#
# This software and the related documents are Intel copyrighted materials, and your use of
# them is governed by the express license under which they were provided to you ("License").
# Unless the License provides otherwise, you may not use, modify, copy, publish, distribute,
# disclose or transmit this software or the related documents without Intel's prior written permission.
#
# This software and the related documents are provided as is, with no express or implied warranties,
# other than those that are expressly stated in the License.

echo "Pre-Installation steps for SDEWAN CNF"

# CNF_NODE variable holds the name of the node where CNF will be deployed
# must be set before running the script 
CNF_NODE=""

if [ -z "$CNF_NODE" ]
then
  echo -e "\tVariables CNF_NODE is not set"
  exit 1
fi

# label the node where SDEWAN CNF will be deployed
function label_cnf_node {
    kubectl label nodes "$CNF_NODE" sdewan=true --overwrite

}

label_cnf_node 

