#!/usr/bin/env python3

# INTEL CONFIDENTIAL
#
# Copyright 2022 Intel Corporation.
#
# This software and the related documents are Intel copyrighted materials, and your use of
# them is governed by the express license under which they were provided to you ("License").
# Unless the License provides otherwise, you may not use, modify, copy, publish, distribute,
# disclose or transmit this software or the related documents without Intel's prior written permission.
#
# This software and the related documents are provided as is, with no express or implied warranties,
# other than those that are expressly stated in the License.

""" Provision Smart Edge Secure Access Service Edge Experience Kits """

import os
import sys

if __name__ == "__main__":
    sys.path.insert(
        1, os.path.join(os.path.dirname(os.path.realpath(__file__)), "opendek", "scripts", "deploy_esp"))
    import deploy_esp # pylint: disable=import-error
    deploy_esp.run_main("./saseek_config.yml", "Secure Access Service Edge Experience Kits")
