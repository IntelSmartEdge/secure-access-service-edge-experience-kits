/**
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
**/
package cmd

import (
	"os"
	"sasectl/utils"

	"github.com/spf13/cobra"
)

var (
	sasectlConf *utils.SaseCtlConf
)

const (
	configFP = "/etc/sasectl.conf"
)

var rootCmd = &cobra.Command{
	Use:   "sasectl",
	Short: "Command line tools for Smart-Edge Open SASE EK",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		sasectlConf, _ = utils.LoadSasectlConfig(configFP)
	},
	// Run: func(cmd *cobra.Command, args []string) {
	// },
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
}
