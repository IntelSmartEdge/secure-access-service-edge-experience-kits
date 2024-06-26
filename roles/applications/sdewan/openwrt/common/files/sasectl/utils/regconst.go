/**
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Intel Corporation
**/
package utils

const (
	NameSpaceName               = "sdewan-system"
	RootIssuerName              = "sdewan-controller"
	RootCAIssuerName            = "sdewan-controller-ca"
	RootCertName                = "sdewan-controller"
	SCCCertName                 = "sdewan-controller-base"
	StoreName                   = "centralcontroller"
	OverlayCollection           = "overlays"
	OverlayResource             = "overlay-name"
	ProposalCollection          = "proposals"
	ProposalResource            = "proposal-name"
	HubCollection               = "hubs"
	HubResource                 = "hub-name"
	ConnectionCollection        = "connections"
	ConnectionResource          = "connection-name"
	CNFCollection               = "cnfs"
	CNFResource                 = "cnf-name"
	DeviceCollection            = "devices"
	DeviceResource              = "device-name"
	IPRangeCollection           = "ipranges"
	IPRangeResource             = "iprange-name"
	CertCollection              = "certificates"
	CertResource                = "certificate-name"
	ClusterSyncCollection       = "cluster-sync-objects"
	ClusterSyncResource         = "cluster-sync-object-name"
	SiteCollection              = "sites"
	SiteResource                = "site-name"
	Resource                    = "resource"
	Resource_Status_NotDeployed = "NotDeployed"
	Resource_Status_Deployed    = "Deployed"
)

const CNFValueCopyright = `#/* Copyright (c) 2021 Intel Corporation, Inc
# *
# * Licensed under the Apache License, Version 2.0 (the "License");
# * you may not use this file except in compliance with the License.
# * You may obtain a copy of the License at
# *
# *     http://www.apache.org/licenses/LICENSE-2.0
# *
# * Unless required by applicable law or agreed to in writing, software
# * distributed under the License is distributed on an "AS IS" BASIS,
# * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# * See the License for the specific language governing permissions and
# * limitations under the License.
# */
#
# Default values for cnf.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.`

const CNFCMCopyright = `
#/* Copyright (c) 2021 Intel Corporation, Inc
# *
# * Licensed under the Apache License, Version 2.0 (the "License");
# * you may not use this file except in compliance with the License.
# * You may obtain a copy of the License at
# *
# *     http://www.apache.org/licenses/LICENSE-2.0
# *
# * Unless required by applicable law or agreed to in writing, software
# * distributed under the License is distributed on an "AS IS" BASIS,
# * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# * See the License for the specific language governing permissions and
# * limitations under the License.
# */`

const CNFBaseShell = ` #!/bin/bash
# Always exit on errors.
set -ex
sysctl -w net.ipv4.ip_forward=1
echo "" > /etc/config/network
cat > /etc/config/mwan3 <<EOF
config globals 'globals'
	option mmx_mask '0x3F00'
	option local_source 'lan'
EOF

providerip=$(echo {{ .Values.providerCIDR }} | cut -d/ -f1)
sep="."
suf="0"

eval "networks=$(grep nfn-network /tmp/podinfo/annotations | awk  -F '=' '{print $2}')"
for net in $(echo -e $networks | jq -c ".interface[]")
do
  interface=$(echo $net | jq -r .interface)
  ipaddr=$(ifconfig $interface | awk '/inet/{print $2}' | cut -f2 -d ":" | awk 'NR==1 {print $1}')
  vif="$interface"
  netmask=$(ifconfig $interface | awk '/inet/{print $4}'| cut -f2 -d ":" | head -1)
  cat >> /etc/config/network <<EOF
config interface '$vif'
	option ifname '$interface'
	option proto 'static'
	option ipaddr '$ipaddr'
	option netmask '$netmask'
EOF
done

if [ -f "/tmp/sdewan/account/password" ]; then
	echo "Changing password ..."
	pass=$(cat /tmp/sdewan/account/password)
	echo root:$pass | chpasswd -m
fi

if [ -d "/tmp/sdewan/serving-certs/" ]; then
	echo "Configuration certificates ..."
	cp /tmp/sdewan/serving-certs/tls.crt /etc/uhttpd.crt
	cp /tmp/sdewan/serving-certs/tls.key /etc/uhttpd.key
fi

/sbin/procd &
/sbin/ubusd &
iptables -t nat -L
sleep 1
/etc/init.d/rpcd start
/etc/init.d/dnsmasq start
/etc/init.d/network start
/etc/init.d/odhcpd start
/etc/init.d/uhttpd start
/etc/init.d/log start
/etc/init.d/dropbear start
/etc/init.d/mwan3 restart
/etc/init.d/firewall restart
defaultip=$(grep "\podIP\b" /tmp/podinfo/annotations | cut -d/ -f2 | cut -d'"' -f2)`

const CNFTemplateShell = `{{- if .Values.publicIpAddress }}
    iptables -t nat -I PREROUTING 1 -m tcp -p tcp -d {{ .Values.publicIpAddress }} --dport 6443 -j DNAT --to-dest 10.96.0.1:443
{{- end }}
{{- if .Values.defaultCIDR }}
    ip rule add from {{ .Values.defaultCIDR }} lookup 40
    ip rule add from $defaultip lookup main
{{- end }}
    echo "Entering sleep... (success)"
    # Sleep forever.
    while true; do sleep 100; done`

const CNFRouterShell = `for net in $(echo -e $networks | jq -c ".interface[]")
do
	interface=$(echo $net | jq -r .interface)
	ipaddr=$(ifconfig $interface | awk '/inet/{print $2}' | cut -f2 -d ":" | awk 'NR==1 {print $1}')
	echo $ipaddr | ( IFS="." read -r var1 var2 var3 var4; CIDR="$var1$sep$var2$sep$var3$sep$suf"; \
		if [ "${CIDR}" = "${providerip}" ] ; then iptables -t nat -A POSTROUTING -o $interface -d {{ .Values.providerCIDR }} -j SNAT --to-source $ipaddr; fi)
done`
